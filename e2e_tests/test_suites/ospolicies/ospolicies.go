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
	osconfig "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/cloud.google.com/go/osconfig/apiv1"
	testconfig "github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/kylelemons/godebug/pretty"
	computeApi "google.golang.org/api/compute/v1"
	"google.golang.org/protobuf/testing/protocmp"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/google.golang.org/genproto/googleapis/cloud/osconfig/v1"
)

var (
	testSuiteName = "OSPolicies"
	dump          = &pretty.Config{IncludeUnexported: true}
)

const (
	packageResourceApt       = "packageresourceapt"
	packageResourceDeb       = "packageresourcedeb"
	packageResourceYum       = "packageresourceyum"
	packageResourceZypper    = "packageresourcezypper"
	packageResourceRpm       = "packageresourcerpm"
	packageResourceGoo       = "packageresourcegoo"
	packageResourceMsi       = "packageresourcemsi"
	repositoryResourceApt    = "repositoryresourceapt"
	repositoryResourceYum    = "repositoryresourceyum"
	repositoryResourceZypper = "repositoryresourcezypper"
	repositoryResourceGoo    = "repositoryresourcegoo"
	fileResource             = "fileresource"
	linuxExecResource        = "linuxexecresource"
	windowsExecResource      = "windowsexecresource"
	validationMode           = "validationmode"
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
	queryPaths           []string
	assertTimeout        time.Duration
	wantCompliances      []*osconfigpb.OSPolicyAssignmentReport_OSPolicyCompliance
}

func newOsPolicyTestSetup(image, imageName, instanceName, testName string, queryPaths []string, machineType string, ospa *osconfigpb.OSPolicyAssignment, startup *computeApi.MetadataItems, assertTimeout time.Duration, wantCompliances []*osconfigpb.OSPolicyAssignmentReport_OSPolicyCompliance) *osPolicyTestSetup {
	return &osPolicyTestSetup{
		image:                image,
		imageName:            imageName,
		osPolicyAssignmentID: instanceName,
		instanceName:         instanceName,
		osPolicyAssignment:   ospa,
		testName:             testName,
		machineType:          machineType,
		queryPaths:           queryPaths,
		assertTimeout:        assertTimeout,
		startup:              startup,
		wantCompliances:      wantCompliances,
	}
}

// TestSuite is a OSPackage test suite.
func TestSuite(ctx context.Context, tswg *sync.WaitGroup, testSuites chan *junitxml.TestSuite, logger *log.Logger, testSuiteRegex, testCaseRegex *regexp.Regexp) {
	defer tswg.Done()

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

// Use this lock to limit write QPS.
var gpMx sync.Mutex

func createOSPolicyAssignment(ctx context.Context, client *osconfig.OsConfigZonalClient, req *osconfigpb.CreateOSPolicyAssignmentRequest, testCase *junitxml.TestCase) (*osconfigpb.OSPolicyAssignment, error) {
	// Use the lock to slow down write QPS just a bit.
	gpMx.Lock()
	op, err := client.CreateOSPolicyAssignment(ctx, req)
	gpMx.Unlock()
	if err != nil {
		return nil, fmt.Errorf("error running CreateOSPolicyAssignment: %s", utils.GetStatusFromError(err))
	}
	testCase.Logf("OSPolicyAssignment created, waiting for operation %q", op.Name())
	// Wait up to 5 min for this to complete.
	ctx, cncl := context.WithTimeout(ctx, 5*time.Minute)
	defer cncl()
	ospa, err := op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("error waiting for create operation %q, to complete: %s", op.Name(), utils.GetStatusFromError(err))
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
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("osconfig-disabled-features", "guestpolicies,osinventory"))
	testProjectConfig := testconfig.GetProject()
	zone := testProjectConfig.AcquireZone()
	defer testProjectConfig.ReleaseZone(zone)
	// No test should take longer than 20 min, start the timer
	// after AcquireZone as that can take some time.
	ctx, cncl := context.WithTimeout(ctx, 20*time.Minute)
	defer cncl()
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

	client, err := gcpclients.GetOsConfigClientV1()
	if err != nil {
		testCase.WriteFailure("Error getting osconfig client: %v", err)
		return
	}

	req := &osconfigpb.CreateOSPolicyAssignmentRequest{
		Parent:               fmt.Sprintf("projects/%s/locations/%s", testProjectConfig.TestProjectID, zone),
		OsPolicyAssignmentId: testSetup.osPolicyAssignmentID,
		OsPolicyAssignment:   testSetup.osPolicyAssignment,
	}

	testCase.Logf("Creating OSPolicyAssignment: %q", fmt.Sprintf("%s/%s", req.GetParent(), req.GetOsPolicyAssignmentId()))
	ospa, err := createOSPolicyAssignment(ctx, client, req, testCase)
	if err != nil {
		testCase.WriteFailure("Error running createOSPolicyAssignment: %s", err)
		return
	}
	defer cleanupOSPolicyAssignment(ctx, client, testCase, ospa.GetName())

	// Check that the compliance output meets expectations.
	repReq := &osconfigpb.GetOSPolicyAssignmentReportRequest{Name: fmt.Sprintf("projects/%s/locations/%s/instances/%d/osPolicyAssignments/%s/report", testProjectConfig.TestProjectID, zone, inst.Id, testSetup.osPolicyAssignmentID)}
	compliance, err := client.GetOSPolicyAssignmentReport(ctx, repReq)
	if err != nil {
		testCase.WriteFailure("Error running GetOSPolicyAssignmentReport: %s", err)
		return
	}
	if diff := cmp.Diff(testSetup.wantCompliances, compliance.OsPolicyCompliances, protocmp.Transform(), cmp.FilterPath(func(p cmp.Path) bool {
		if p.Last().String() == `["os_policy_assignment"]` {
			return true
		}
		return false
	}, cmp.Ignore())); diff != "" {
		testCase.WriteFailure("Did not get expected OsPolicyCompliances (-want +got):\n%s", diff)
		return
	}

	for _, p := range testSetup.queryPaths {
		if _, err := inst.WaitForGuestAttributes(p, 10*time.Second, testSetup.assertTimeout); err != nil {
			testCase.WriteFailure("Error while asserting: %v", err)
			return
		}
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
		if tc.Failure != nil {
			rerunTC := junitxml.NewTestCase(testSuiteName, strings.TrimPrefix(tc.Name, fmt.Sprintf("[%s] ", testSuiteName)))
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Printf("Rerunning TestCase %q", rerunTC.Name)
				runTest(ctx, rerunTC, testSetup, logger)
				rerunTC.Finish(tests)
				logger.Printf("TestCase %q finished in %fs", rerunTC.Name, rerunTC.Time)
			}()
		}
		tc.Finish(tests)
		logger.Printf("TestCase %q finished in %fs", tc.Name, tc.Time)
	}
}

// factory method to get testcase from the testsetup
func getTestCaseFromTestSetUp(testSetup *osPolicyTestSetup) (*junitxml.TestCase, error) {
	var tc *junitxml.TestCase

	switch testSetup.testName {
	case packageResourceApt:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource Apt] [%s]", testSetup.imageName))
	case packageResourceDeb:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource Deb] [%s]", testSetup.imageName))
	case packageResourceYum:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource Yum] [%s]", testSetup.imageName))
	case packageResourceZypper:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource Zypper] [%s]", testSetup.imageName))
	case packageResourceRpm:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource Rpm] [%s]", testSetup.imageName))
	case packageResourceGoo:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource GooGet] [%s]", testSetup.imageName))
	case packageResourceMsi:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[PackageResource Msi] [%s]", testSetup.imageName))

	case repositoryResourceApt:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[RepositoryResource Apt] [%s]", testSetup.imageName))
	case repositoryResourceYum:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[RepositoryResource Yum] [%s]", testSetup.imageName))
	case repositoryResourceZypper:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[RepositoryResource Zypper] [%s]", testSetup.imageName))
	case repositoryResourceGoo:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[RepositoryResource GooGet] [%s]", testSetup.imageName))

	case fileResource:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[FileResource] [%s]", testSetup.imageName))

	case linuxExecResource:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Linux ExecResource] [%s]", testSetup.imageName))
	case windowsExecResource:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Windows ExecResource] [%s]", testSetup.imageName))

	case validationMode:
		tc = junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[ValidationMode] [%s]", testSetup.imageName))
	default:
		return nil, fmt.Errorf("unknown test function name: %s", testSetup.testName)
	}

	return tc, nil
}

func cleanupOSPolicyAssignment(ctx context.Context, client *osconfig.OsConfigZonalClient, testCase *junitxml.TestCase, name string) {
	// Use the lock to slow down write QPS just a bit.
	gpMx.Lock()
	op, err := client.DeleteOSPolicyAssignment(ctx, &osconfigpb.DeleteOSPolicyAssignmentRequest{Name: name})
	gpMx.Unlock()
	if err != nil {
		testCase.WriteFailure(fmt.Sprintf("Error calling DeleteOSPolicyAssignment: %s", utils.GetStatusFromError(err)))
		return
	}
	ctx, cncl := context.WithTimeout(ctx, 5*time.Minute)
	defer cncl()
	op.Wait(ctx)
}
