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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/osconfig/v1"
)

func TestLabelValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "string with uppercase and special characters, returns sanitized lowercase value",
			input: "Debian 12 Test!",
			want:  "debian-12-test",
		},
		{
			name:  "string with leading and trailing dashes, returns trimmed value",
			input: "---leading-trailing---",
			want:  "leading-trailing",
		},
		{
			name:  "empty input string, returns fallback unknown",
			input: "",
			want:  "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := labelValue(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("labelValue(%q) mismatch (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func gzipBase64(t *testing.T, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestDecodeInstalledPackages(t *testing.T) {
	validPkgs := &InstalledPackages{
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

	validJSON, err := json.Marshal(validPkgs)
	if err != nil {
		t.Fatalf("marshal valid packages: %v", err)
	}

	tests := []struct {
		name             string
		input            string
		wantErr          error
		wantPkgs         *InstalledPackages
		wantPackageTypes []string
		wantOSTypes      []string
		wantPackages     []string
		wantPackagesIn   map[string][]string
		wantQFE          bool
		wantWUA          bool
		wantTotalPkgs    int
	}{
		{
			name:             "valid full package data payload, returns populated InstalledPackages struct",
			input:            gzipBase64(t, validJSON),
			wantErr:          nil,
			wantPkgs:         validPkgs,
			wantPackageTypes: []string{"cos", "deb", "googet", "pip", "qfe", "rpm", "wua"},
			wantOSTypes:      []string{"cos", "deb", "googet", "qfe", "rpm", "wua"},
			wantPackages:     []string{"bash", "GOOGLE-OSCONFIG-AGENT", "app-shells/bash"},
			wantPackagesIn: map[string][]string{
				"deb": {"bash"},
				"cos": {"app-shells/bash"},
			},
			wantQFE:       true,
			wantWUA:       true,
			wantTotalPkgs: 6,
		},
		{
			name:          "valid empty JSON payload, returns empty InstalledPackages struct",
			input:         gzipBase64(t, []byte("{}")),
			wantErr:       nil,
			wantPkgs:      &InstalledPackages{},
			wantTotalPkgs: 0,
		},
		{
			name:    "empty input string, returns package data attribute is empty error",
			input:   "",
			wantErr: errors.New("package data attribute is empty"),
		},
		{
			name:    "whitespace only input string, returns package data attribute is empty error",
			input:   "   \t\n",
			wantErr: errors.New("package data attribute is empty"),
		},
		{
			name:    "invalid base64 string, returns base64 decode error",
			input:   "invalid-base64-!!!",
			wantErr: errors.New("base64 decode: illegal base64 data at input byte 7"),
		},
		{
			name:    "invalid gzip payload, returns gzip reader error",
			input:   base64.StdEncoding.EncodeToString([]byte("not-gzip-data")),
			wantErr: errors.New("gzip reader: gzip: invalid header"),
		},
		{
			name:    "invalid JSON inside gzip payload, returns unmarshal JSON error",
			input:   gzipBase64(t, []byte("{invalid-json:")),
			wantErr: errors.New("unmarshal JSON: invalid character 'i' looking for beginning of object key string"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPkgs, gotErr := DecodeInstalledPackages(tc.input)
			if (gotErr == nil && tc.wantErr != nil) || (gotErr != nil && tc.wantErr == nil) || (gotErr != nil && tc.wantErr != nil && gotErr.Error() != tc.wantErr.Error()) {
				t.Fatalf("Expected error mismatch: got error: %v, want error: %v", gotErr, tc.wantErr)
			}

			if tc.wantPkgs != nil {
				if diff := cmp.Diff(tc.wantPkgs, gotPkgs); diff != "" {
					t.Errorf("InstalledPackages mismatch (-want +got):\n%s", diff)
				}
			}

			sortOpt := cmpopts.SortSlices(func(a, b string) bool { return a < b })
			if tc.wantPackageTypes != nil {
				if diff := cmp.Diff(tc.wantPackageTypes, gotPkgs.PackageTypes(), sortOpt); diff != "" {
					t.Errorf("PackageTypes() mismatch (-want +got):\n%s", diff)
				}
			}

			if tc.wantOSTypes != nil {
				if diff := cmp.Diff(tc.wantOSTypes, gotPkgs.OSPackageTypes(), sortOpt); diff != "" {
					t.Errorf("OSPackageTypes() mismatch (-want +got):\n%s", diff)
				}
			}

			if len(tc.wantPackages) > 0 {
				if !gotPkgs.HasPackages(tc.wantPackages...) {
					t.Errorf("expected HasPackages(%v) to be true", tc.wantPackages)
				}
			}

			for manager, pkgs := range tc.wantPackagesIn {
				for _, pkg := range pkgs {
					if !gotPkgs.HasPackageIn(manager, pkg) {
						t.Errorf("expected HasPackageIn(%q, %q) to be true", manager, pkg)
					}
				}
			}

			if gotPkgs.HasQFE() != tc.wantQFE {
				t.Errorf("HasQFE() = %v, want %v", gotPkgs.HasQFE(), tc.wantQFE)
			}

			if gotPkgs.HasWUA() != tc.wantWUA {
				t.Errorf("HasWUA() = %v, want %v", gotPkgs.HasWUA(), tc.wantWUA)
			}

			if len(gotPkgs.AllPackageNames()) != tc.wantTotalPkgs {
				t.Errorf("AllPackageNames() returned %d names, want %d", len(gotPkgs.AllPackageNames()), tc.wantTotalPkgs)
			}
		})
	}
}

func TestExtractInstalledPackages(t *testing.T) {
	inv := &osconfig.Inventory{
		OsInfo: &osconfig.InventoryOsInfo{
			Hostname:  "test-instance",
			ShortName: "debian",
		},
		Items: map[string]osconfig.InventoryItem{
			"pkg-deb-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					AptPackage: &osconfig.InventoryVersionedPackage{
						PackageName:  "bash",
						Architecture: "amd64",
						Version:      "5.2.15",
					},
				},
			},
			"pkg-rpm-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					YumPackage: &osconfig.InventoryVersionedPackage{
						PackageName:  "kernel",
						Architecture: "x86_64",
						Version:      "5.14.0",
					},
				},
			},
			"pkg-zypper-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					ZypperPackage: &osconfig.InventoryVersionedPackage{
						PackageName:  "systemd",
						Architecture: "x86_64",
						Version:      "254",
					},
				},
			},
			"pkg-cos-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					CosPackage: &osconfig.InventoryVersionedPackage{
						PackageName:  "app-shells/bash",
						Architecture: "x86_64",
						Version:      "5.1",
					},
				},
			},
			"pkg-googet-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					GoogetPackage: &osconfig.InventoryVersionedPackage{
						PackageName:  "googet-pkg",
						Architecture: "x86_64",
						Version:      "1.0.0",
					},
				},
			},
			"pkg-wua-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					WuaPackage: &osconfig.InventoryWindowsUpdatePackage{
						Title: "Security Update KB123456",
					},
				},
			},
			"pkg-qfe-1": {
				Type: "INSTALLED_PACKAGE",
				InstalledPackage: &osconfig.InventorySoftwarePackage{
					QfePackage: &osconfig.InventoryWindowsQuickFixEngineeringPackage{
						HotFixId: "KB123456",
					},
				},
			},
		},
	}

	pkgs := ExtractInstalledPackages(inv)
	if pkgs == nil {
		t.Fatal("expected non-nil pkgs")
	}

	if !pkgs.HasPackages("bash", "kernel", "systemd", "app-shells/bash", "googet-pkg") {
		t.Error("expected HasPackages to find all installed packages")
	}
	if !pkgs.HasQFE() {
		t.Error("expected HasQFE() to be true")
	}
	if !pkgs.HasWUA() {
		t.Error("expected HasWUA() to be true")
	}

	types := pkgs.OSPackageTypes()
	wantTypes := []string{"cos", "deb", "googet", "qfe", "rpm", "wua", "zypper"}
	sortOpt := cmpopts.SortSlices(func(a, b string) bool { return a < b })
	if diff := cmp.Diff(wantTypes, types, sortOpt); diff != "" {
		t.Errorf("OSPackageTypes() mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractAvailablePackages(t *testing.T) {
	inv := &osconfig.Inventory{
		Items: map[string]osconfig.InventoryItem{
			"pkg-update-1": {
				Type: "AVAILABLE_PACKAGE",
				AvailablePackage: &osconfig.InventorySoftwarePackage{
					AptPackage: &osconfig.InventoryVersionedPackage{
						PackageName:  "curl",
						Architecture: "amd64",
						Version:      "7.88.1-10+deb12u5",
					},
				},
			},
		},
	}

	updates := ExtractAvailablePackages(inv)
	if updates == nil {
		t.Fatal("expected non-nil updates")
	}
	if !updates.HasPackages("curl") {
		t.Error("expected HasPackages(\"curl\") to be true for updates")
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
