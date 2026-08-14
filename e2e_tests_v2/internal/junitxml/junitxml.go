//  Copyright 2026 Google LLC
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

// Package junitxml provides helpers to generate standard JUnit XML test reports.
package junitxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TestSuites is the root XML container for JUnit reports.
type TestSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Suites   []*TestSuite `xml:"testsuite"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Disabled int          `xml:"disabled,attr"`
	Time     float64      `xml:"time,attr"`
}

// TestSuite represents a test suite in JUnit XML format.
type TestSuite struct {
	XMLName   xml.Name    `xml:"testsuite"`
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Errors    int         `xml:"errors,attr"`
	Disabled  int         `xml:"disabled,attr"`
	Skipped   int         `xml:"skipped,attr"`
	Time      float64     `xml:"time,attr"`
	Timestamp string      `xml:"timestamp,attr"`
	TestCases []*TestCase `xml:"testcase"`
}

// TestCase represents an individual test case.
type TestCase struct {
	XMLName   xml.Name `xml:"testcase"`
	Classname string   `xml:"classname,attr"`
	Name      string   `xml:"name,attr"`
	Time      float64  `xml:"time,attr"`
	Failure   *Failure `xml:"failure,omitempty"`
	Skipped   *Skipped `xml:"skipped,omitempty"`
	SystemOut string   `xml:"system-out,omitempty"`
}

// Failure contains details about a test failure.
type Failure struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Content string `xml:",chardata"`
}

// Skipped indicates a skipped test.
type Skipped struct {
	Message string `xml:"message,attr,omitempty"`
}

// Reporter collects test case executions and writes standard JUnit XML files.
type Reporter struct {
	mu     sync.Mutex
	suites map[string]*TestSuite
}

// NewReporter creates a new JUnit reporter.
func NewReporter() *Reporter {
	return &Reporter{
		suites: make(map[string]*TestSuite),
	}
}

// Record records a test case result in the specified test suite.
func (r *Reporter) Record(suiteName, caseName, classname string, duration time.Duration, failureMsg string, skipped bool, systemOut string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	suite, ok := r.suites[suiteName]
	if !ok {
		suite = &TestSuite{
			Name:      suiteName,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		r.suites[suiteName] = suite
	}

	tc := &TestCase{
		Classname: classname,
		Name:      caseName,
		Time:      duration.Seconds(),
		SystemOut: systemOut,
	}
	suite.Tests++
	suite.Time += duration.Seconds()

	if skipped {
		suite.Skipped++
		tc.Skipped = &Skipped{Message: "Test skipped"}
	} else if failureMsg != "" {
		suite.Failures++
		tc.Failure = &Failure{
			Message: failureMsg,
			Type:    "failure",
			Content: failureMsg,
		}
	}

	suite.TestCases = append(suite.TestCases, tc)
}

// WriteXML serializes all recorded test results as XML into writer w.
func (r *Reporter) WriteXML(w io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var totalTests, totalFailures, totalErrors, totalDisabled int
	var totalTime float64
	var suites []*TestSuite

	for _, suite := range r.suites {
		totalTests += suite.Tests
		totalFailures += suite.Failures
		totalErrors += suite.Errors
		totalDisabled += suite.Disabled
		totalTime += suite.Time
		suites = append(suites, suite)
	}

	root := TestSuites{
		Suites:   suites,
		Tests:    totalTests,
		Failures: totalFailures,
		Errors:   totalErrors,
		Disabled: totalDisabled,
		Time:     totalTime,
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// WriteToFile writes the JUnit XML document to the given file path.
func (r *Reporter) WriteToFile(filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("create directory for junit xml %q: %w", filePath, err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create junit xml file %q: %w", filePath, err)
	}
	defer f.Close()
	return r.WriteXML(f)
}
