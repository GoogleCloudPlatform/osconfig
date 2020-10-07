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

package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/GoogleCloudPlatform/compute-image-tools/go/e2e_test_utils/junitxml"
	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	gcpclients "github.com/GoogleCloudPlatform/osconfig/e2e_tests/gcp_clients"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_suites/guestpolicies"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_suites/inventory"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_suites/inventoryreporting"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/test_suites/patch"

	_ "google.golang.org/genproto/googleapis/rpc/errdetails"
)

var testFunctions = []func(context.Context, *sync.WaitGroup, chan *junitxml.TestSuite, *log.Logger, *regexp.Regexp, *regexp.Regexp){
	guestpolicies.TestSuite,
	inventory.TestSuite,
	inventoryreporting.TestSuite,
	patch.TestSuite,
}

type logWriter struct {
	log *log.Logger
}

func (l *logWriter) Write(b []byte) (int, error) {
	l.log.Print(string(b))
	return len(b), nil
}

func main() {
	ctx := context.Background()

	if err := gcpclients.PopulateClients(ctx); err != nil {
		log.Fatal(err)
	}

	testLogger := log.New(os.Stdout, "[OsConfigTests] ", 0)
	testLogger.Println("Starting...")

	// Initialize logger for any shared function calls.
	opts := logger.LogOpts{LoggerName: "OsConfigTests", Debug: true, Writers: []io.Writer{&logWriter{log: testLogger}}, DisableCloudLogging: true, DisableLocalLogging: true}
	logger.Init(ctx, opts)

	tests := make(chan *junitxml.TestSuite)
	var wg sync.WaitGroup
	for _, tf := range testFunctions {
		wg.Add(1)
		go tf(ctx, &wg, tests, testLogger, config.TestSuiteFilter(), config.TestCaseFilter())
	}
	go func() {
		wg.Wait()
		close(tests)
	}()

	var testSuites []*junitxml.TestSuite
	for ret := range tests {
		testSuites = append(testSuites, ret)
		testSuiteOutPath := filepath.Join(*config.OutDir, fmt.Sprintf("junit_%s.xml", ret.Name))
		if err := os.MkdirAll(filepath.Dir(testSuiteOutPath), 0770); err != nil {
			testLogger.Fatal(err)
		}

		testLogger.Printf("Creating junit xml file: %s", testSuiteOutPath)
		d, err := xml.MarshalIndent(ret, "  ", "   ")
		if err != nil {
			testLogger.Fatal(err)
		}

		if err := ioutil.WriteFile(testSuiteOutPath, d, 0644); err != nil {
			testLogger.Fatal(err)
		}
	}

	var buf bytes.Buffer
	for _, ts := range testSuites {
		if ts.Failures > 0 {
			buf.WriteString(fmt.Sprintf("TestSuite %q has errors:\n", ts.Name))
			for _, tc := range ts.TestCase {
				if tc.Failure != nil {
					buf.WriteString(fmt.Sprintf(" - %q: %s\n", tc.Name, tc.Failure.FailMessage))
				}
			}

		}
	}

	if buf.Len() > 0 {
		testLogger.Fatalf("%sExiting with exit code 1", buf.String())
	}
	testLogger.Print("All test cases completed successfully.")
}
