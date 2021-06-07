//  Copyright 2018 Google Inc. All Rights Reserved.
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

package inventory

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	testconfig "github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	apiBeta "google.golang.org/api/compute/v0.beta"
	api "google.golang.org/api/compute/v1"
)

const (
	testSuiteName = "OSInventory"
)

// TestSuite is a OSInventory test suite.
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
		go inventoryTestCase(ctx, setup, tests, &wg, logger, testCaseRegex)
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

func runGatherInventoryTest(ctx context.Context, testSetup *inventoryTestSetup, testCase *junitxml.TestCase, logwg *sync.WaitGroup) ([]*apiBeta.GuestAttributesEntry, bool) {
	testCase.Logf("Creating compute client")

	computeClient, err := gcpclients.GetComputeClient()
	if err != nil {
		testCase.WriteFailure("Error getting compute client: %v", err)
		return nil, false
	}

	testSetup.hostname = fmt.Sprintf("inventory-test-%s-%s", path.Base(testSetup.testName), utils.RandString(5))

	var metadataItems []*api.MetadataItems
	metadataItems = append(metadataItems, testSetup.startup)
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("enable-osconfig", "true"))
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("osconfig-disabled-features", "tasks,guestpolicies"))
	metadataItems = append(metadataItems, compute.BuildInstanceMetadataItem("osconfig-poll-interval", "1"))

	testProjectConfig := testconfig.GetProject()
	zone := testProjectConfig.AcquireZone()
	defer testProjectConfig.ReleaseZone(zone)

	testCase.Logf("Creating instance %q with image %q", testSetup.hostname, testSetup.image)
	inst, err := utils.CreateComputeInstance(metadataItems, computeClient, testSetup.machineType, testSetup.image, testSetup.hostname, testProjectConfig.TestProjectID, zone, testProjectConfig.ServiceAccountEmail, testProjectConfig.ServiceAccountScopes)
	if err != nil {
		testCase.WriteFailure("Error creating instance: %v", err)
		return nil, false
	}
	defer inst.Cleanup()
	defer inst.RecordSerialOutput(ctx, path.Join(*config.OutDir, testSuiteName), 1)

	testCase.Logf("Waiting for agent install to complete")
	if _, err := inst.WaitForGuestAttributes("osconfig_tests/install_done", 5*time.Second, 10*time.Minute); err != nil {
		testCase.WriteFailure("Error waiting for osconfig agent install: %v", err)
		return nil, false
	}

	return gatherInventory(testCase, testSetup, inst)
}

func gatherInventory(testCase *junitxml.TestCase, testSetup *inventoryTestSetup, inst *compute.Instance) ([]*apiBeta.GuestAttributesEntry, bool) {
	testCase.Logf("Checking inventory data")
	// LastUpdated is the last entry written by the agent, so wait on that.
	_, err := inst.WaitForGuestAttributes("guestInventory/LastUpdated", 10*time.Second, testSetup.timeout)
	if err != nil {
		testCase.WriteFailure("Error waiting for guest attributes: %v", err)
		return nil, false
	}

	ga, err := inst.GetGuestAttributes("guestInventory/")
	if err != nil {
		testCase.WriteFailure("Error getting guest attributes: %v", err)
		return nil, false
	}
	return ga, true
}

func runHostnameTest(ga []*apiBeta.GuestAttributesEntry, testSetup *inventoryTestSetup, testCase *junitxml.TestCase) {
	var hostname string
	for _, item := range ga {
		if item.Key == "Hostname" {
			hostname = item.Value
			break
		}
	}

	if hostname == "" {
		s, _ := json.MarshalIndent(ga, "", "  ")
		testCase.WriteFailure("Hostname not found in guestInventory: %s", s)
		return
	}

	if hostname != testSetup.hostname {
		testCase.WriteFailure("Hostname does not match expectation: got: %q, want: %q", hostname, testSetup.hostname)
	}
}

func runShortNameTest(ga []*apiBeta.GuestAttributesEntry, testSetup *inventoryTestSetup, testCase *junitxml.TestCase) {
	var shortName string
	for _, item := range ga {
		if item.Key == "ShortName" {
			shortName = item.Value
			break
		}
	}

	if shortName == "" {
		s, _ := json.MarshalIndent(ga, "", "  ")
		testCase.WriteFailure("ShortName not found in guestInventory: %s", s)
		return
	}

	if shortName != testSetup.shortName {
		testCase.WriteFailure("ShortName does not match expectation: got: %q, want: %q", shortName, testSetup.shortName)
	}
}

func runPackagesTest(ga []*apiBeta.GuestAttributesEntry, testSetup *inventoryTestSetup, testCase *junitxml.TestCase) {
	var packagesEncoded string
	for _, item := range ga {
		if item.Key == "InstalledPackages" {
			packagesEncoded = item.Value
			break
		}
	}

	if packagesEncoded == "" {
		s, _ := json.MarshalIndent(ga, "", "  ")
		testCase.WriteFailure("InstalledPackages not found in guestInventory: %s", s)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(packagesEncoded)
	if err != nil {
		testCase.WriteFailure(err.Error())
		return
	}

	zr, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		testCase.WriteFailure(err.Error())
		return
	}
	defer zr.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zr); err != nil {
		testCase.WriteFailure(err.Error())
		return
	}

	var pkgs packages.Packages
	if err := json.Unmarshal(buf.Bytes(), &pkgs); err != nil {
		testCase.WriteFailure(err.Error())
		return
	}

	for _, pt := range testSetup.packageType {
		switch pt {
		case "googet":
			if len(pkgs.GooGet) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		case "deb":
			if len(pkgs.Deb) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		case "rpm":
			if len(pkgs.Rpm) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		case "pip":
			if len(pkgs.Pip) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		case "gem":
			if len(pkgs.Gem) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		case "wua":
			if len(pkgs.WUA) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		case "qfe":
			if len(pkgs.QFE) < 1 {
				testCase.WriteFailure("No packages exported in InstalledPackages for %q", pt)
				return
			}
		}
	}
}

func inventoryTestCase(ctx context.Context, testSetup *inventoryTestSetup, tests chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp) {
	defer wg.Done()

	var logwg sync.WaitGroup
	gatherInventoryTest := junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Gather inventory] [%s]", testSetup.testName))
	hostnameTest := junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Check Hostname] [%s]", testSetup.testName))
	shortNameTest := junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Check ShortName] [%s]", testSetup.testName))
	packageTest := junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Check InstalledPackages] [%s]", testSetup.testName))

	if gatherInventoryTest.FilterTestCase(regex) {
		gatherInventoryTest.Finish(tests)

		hostnameTest.WriteSkipped("Setup skipped")
		hostnameTest.Finish(tests)
		shortNameTest.WriteSkipped("Setup skipped")
		hostnameTest.Finish(tests)
		packageTest.WriteSkipped("Setup skipped")
		packageTest.Finish(tests)
		return
	}

	logger.Printf("Running TestCase %q", gatherInventoryTest.Name)
	ga, ok := runGatherInventoryTest(ctx, testSetup, gatherInventoryTest, &logwg)
	gatherInventoryTest.Finish(tests)
	logger.Printf("TestCase %q finished", gatherInventoryTest.Name)
	if !ok {
		rerunTC := junitxml.NewTestCase(testSuiteName, strings.TrimPrefix(gatherInventoryTest.Name, fmt.Sprintf("[%s] ", testSuiteName)))
		logger.Printf("Rerunning TestCase %q", rerunTC.Name)
		ga, ok = runGatherInventoryTest(ctx, testSetup, rerunTC, &logwg)
		rerunTC.Finish(tests)
		logger.Printf("TestCase %q finished in %fs", rerunTC.Name, rerunTC.Time)
	}
	if !ok {
		hostnameTest.WriteFailure("Setup Failure")
		hostnameTest.Finish(tests)
		shortNameTest.WriteFailure("Setup Failure")
		shortNameTest.Finish(tests)
		packageTest.WriteFailure("Setup Failure")
		packageTest.Finish(tests)
		return
	}

	for tc, f := range map[*junitxml.TestCase]func([]*apiBeta.GuestAttributesEntry, *inventoryTestSetup, *junitxml.TestCase){
		hostnameTest:  runHostnameTest,
		shortNameTest: runShortNameTest,
		packageTest:   runPackagesTest,
	} {
		// Skip packages test for cos as it is not currently supported.
		if strings.Contains(tc.Name, "cos") && strings.Contains(tc.Name, "Packages") {
			tc.WriteSkipped("Inventory Packages not currently supported on COS")
			tc.Finish(tests)
		} else if tc.FilterTestCase(regex) {
			tc.Finish(tests)
		} else {
			logger.Printf("Running TestCase %q", tc.Name)
			f(ga, testSetup, tc)
			tc.Finish(tests)
			logger.Printf("TestCase %q finished in %fs", tc.Name, tc.Time)
		}
	}
	logwg.Wait()

}
