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

package testenv

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestResourceNameIsValidAndBounded(t *testing.T) {
	got := resourceName("20260813t120000z", "debian-12-very-long-test-name-that-exceeds-standard-lengths", "a1b2c3d4")
	if len(got) > 63 {
		t.Fatalf("resourceName() length = %d > 63: %q", len(got), got)
	}
	if unsafeResourceName.MatchString(got) || strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Fatalf("resourceName() is invalid: %q", got)
	}
}

func TestLabelValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Debian 12 Test!", "debian-12-test"},
		{"---leading-trailing---", "leading-trailing"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		got := labelValue(tt.input)
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("labelValue(%q) mismatch (-want +got):\n%s", tt.input, diff)
		}
	}
}

func TestDecodeInstalledPackagesValid(t *testing.T) {
	pkgs := InstalledPackages{
		Deb: []*PackageInfo{
			{Name: "bash", Arch: "amd64", Version: "5.2.15"},
			{Name: "google-osconfig-agent", Arch: "amd64", Version: "20260701.00"},
		},
		Rpm: []*PackageInfo{
			{Name: "kernel", Arch: "x86_64", Version: "5.14.0"},
		},
		GooGet: []*PackageInfo{
			{Name: "googet-pkg", Arch: "x86_64", Version: "1.0.0"},
		},
		WUA: []*WUAPackage{
			{Title: "Security Update KB123456"},
		},
		QFE: []*QFEPackage{
			{HotFixID: "KB123456"},
		},
		COS: []*PackageInfo{
			{Name: "app-shells/bash", Arch: "x86_64", Version: "5.1"},
		},
		Pip: []*PackageInfo{
			{Name: "requests", Version: "2.31.0"},
		},
	}

	data, err := json.Marshal(pkgs)
	if err != nil {
		t.Fatalf("marshal packages: %v", err)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	decoded, err := DecodeInstalledPackages(encoded)
	if err != nil {
		t.Fatalf("DecodeInstalledPackages() error = %v", err)
	}

	if diff := cmp.Diff(pkgs.Deb, decoded.Deb); diff != "" {
		t.Errorf("Deb packages mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(pkgs.Rpm, decoded.Rpm); diff != "" {
		t.Errorf("Rpm packages mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(pkgs.COS, decoded.COS); diff != "" {
		t.Errorf("COS packages mismatch (-want +got):\n%s", diff)
	}

	types := decoded.PackageTypes()
	wantTypes := []string{"cos", "deb", "googet", "pip", "qfe", "rpm", "wua"}
	sortOpt := cmpopts.SortSlices(func(a, b string) bool { return a < b })
	if diff := cmp.Diff(wantTypes, types, sortOpt); diff != "" {
		t.Errorf("PackageTypes() mismatch (-want +got):\n%s", diff)
	}

	osTypes := decoded.OSPackageTypes()
	wantOSTypes := []string{"cos", "deb", "googet", "qfe", "rpm", "wua"}
	if diff := cmp.Diff(wantOSTypes, osTypes, sortOpt); diff != "" {
		t.Errorf("OSPackageTypes() mismatch (-want +got):\n%s", diff)
	}
}

func TestDecodeInstalledPackagesInvalid(t *testing.T) {
	if _, err := DecodeInstalledPackages(""); err == nil {
		t.Error("expected error for empty string")
	}
	if _, err := DecodeInstalledPackages("invalid-base64-!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
	if _, err := DecodeInstalledPackages(base64.StdEncoding.EncodeToString([]byte("not-gzip"))); err == nil {
		t.Error("expected error for invalid gzip")
	}
}

func TestStepExecution(t *testing.T) {
	test := &Test{
		t:       t,
		Context: t.Context(),
	}

	executed := false
	test.Step("simple step", func(ctx context.Context) error {
		executed = true
		return nil
	})

	if !executed {
		t.Error("expected step function to execute")
	}
}
