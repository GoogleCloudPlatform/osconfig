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

package test_suite_base

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	daisyCompute "github.com/GoogleCloudPlatform/compute-image-tools/daisy/compute"
	e2etestcompute "github.com/GoogleCloudPlatform/osconfig/e2etester/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2etester/config"
	"github.com/GoogleCloudPlatform/osconfig/e2etester/logger"
	"github.com/GoogleCloudPlatform/osconfig/e2etester/workflow"
	"github.com/google/uuid"
	"google.golang.org/api/compute/v1"
)

// A TestSuite describes the tests to run.
type TestSuite struct {
	// Name for this set of tests.
	Name string
	// Project pool to use
	Project string
	// Images to test.
	Images []string
	// Default zone to use.
	Zone string
	// The test cases to run.
	Tests map[string]*TestCase
	// How many tests to run in parallel.
	TestParallelCount int

	OAuthPath       string
	ComputeEndpoint string
}

// A TestCase is a single test to run.
type TestCase struct {
	// Path to the daisy workflow to use.
	// Each test workflow should manage its own resource creation and cleanup.
	W      *daisy.Workflow
	id     string
	logger *logger.TLogger

	// Default timeout is 2 hours.
	// Must be parsable by https://golang.org/pkg/time/#ParseDuration.
	TestTimeout string
	timeout     time.Duration

	// If set this test will be the only test allowed to run in the project.
	// This is required for any test that changes project level settings that may
	// impact other concurrent test runs.
	// TODO: allow setting these..
	ProjectLock       bool
	CustomProjectLock string
}

func CreateTestSuite(suiteName string, images []string, testHandle interface{}) (*TestSuite, error) {
	fmt.Printf("creating test suite: %s\n", testHandle)
	var t TestSuite

	t.Name = suiteName
	t.Project = *config.Project

	if len(images) == 0 {
		return nil, errors.New("no images provided")
	}
	t.Images = images

	if *config.Zone != "" {
		t.Zone = *config.Zone
	}
	if *config.Oauth != "" {
		t.OAuthPath = *config.Oauth
	}
	if *config.Ce != "" {
		t.ComputeEndpoint = *config.Ce
	}
	if *config.ParallelCount != 0 {
		t.TestParallelCount = *config.ParallelCount
	}

	if t.TestParallelCount == 0 {
		t.TestParallelCount = config.DefaultParallelCount
	}

	var regex *regexp.Regexp
	if *config.Filter != "" {
		var err error
		regex, err = regexp.Compile(*config.Filter)
		if err != nil {
			fmt.Println("-filter flag not valid:", err)
			os.Exit(1)
		}
	}

	fmt.Printf("[TestRunner] Creating test cases\n")

	workflows, err := workflow.CreateTestWorkflows(testHandle, images, regex)
	if err != nil {
		return nil, err
	}

	t.Tests = make(map[string]*TestCase)
	for name, w := range workflows {
		t.Tests[name] = &TestCase{W: w}
	}

	for name, test := range t.Tests {
		test.id = uuid.New().String()
		if test.TestTimeout == "" {
			test.timeout = defaultTimeout
		} else {
			d, err := time.ParseDuration(test.TestTimeout)
			if err != nil {
				test.timeout = defaultTimeout
			} else {
				test.timeout = d
			}
		}

		fmt.Printf("  - Creating test case for %q\n", name)

		test.logger = &logger.TLogger{}

		rand.Seed(time.Now().UnixNano())
		test.W.Project = t.Project
		test.W.Zone = t.Zone
		test.W.OAuthPath = t.OAuthPath
		test.W.ComputeEndpoint = t.ComputeEndpoint
		test.W.DisableGCSLogging()
		test.W.DisableCloudLogging()
		test.W.DisableStdoutLogging()
		test.W.Logger = test.logger
	}

	return &t, nil
}

func CheckError(errors chan error) {
	select {
	case err := <-errors:
		fmt.Fprintln(os.Stderr, "\n[TestRunner] Errors in one or more test cases:")
		fmt.Fprintln(os.Stderr, "\n - ", err)
		for {
			select {
			case err := <-errors:
				fmt.Fprintln(os.Stderr, "\n - ", err)
				continue
			default:
				fmt.Fprintln(os.Stderr, "\n[TestRunner] Exiting with exit code 1")
				os.Exit(1)
			}
		}
	default:
		return
	}
}

type test struct {
	name     string
	testCase *TestCase
}

func delItem(items []*compute.MetadataItems, i int) []*compute.MetadataItems {
	// Delete the element.
	// https://github.com/golang/go/wiki/SliceTricks
	copy(items[i:], items[i+1:])
	items[len(items)-1] = nil
	return items[:len(items)-1]
}

func isExpired(val string) bool {
	t, err := time.Parse(config.TimeFormat, val)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

const (
	writeLock      = "TestWriteLock-"
	readLock       = "TestReadLock-"
	defaultTimeout = 2 * time.Hour
)

func waitLock(client daisyCompute.Client, project string, prefix ...string) (*compute.Metadata, error) {
	var md *compute.Metadata
	var err error
Loop:
	for {
		md, err = e2etestcompute.GetCommonInstanceMetadata(client, project)
		if err != nil {
			return nil, err
		}

		for i, mdi := range md.Items {
			if mdi != nil {
				for _, p := range prefix {
					if strings.HasPrefix(mdi.Key, p) {
						if isExpired(*mdi.Value) {
							md.Items = delItem(md.Items, i)
						} else {
							r := rand.Intn(10) + 5
							time.Sleep(time.Duration(r) * time.Second)
							continue Loop
						}
					}
				}
			}
		}
		return md, nil
	}
}

func projectReadLock(client daisyCompute.Client, project, key string, timeout time.Duration) (string, error) {
	md, err := waitLock(client, project, writeLock)
	if err != nil {
		return "", err
	}

	lock := readLock + key
	val := time.Now().Add(timeout).Format(config.TimeFormat)
	md.Items = append(md.Items, &compute.MetadataItems{Key: lock, Value: &val})
	if err := client.SetCommonInstanceMetadata(project, md); err != nil {
		return "", err
	}
	return lock, nil
}

func customProjectWriteLock(client daisyCompute.Client, project, custom, key string, timeout time.Duration) (string, error) {
	customLock := readLock + custom
	md, err := waitLock(client, project, writeLock, customLock)
	if err != nil {
		return "", err
	}

	lock := customLock + key
	val := time.Now().Add(timeout).Format(config.TimeFormat)
	md.Items = append(md.Items, &compute.MetadataItems{Key: lock, Value: &val})
	if err := client.SetCommonInstanceMetadata(project, md); err != nil {
		return "", err
	}

	return lock, nil
}

func projectWriteLock(client daisyCompute.Client, project, key string, timeout time.Duration) (string, error) {
	md, err := waitLock(client, project, writeLock)
	if err != nil {
		return "", err
	}

	// This means the project has no current write locks, set the write lock
	// now and then wait till all current read locks are gone.
	lock := writeLock + key
	val := time.Now().Add(timeout).Format(config.TimeFormat)
	md.Items = append(md.Items, &compute.MetadataItems{Key: lock, Value: &val})
	if err := client.SetCommonInstanceMetadata(project, md); err != nil {
		return "", err
	}

	if _, err := waitLock(client, project, readLock); err != nil {
		// Attempt to unlock.
		projectUnlock(client, project, lock)
		return "", err
	}
	return lock, nil
}

func projectUnlock(client daisyCompute.Client, project, lock string) error {
	md, err := e2etestcompute.GetCommonInstanceMetadata(client, project)
	if err != nil {
		return err
	}

	for i, mdi := range md.Items {
		if mdi != nil && lock == mdi.Key {
			md.Items = delItem(md.Items, i)
		}
	}

	return client.SetCommonInstanceMetadata(project, md)
}

var allowedChars = regexp.MustCompile("[^-_a-zA-Z0-9]+")
