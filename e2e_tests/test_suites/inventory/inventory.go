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

func runGatherInventoryTest(ctx context.Context, testSetup *inventoryTestSetup, testCase *junitxml.TestCase, logwg *sync.WaitGroup) []*apiBeta.GuestAttributesEntry {
	testCase.Logf("Creating compute client")

	computeClient, err := gcpclients.GetComputeClient()
	if err != nil {
		testCase.WriteFailure("Error getting compute client: %v", err)
		return nil
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
		return nil
	}
	defer inst.Cleanup()
	defer inst.RecordSerialOutput(ctx, path.Join(*config.OutDir, testSuiteName), 1)

	testCase.Logf("Waiting for agent install to complete")
	if _, err := inst.WaitForGuestAttributes("osconfig_tests/install_done", 5*time.Second, 10*time.Minute); err != nil {
		testCase.WriteFailure("Error waiting for osconfig agent install: %v", err)
		return nil
	}

	return gatherInventory(testCase, testSetup, inst)
}

func gatherInventory(testCase *junitxml.TestCase, testSetup *inventoryTestSetup, inst *compute.Instance) []*apiBeta.GuestAttributesEntry {
	testCase.Logf("Checking inventory data")
	// LastUpdated is the last entry written by the agent, so wait on that.
	_, err := inst.WaitForGuestAttributes("guestInventory/LastUpdated", 10*time.Second, testSetup.timeout)
	if err != nil {
		testCase.WriteFailure("Error waiting for guest attributes: %v", err)
		return nil
	}

	ga, err := inst.GetGuestAttributes("guestInventory/")
	if err != nil {
		testCase.WriteFailure("Error getting guest attributes: %v", err)
		return nil
	}
	return ga
}

func runHostnameTest(ga []*apiBeta.GuestAttributesEntry, testSetup *inventoryTestSetup) error {
	var hostname string
	for _, item := range ga {
		if item.Key == "Hostname" {
			hostname = item.Value
			break
		}
	}

	if hostname == "" {
		s, _ := json.MarshalIndent(ga, "", "  ")
		return fmt.Errorf("Hostname not found in guestInventory: %s", s)
	}

	if hostname != testSetup.hostname {
		return fmt.Errorf("Hostname does not match expectation: got: %q, want: %q", hostname, testSetup.hostname)
	}
	return nil
}

func runShortNameTest(ga []*apiBeta.GuestAttributesEntry, testSetup *inventoryTestSetup) error {
	var shortName string
	for _, item := range ga {
		if item.Key == "ShortName" {
			shortName = item.Value
			break
		}
	}

	if shortName == "" {
		s, _ := json.MarshalIndent(ga, "", "  ")
		return fmt.Errorf("ShortName not found in guestInventory: %s", s)
	}

	if shortName != testSetup.shortName {
		return fmt.Errorf("ShortName does not match expectation: got: %q, want: %q", shortName, testSetup.shortName)
	}
	return nil
}

func runPackagesTest(ga []*apiBeta.GuestAttributesEntry, testSetup *inventoryTestSetup) error {
	var packagesEncoded string
	for _, item := range ga {
		if item.Key == "InstalledPackages" {
			packagesEncoded = item.Value
			break
		}
	}

	if packagesEncoded == "" {
		s, _ := json.MarshalIndent(ga, "", "  ")
		return fmt.Errorf("InstalledPackages not found in guestInventory: %s", s)
	}

	decoded, err := base64.StdEncoding.DecodeString(packagesEncoded)
	if err != nil {
		return err
	}

	zr, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return err
	}
	defer zr.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zr); err != nil {
		return err
	}

	var pkgs packages.Packages
	if err := json.Unmarshal(buf.Bytes(), &pkgs); err != nil {
		return err
	}

	for _, pt := range testSetup.packageType {
		switch pt {
		case "googet":
			if len(pkgs.GooGet) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		case "deb":
			if len(pkgs.Deb) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		case "rpm":
			if len(pkgs.Rpm) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		case "pip":
			if len(pkgs.Pip) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		case "gem":
			if len(pkgs.Gem) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		case "wua":
			if len(pkgs.WUA) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		case "qfe":
			if len(pkgs.QFE) < 1 {
				return fmt.Errorf("no packages exported in InstalledPackages for %q", pt)
			}
		}
	}
	return nil
}

func inventoryTestCase(ctx context.Context, testSetup *inventoryTestSetup, tests chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp) {
	defer wg.Done()

	var logwg sync.WaitGroup
	inventoryTest := junitxml.NewTestCase(testSuiteName, fmt.Sprintf("[Guest Attributes inventory] [%s]", testSetup.testName))

	if inventoryTest.FilterTestCase(regex) {
		inventoryTest.Finish(tests)
		return
	}

	logger.Printf("Running TestCase %q", inventoryTest.Name)
	ga := runGatherInventoryTest(ctx, testSetup, inventoryTest, &logwg)
	if inventoryTest.Failure != nil {
		rerunTC := junitxml.NewTestCase(testSuiteName, strings.TrimPrefix(inventoryTest.Name, fmt.Sprintf("[%s] ", testSuiteName)))
		logger.Printf("Rerunning TestCase %q", rerunTC.Name)
		ga = runGatherInventoryTest(ctx, testSetup, rerunTC, &logwg)
		if rerunTC.Failure != nil {
			logger.Printf("TestCase %q finished in %fs", rerunTC.Name, rerunTC.Time)
			rerunTC.Finish(tests)
			return
		}
	}

	if err := runHostnameTest(ga, testSetup); err != nil {
		inventoryTest.WriteFailure("Error checking hostname: %v", err)
	}
	if err := runShortNameTest(ga, testSetup); err != nil {
		inventoryTest.WriteFailure("Error checking shortname: %v", err)
	}

	// Skip packages test for cos as it is not currently supported.
	if !strings.Contains(inventoryTest.Name, "cos") {
		if err := runPackagesTest(ga, testSetup); err != nil {
			inventoryTest.WriteFailure("Error checking packages: %v", err)
		}
	}

	logwg.Wait()
	inventoryTest.Finish(tests)
	logger.Printf("TestCase %q finished", inventoryTest.Name)
}
