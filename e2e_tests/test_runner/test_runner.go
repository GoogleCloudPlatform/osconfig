//  Copyright 2019 Google Inc. All Rights Reserved.
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

// Package testrunner contains a wrapper for e2e test execution
package testrunner

import (
	"context"
	"fmt"
	"log"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	gcpclients "github.com/GoogleCloudPlatform/osconfig/e2e_tests/gcp_clients"
	testconfig "github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_config"
	utils "github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	computeApi "google.golang.org/api/compute/v1"

	"github.com/GoogleCloudPlatform/compute-image-tools/go/e2e_test_utils/junitxml"
)

const (
	timeout = 1 * time.Hour // Single test case timeout
)

// TestRunner is a test runner for e2e tests
type TestRunner struct { // TODO: Unify test suites implementation to use this runner
	TestSuiteName string
}

// ComputeInstanceMetadata contains metadata for VM instance creation.
type ComputeInstanceMetadata struct {
	Metadata           []*computeApi.MetadataItems
	MachineType        string
	Image              string
	InstanceNamePrefix string
}

// RunTestCase wraps execution of a test case with timeout and retry logic as well as creating a JUnit XML record.
func (tr *TestRunner) RunTestCase(ctx context.Context, tc *junitxml.TestCase, f func(tc *junitxml.TestCase, inst *compute.Instance, testProjectConfig *testconfig.Project, instName string), tests chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp, computeInstanceMetadata *ComputeInstanceMetadata) {
	defer wg.Done()

	if tc.FilterTestCase(regex) {
		tc.Finish(tests)
	} else {
		logger.Printf("Running TestCase %q", tc.Name)

		tr.runTestCase(ctx, tc, f, tests, logger, computeInstanceMetadata)

		// Retry test if failure
		if tc.Failure != nil {
			rerunTC := junitxml.NewTestCase(tr.TestSuiteName, strings.TrimPrefix(tc.Name, fmt.Sprintf("[%s] ", tr.TestSuiteName)))
			logger.Printf("Rerunning TestCase %q", rerunTC.Name)
			tr.runTestCase(ctx, rerunTC, f, tests, logger, computeInstanceMetadata)
		}
	}
}

func (tr *TestRunner) runTestCase(ctx context.Context, tc *junitxml.TestCase, f func(tc *junitxml.TestCase, inst *compute.Instance, testProjectConfig *testconfig.Project, instName string), tests chan *junitxml.TestCase, logger *log.Logger, computeInstanceMetadata *ComputeInstanceMetadata) {
	ctx, cancel := context.WithTimeout(ctx, timeout) // Timeout applied on single test execution
	defer cancel()

	testProjectConfig := testconfig.GetProject()
	zone := testProjectConfig.AcquireZone()
	defer testProjectConfig.ReleaseZone(zone)

	instanceName := fmt.Sprintf("%s-%s", computeInstanceMetadata.InstanceNamePrefix, utils.RandString(5))
	inst, err := createComputeInstance(tc, instanceName, computeInstanceMetadata.Image, computeInstanceMetadata.MachineType, zone, computeInstanceMetadata.Metadata, testProjectConfig, logger)
	defer inst.Cleanup()
	defer inst.RecordSerialOutput(ctx, path.Join(*config.OutDir, tr.TestSuiteName), 1)

	if err != nil {
		return
	}

	select {
	case <-ctx.Done():
		logger.Printf("TestCase %q timed out after %v.", tc.Name, timeout)
		tc.WriteFailure("Test timed out.")
		tc.Finish(tests)
	case <-runAsync(tc, inst, testProjectConfig, instanceName, f):
		tc.Finish(tests)
		logger.Printf("TestCase %q finished in %fs", tc.Name, tc.Time)
	}
}

func runAsync(tc *junitxml.TestCase, inst *compute.Instance, testProjectConfig *testconfig.Project, instName string, f func(tc *junitxml.TestCase, inst *compute.Instance, testProjectConfig *testconfig.Project, instName string)) <-chan int {
	resultChan := make(chan int, 1)
	go func() {
		f(tc, inst, testProjectConfig, instName)
		resultChan <- 1
	}()
	return resultChan
}

func createComputeInstance(tc *junitxml.TestCase, instanceName, image, machineType, zone string, metadata []*computeApi.MetadataItems, testProjectConfig *testconfig.Project, logger *log.Logger) (*compute.Instance, error) {
	computeClient, err := gcpclients.GetComputeClient()
	if err != nil {
		tc.WriteFailure("Error getting compute client: %v", err)
		return nil, err
	}

	tc.Logf("Creating instance %q with image %q", instanceName, image)
	inst, err := utils.CreateComputeInstance(metadata, computeClient, machineType, image, instanceName, testProjectConfig.TestProjectID, zone, testProjectConfig.ServiceAccountEmail, testProjectConfig.ServiceAccountScopes)

	if err != nil {
		tc.WriteFailure("Error creating instance: %v", utils.GetStatusFromError(err))
		return nil, err
	}

	tc.Logf("Waiting for agent install to complete")
	if _, err := inst.WaitForGuestAttributes("osconfig_tests/install_done", 5*time.Second, 25*time.Minute); err != nil {
		tc.WriteFailure("Error waiting for osconfig agent install: %v", err)
		return nil, err
	}
	tc.Logf("Agent installed successfully")

	return inst, nil
}
