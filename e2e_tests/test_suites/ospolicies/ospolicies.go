//  Copyright 2021 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

// Package ospolicies are OSPolicy osconfig agent tests.
package ospolicies

import (
	"context"
	"fmt"
	"log"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/compute-image-tools/go/e2e_test_utils/junitxml"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	gcpclients "github.com/GoogleCloudPlatform/osconfig/e2e_tests/gcp_clients"
	osconfigZonalV1alpha "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/cloud.google.com/go/osconfig/apiv1alpha"
	testconfig "github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	"github.com/kylelemons/godebug/pretty"
	computeApi "google.golang.org/api/compute/v1"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha"
)

var (
	testSuiteName = "OSPolicies"
	dump          = &pretty.Config{IncludeUnexported: true}
)

const (
	packageInstallFunction            = "pkginstall"
	packageRemovalFunction            = "pkgremoval"
	packageInstallFromNewRepoFunction = "pkgfromnewrepo"
	recipeInstallFunction             = "recipeinstall"
)

type osPolicyTestSetup struct {
	image                string
	imageName            string
	osPolicyAssignmentID string
	instanceName         string
	testName             string
	osPolicyAssignment   *osconfigpb.OSPolicyAssignment
	startup              *computeApi.MetadataItems
	machineType          string
	queryPath            string
	assertTimeout        time.Duration
}

func newOsPolicyTestSetup(image, imageName, instanceName, testName, queryPath, machineType string, ospa *osconfigpb.OSPolicyAssignment, startup *computeApi.MetadataItems, assertTimeout time.Duration) *osPolicyTestSetup {
	return &osPolicyTestSetup{
		image:                image,
		imageName:            imageName,
		osPolicyAssignmentID: instanceName,
		instanceName:         instanceName,
		osPolicyAssignment:   ospa,
		testName:             testName,
		machineType:          machineType,
		queryPath:            queryPath,
		assertTimeout:        assertTimeout,
		startup:              startup,
	}
}

// TestSuite is a OSPackage test suite.
func TestSuite(ctx context.Context, tswg *sync.WaitGroup, testSuites chan *junitxml.TestSuite, logger *log.Logger, testSuiteRegex, testCaseRegex *regexp.Regexp) {
	defer tswg.Done()

	if !strings.Contains(config.SvcEndpoint(), "staging") {
		return
	}
	if testSuiteRegex != nil && !testSuiteRegex.MatchString(testSuiteName) {
		return
	}

	testSuite := junitxml.NewTestSuite(testSuiteName)
	defer testSuite.Finish(testSuites)

	logger.Printf("Running TestSuite %q", testSuite.Name)
	testSetup := generateAllTestSetup()
	var wg sync.WaitGroup
	tests := make(chan *junitxml.TestCase)
	for _, setup := range testSetup {
		wg.Add(1)
		go testCase(ctx, setup, tests, &wg, logger, testCaseRegex)
	}

	go func() {
		wg.Wait()
		close(tests)
	}()

	for ret := range tests {
		testSuite.TestCase = append(testSuite.TestCase, ret)
	}

	logger.Printf("Finished TestSuite %q", testSuite.Name)
}

// We only want to create one GuestPolicy at a time to limit QPS.
var gpMx sync.Mutex

func createOSPolicyAssignment(ctx context.Context, client *osconfigZonalV1alpha.OsConfigZonalClient, req *osconfigpb.CreateOSPolicyAssignmentRequest) (*osconfigpb.OSPolicyAssignment, error) {
	gpMx.Lock()
	defer gpMx.Unlock()
	op, err := client.CreateOSPolicyAssignment(ctx, req)
	if err != nil {
		return nil, err
	}
	//return op.Wait(ctx)
	_ = op

	getReq := &osconfigpb.GetOSPolicyAssignmentRequest{Name: fmt.Sprintf("%s/osPolicyAssignments/%s", req.GetParent(), req.GetOsPolicyAssignmentId())}
	ospa, err := client.GetOSPolicyAssignment(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return ospa, nil
}

func runTest(ctx context.Context, testCase *junitxml.TestCase, testSetup *osPolicyTestSetup, logger *log.Logger) {
	computeClient, err := gcpclients.GetComputeClient()
	if err != nil {
		testCase.WriteFailure("Error getting compute client: %v", err)
		return
	}

	var metadataItems []*computeApi.MetadataItems
	metadataItems = append(metadataItems, testSetup.startup)
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("enable-osconfig", "true"))
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("osconfig-disabled-features", "guestpolicies"))
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("osconfig-enabled-prerelease-features", "ospolicies"))
	testProjectConfig := testconfig.GetProject()
	zone := testProjectConfig.AcquireZone()
	defer testProjectConfig.ReleaseZone(zone)
	testCase.Logf("Creating instance %q with image %q", testSetup.instanceName, testSetup.image)
	inst, err := utils.CreateComputeInstance(metadataItems, computeClient, testSetup.machineType, testSetup.image, testSetup.instanceName, testProjectConfig.TestProjectID, zone, testProjectConfig.ServiceAccountEmail, testProjectConfig.ServiceAccountScopes)
	if err != nil {
		testCase.WriteFailure("Error creating instance: %s", utils.GetStatusFromError(err))
		return
	}
	defer inst.Cleanup()
	defer inst.RecordSerialOutput(ctx, path.Join(*config.OutDir, testSuiteName), 1)

	testCase.Logf("Waiting for agent install to complete")
	if _, err := inst.WaitForGuestAttributes("osconfig_tests/install_done", 5*time.Second, 10*time.Minute); err != nil {
		testCase.WriteFailure("Error waiting for osconfig agent install: %v", err)
		return
	}

	client, err := gcpclients.GetOsConfigClientV1Alpha()
	if err != nil {
		testCase.WriteFailure("Error getting osconfig client: %v", err)
		return
	}

	req := &osconfigpb.CreateOSPolicyAssignmentRequest{
		Parent:               fmt.Sprintf("projects/%s/locations/%s", testProjectConfig.TestProjectID, zone),
		OsPolicyAssignmentId: testSetup.osPolicyAssignmentID,
		OsPolicyAssignment:   testSetup.osPolicyAssignment,
	}

	testCase.Logf("Creating OSPolicyAssignment")
	ospa, err := createOSPolicyAssignment(ctx, client, req)
	if err != nil {
		testCase.WriteFailure("Error running CreateOSPolicyAssignment: %s", utils.GetStatusFromError(err))
		return
	}
	defer cleanupOSPolicyAssignment(ctx, testCase, ospa.GetName())

	testCase.Logf("Waiting for signal from GuestAttributes")
	if _, err := inst.WaitForGuestAttributes(testSetup.queryPath, 10*time.Second, testSetup.assertTimeout); err != nil {
		testCase.WriteFailure("error while asserting: %v", err)
		return
	}
}

func testCase(ctx context.Context, testSetup *osPolicyTestSetup, tests chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp) {
	defer wg.Done()

	tc, err := getTestCaseFromTestSetUp(testSetup)
	if err != nil {
		logger.Fatalf("invalid testcase: %+v", err)
		return
	}
	if tc.FilterTestCase(regex) {
		tc.Finish(tests)
	} else {
		logger.Printf("Running TestCase %q", tc.Name)
		runTest(ctx, tc, testSetup, logger)
		tc.Finish(tests)
		logger.Printf("TestCase %q finished in %fs", tc.Name, tc.Time)
	}
}

// factory method to get testcase from the testsetup
func getTestCaseFromTestSetUp(testSetup *osPolicyTestSetup) (*junitxml.TestCase, error) {
	var tc *junitxml.TestCase

	switch testSetup.testName {
	case packageInstallFunction:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Package installation] [%s]", testSetup.imageName))
	case packageRemovalFunction:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Package removal] [%s]", testSetup.imageName))
	case packageInstallFromNewRepoFunction:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Add a new package from new repository] [%s]", testSetup.imageName))
	default:
		return nil, fmt.Errorf("unknown test function name: %s", testSetup.testName)
	}

	return tc, nil
}

func cleanupOSPolicyAssignment(ctx context.Context, testCase *junitxml.TestCase, name string) {
	client, err := gcpclients.GetOsConfigClientV1Alpha()
	if err != nil {
		testCase.WriteFailure(fmt.Sprintf("Error while deleting guest policy: %s", utils.GetStatusFromError(err)))
	}

	op, err := client.DeleteOSPolicyAssignment(ctx, &osconfigpb.DeleteOSPolicyAssignmentRequest{Name: name})
	if err != nil {
		testCase.WriteFailure(fmt.Sprintf("Error calling DeleteOSPolicyAssignment: %s", utils.GetStatusFromError(err)))
	}

	// op.Wait(ctx)
	_ = op
	return
}
