//  Copyright 2020 Google Inc. All Rights Reserved.
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

package inventoryreporting

import (
	"context"
	"fmt"
	"log"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/compute-image-tools/go/e2e_test_utils/junitxml"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	gcpclients "github.com/GoogleCloudPlatform/osconfig/e2e_tests/gcp_clients"
	testconfig "github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	api "google.golang.org/api/compute/v1"
)

const (
	testSuiteName = "OSInventoryReporting"
)

// TestSuite is an OSInventoryReporting test suite.
func TestSuite(ctx context.Context, tswg *sync.WaitGroup, testSuites chan *junitxml.TestSuite, logger *log.Logger, testSuiteRegex, testCaseRegex *regexp.Regexp) {
	defer tswg.Done()

	if testSuiteRegex != nil && !testSuiteRegex.MatchString(testSuiteName) {
		return
	}

	testSuite := junitxml.NewTestSuite(testSuiteName)
	defer testSuite.Finish(testSuites)

	logger.Printf("Running TestSuite %q", testSuite.Name)

	var wg sync.WaitGroup
	tests := make(chan *junitxml.TestCase)
	for _, setup := range headImageTestSetup() {
		wg.Add(1)
		go inventoryReportingTestCase(ctx, setup, tests, &wg, logger, testCaseRegex)
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

func runInventoryReportingTest(ctx context.Context, testSetup *inventoryTestSetup, testCase *junitxml.TestCase, logwg *sync.WaitGroup) {
	testCase.Logf("Creating compute client")

	computeClient, err := gcpclients.GetComputeClient()
	if err != nil {
		testCase.WriteFailure("Error getting compute client: %v", err)
		return
	}

	testSetup.hostname = fmt.Sprintf("inventoryreporting-test-%s-%s", path.Base(testSetup.testName), utils.RandString(5))

	var metadataItems []*api.MetadataItems
	metadataItems = append(metadataItems, testSetup.startup)
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("enable-os-inventory", "true"))
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("osconfig-disabled-features", "tasks,guestpolicies"))

	testProjectConfig := testconfig.GetProject()
	zone := testProjectConfig.AcquireZone()
	defer testProjectConfig.ReleaseZone(zone)

	testCase.Logf("Creating instance %q with image %q", testSetup.hostname, testSetup.image)
	inst, err := utils.CreateComputeInstance(metadataItems, computeClient, testSetup.machineType, testSetup.image, testSetup.hostname, testProjectConfig.TestProjectID, zone, testProjectConfig.ServiceAccountEmail, testProjectConfig.ServiceAccountScopes)
	if err != nil {
		testCase.WriteFailure("Error creating instance: %v", err)
		return
	}
	defer inst.Cleanup()
	defer inst.RecordSerialOutput(ctx, path.Join(*config.OutDir, testSuiteName), 1)

	testCase.Logf("Waiting for agent install to complete")
	if _, err := inst.WaitForGuestAttributes("osconfig_tests/install_done", 5*time.Second, 10*time.Minute); err != nil {
		testCase.WriteFailure("Error waiting for osconfig agent install: %v", err)
		return
	}

	// Build regexes for verification.
	positivePatterns := []string{
		fmt.Sprintf(`.*Calling ReportInventory with request containing hostname %s, short name %s, [1-9]+[0-9]* installed packages, [0-9]+ available packages`, testSetup.hostname, testSetup.shortName),
		`.*"report_full_inventory".*true.*`,
		`.*"report_full_inventory".*false.*`,
		`.*Finished task "Report OSInventory".*`,
	}
	positiveRegexes, err := compileRegex(positivePatterns)
	if err != nil {
		testCase.WriteFailure("Error compiling ReportInventory RPC payload regex: %v", err)
		return
	}

	negativePatterns := []string{
		".*Error reporting inventory.*",
	}
	negativeRegexes, err := compileRegex(negativePatterns)
	if err != nil {
		testCase.WriteFailure("Error compiling ReportInventory negative regex: %v", err)
		return
	}

	// Verify a successful ReportingInventory call.
	if err := inst.WaitForSerialOutput(positiveRegexes, negativeRegexes, 1, 10*time.Second, testSetup.timeout); err != nil {
		testCase.WriteFailure("Error verifying a successful ReportInventory call: %v", err)
		return
	}
}

func inventoryReportingTestCase(ctx context.Context, testSetup *inventoryTestSetup, tc chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp) {
	defer wg.Done()

	var logwg sync.WaitGroup
	inventoryTest := junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Report inventory] [%s]", testSetup.testName))

	if inventoryTest.FilterTestCase(regex) {
		inventoryTest.Finish(tc)
		return
	}

	logger.Printf("Running TestCase %q", inventoryTest.Name)
	runInventoryReportingTest(ctx, testSetup, inventoryTest, &logwg)
	inventoryTest.Finish(tc)
	logger.Printf("TestCase %q finished", inventoryTest.Name)

	logwg.Wait()
}

func compileRegex(patterns []string) ([]*regexp.Regexp, error) {
	var regexes []*regexp.Regexp
	for _, pattern := range patterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		regexes = append(regexes, regex)
	}
	return regexes, nil
}
