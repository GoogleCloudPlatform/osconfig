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

package junitxml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestReporterRecordsAndFormatsXML(t *testing.T) {
	reporter := NewReporter()
	reporter.Record("OSInventory", "debian-12", "e2etests_test", 1500*time.Millisecond, "", false, "stdout output")
	reporter.Record("OSInventory", "rhel-9", "e2etests_test", 2500*time.Millisecond, "mismatch on short name", false, "stderr output")
	reporter.Record("OSInventory", "cos-stable", "e2etests_test", 500*time.Millisecond, "", true, "")

	var buf bytes.Buffer
	if err := reporter.WriteXML(&buf); err != nil {
		t.Fatalf("WriteXML() error = %v", err)
	}

	content := buf.String()
	if !strings.HasPrefix(content, xml.Header) {
		t.Fatalf("WriteXML() output missing XML header: %s", content)
	}

	var root TestSuites
	if err := xml.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatalf("xml.Unmarshal() error = %v", err)
	}

	if root.Tests != 3 {
		t.Errorf("root.Tests = %d, want 3", root.Tests)
	}
	if root.Failures != 1 {
		t.Errorf("root.Failures = %d, want 1", root.Failures)
	}
	if len(root.Suites) != 1 {
		t.Fatalf("len(root.Suites) = %d, want 1", len(root.Suites))
	}

	suite := root.Suites[0]
	if suite.Name != "OSInventory" {
		t.Errorf("suite.Name = %q, want OSInventory", suite.Name)
	}
	if suite.Skipped != 1 {
		t.Errorf("suite.Skipped = %d, want 1", suite.Skipped)
	}
	if len(suite.TestCases) != 3 {
		t.Fatalf("len(suite.TestCases) = %d, want 3", len(suite.TestCases))
	}

	tc1 := suite.TestCases[0]
	if tc1.Name != "debian-12" || tc1.Failure != nil || tc1.Skipped != nil {
		t.Errorf("tc1 = %+v, want passing debian-12", tc1)
	}

	tc2 := suite.TestCases[1]
	if tc2.Name != "rhel-9" || tc2.Failure == nil || tc2.Failure.Message != "mismatch on short name" {
		t.Errorf("tc2 = %+v, want failed rhel-9", tc2)
	}

	tc3 := suite.TestCases[2]
	if tc3.Name != "cos-stable" || tc3.Skipped == nil {
		t.Errorf("tc3 = %+v, want skipped cos-stable", tc3)
	}
}
