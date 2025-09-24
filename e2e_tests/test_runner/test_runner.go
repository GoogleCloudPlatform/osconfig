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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/compute-image-tools/go/e2e_test_utils/junitxml"
)

const (
	timeout = 1 * time.Hour // Single test case timeout
)

// TestRunner is a test runner for e2e tests
type TestRunner struct { // TODO: Unify test suites implementation to use this runner
	TestSuiteName string
}

// RunTestCase wraps execution of a test case with timeout and retry logic as well as creating a JUnit XML record.
func (tr *TestRunner) RunTestCase(ctx context.Context, tc *junitxml.TestCase, f func(tc *junitxml.TestCase), tests chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp) {
	defer wg.Done()

	if tc.FilterTestCase(regex) {
		tc.Finish(tests)
	} else {
		logger.Printf("Running TestCase %q", tc.Name)

		runTestCase(ctx, tc, f, tests, wg, logger, regex)

		// Retry test if failure
		if tc.Failure != nil {
			rerunTC := junitxml.NewTestCase(tr.TestSuiteName, strings.TrimPrefix(tc.Name, fmt.Sprintf("[%s] ", tr.TestSuiteName)))
			logger.Printf("Rerunning TestCase %q", rerunTC.Name)
			runTestCase(ctx, rerunTC, f, tests, wg, logger, regex)
		}
	}
}

func runTestCase(ctx context.Context, tc *junitxml.TestCase, f func(tc *junitxml.TestCase), tests chan *junitxml.TestCase, wg *sync.WaitGroup, logger *log.Logger, regex *regexp.Regexp) {
	ctx, cancel := context.WithTimeout(ctx, timeout) // Timeout applied on single test execution
	defer cancel()

	select {
	case <-ctx.Done():
		logger.Printf("TestCase %q timed out after %v.", tc.Name, timeout)
		tc.WriteFailure("Test timed out.")
		tc.Finish(tests)
	case <-runAsync(tc, f):
		tc.Finish(tests)
		logger.Printf("TestCase %q finished in %fs", tc.Name, tc.Time)
	}
}

func runAsync(tc *junitxml.TestCase, f func(tc *junitxml.TestCase)) <-chan int {
	resultChan := make(chan int, 1)
	go func() {
		f(tc)
		resultChan <- 1
	}()
	return resultChan
}
