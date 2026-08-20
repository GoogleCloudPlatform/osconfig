//go:build e2e

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

package e2etests_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/osconfig/v1"
)

type inventoryTestCase struct {
	name             string
	image            string
	wantShortName    string
	wantPackageTypes []string
	wantPackages     []string
	requireQFE       bool
	requireWUA       bool
	machineType      string
	timeout          time.Duration
}

// TestOSInventory verifies that the OS Config agent reports system inventory
// (Hostname, ShortName, InstalledPackages) via the public OS Config API.
// This test directly replaces test_suites/inventory/inventory.go using standard Go testing.
func TestOSInventory(t *testing.T) {
	testCases := []inventoryTestCase{
		// Debian
		{
			name:             "debian-11",
			image:            "projects/debian-cloud/global/images/family/debian-11",
			wantShortName:    "debian",
			wantPackageTypes: []string{"deb"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "debian-12",
			image:            "projects/debian-cloud/global/images/family/debian-12",
			wantShortName:    "debian",
			wantPackageTypes: []string{"deb"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		// Ubuntu
		{
			name:             "ubuntu-2204-lts",
			image:            "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			wantShortName:    "ubuntu",
			wantPackageTypes: []string{"deb"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "ubuntu-2404-lts",
			image:            "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			wantShortName:    "ubuntu",
			wantPackageTypes: []string{"deb"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		// Enterprise Linux (RHEL, CentOS Stream, Rocky Linux)
		{
			name:             "rhel-8",
			image:            "projects/rhel-cloud/global/images/family/rhel-8",
			wantShortName:    "rhel",
			wantPackageTypes: []string{"rpm"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "rhel-9",
			image:            "projects/rhel-cloud/global/images/family/rhel-9",
			wantShortName:    "rhel",
			wantPackageTypes: []string{"rpm"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "centos-stream-9",
			image:            "projects/centos-cloud/global/images/family/centos-stream-9",
			wantShortName:    "centos",
			wantPackageTypes: []string{"rpm"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "rocky-linux-8",
			image:            "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
			wantShortName:    "rocky",
			wantPackageTypes: []string{"rpm"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "rocky-linux-9",
			image:            "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
			wantShortName:    "rocky",
			wantPackageTypes: []string{"rpm"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		// SUSE / openSUSE
		{
			name:             "sles-12",
			image:            "projects/suse-cloud/global/images/family/sles-12",
			wantShortName:    "sles",
			wantPackageTypes: []string{"zypper"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "sles-15",
			image:            "projects/suse-cloud/global/images/family/sles-15",
			wantShortName:    "sles",
			wantPackageTypes: []string{"zypper"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "opensuse-leap-15",
			image:            "projects/opensuse-cloud/global/images/family/opensuse-leap",
			wantShortName:    "opensuse-leap",
			wantPackageTypes: []string{"zypper"},
			wantPackages:     []string{"bash", "systemd"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		// Windows Server
		{
			name:             "windows-2016",
			image:            "projects/windows-cloud/global/images/family/windows-2016",
			wantShortName:    "windows",
			wantPackageTypes: []string{"googet", "qfe", "wua"},
			wantPackages:     []string{"googet"},
			requireQFE:       true,
			requireWUA:       true,
			machineType:      "e2-standard-4",
			timeout:          20 * time.Minute,
		},
		{
			name:             "windows-2019",
			image:            "projects/windows-cloud/global/images/family/windows-2019",
			wantShortName:    "windows",
			wantPackageTypes: []string{"googet", "qfe", "wua"},
			wantPackages:     []string{"googet"},
			requireQFE:       true,
			requireWUA:       true,
			machineType:      "e2-standard-4",
			timeout:          20 * time.Minute,
		},
		{
			name:             "windows-2022",
			image:            "projects/windows-cloud/global/images/family/windows-2022",
			wantShortName:    "windows",
			wantPackageTypes: []string{"googet", "qfe", "wua"},
			wantPackages:     []string{"googet"},
			requireQFE:       true,
			requireWUA:       true,
			machineType:      "e2-standard-4",
			timeout:          20 * time.Minute,
		},
		// Container-Optimized OS (COS)
		{
			name:             "cos-stable",
			image:            "projects/cos-cloud/global/images/family/cos-stable",
			wantShortName:    "cos",
			wantPackageTypes: []string{"cos"},
			wantPackages:     []string{"app-shells/bash"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "cos-beta",
			image:            "projects/cos-cloud/global/images/family/cos-beta",
			wantShortName:    "cos",
			wantPackageTypes: []string{"cos"},
			wantPackages:     []string{"app-shells/bash"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
		{
			name:             "cos-dev",
			image:            "projects/cos-cloud/global/images/family/cos-dev",
			wantShortName:    "cos",
			wantPackageTypes: []string{"cos"},
			wantPackages:     []string{"app-shells/bash"},
			machineType:      "e2-standard-2",
			timeout:          10 * time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, nil)
				return err
			})

			var inv *osconfig.Inventory
			test.Step("wait for inventory", func(ctx context.Context) error {
				var err error
				inv, err = test.WaitForInventory(vm)
				return err
			})

			test.Step("validate OsInfo names", func(ctx context.Context) error {
				if inv == nil || inv.OsInfo == nil {
					t.Fatalf("inventory or OsInfo is nil")
				}
				if diff := cmp.Diff(vm.Name, inv.OsInfo.Hostname); diff != "" {
					t.Errorf("hostname mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.wantShortName, inv.OsInfo.ShortName); diff != "" {
					t.Errorf("short name mismatch (-want +got):\n%s", diff)
				}
				return nil
			})

			test.Step("verify installed packages", func(ctx context.Context) error {
				pkgs := testenv.ExtractInstalledPackages(inv)
				if pkgs == nil || (len(pkgs.AllPackageNames()) == 0 && len(pkgs.WUA) == 0 && len(pkgs.QFE) == 0) {
					return fmt.Errorf("no installed packages found in inventory items")
				}

				// 1. Verify package manager types
				gotPackageTypes := pkgs.OSPackageTypes()
				sortOpt := cmpopts.SortSlices(func(a, b string) bool { return a < b })
				if diff := cmp.Diff(tc.wantPackageTypes, gotPackageTypes, sortOpt); diff != "" {
					return fmt.Errorf("installed package types mismatch (-want +got):\n%s", diff)
				}

				// 2. Verify specific expected packages exist in the inventory
				if !pkgs.HasPackages(tc.wantPackages...) {
					return fmt.Errorf("expected packages %v not found in installed packages (total packages: %d)", tc.wantPackages, len(pkgs.AllPackageNames()))
				}

				// 3. Verify Windows QFE / WUA if required
				if tc.requireQFE && !pkgs.HasQFE() {
					return fmt.Errorf("expected at least one QFE package in installed packages, got 0")
				}
				if tc.requireWUA && !pkgs.HasWUA() {
					return fmt.Errorf("expected at least one WUA package in installed packages, got 0")
				}

				return nil
			})
		})
	}
}
