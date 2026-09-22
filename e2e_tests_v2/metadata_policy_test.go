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
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
)

const defaultRecipeName = "testrecipe"

type metadataPolicyTestCase struct {
	name        string
	image       string
	pkgManager  string
	machineType string
	timeout     time.Duration
}

// metadataPolicyTestCases returns the list of test cases for metadata policy tests.
func metadataPolicyTestCases() []metadataPolicyTestCase {
	return []metadataPolicyTestCase{
		// Debian
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
			pkgManager:  "apt",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "debian-13",
			image:       "projects/debian-cloud/global/images/family/debian-13",
			pkgManager:  "apt",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		// Ubuntu
		{
			name:        "ubuntu-2204-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			pkgManager:  "apt",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "ubuntu-2404-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			pkgManager:  "apt",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		// Enterprise Linux (RHEL, CentOS Stream, Rocky Linux)
		{
			name:        "rhel-8",
			image:       "projects/rhel-cloud/global/images/family/rhel-8",
			pkgManager:  "yum",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "rhel-9",
			image:       "projects/rhel-cloud/global/images/family/rhel-9",
			pkgManager:  "yum",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "centos-stream-9",
			image:       "projects/centos-cloud/global/images/family/centos-stream-9",
			pkgManager:  "yum",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "rocky-linux-8",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
			pkgManager:  "yum",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "rocky-linux-9",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
			pkgManager:  "yum",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		// SUSE / openSUSE
		{
			name:        "sles-15",
			image:       "projects/suse-cloud/global/images/family/sles-15",
			pkgManager:  "zypper",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "opensuse-leap-15",
			image:       "projects/opensuse-cloud/global/images/family/opensuse-leap",
			pkgManager:  "zypper",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		// Windows Server
		{
			name:        "windows-2016",
			image:       "projects/windows-cloud/global/images/family/windows-2016",
			pkgManager:  "googet",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			pkgManager:  "googet",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			pkgManager:  "googet",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		// Container-Optimized OS (COS)
		{
			name:        "cos-stable",
			image:       "projects/cos-cloud/global/images/family/cos-stable",
			pkgManager:  "cos",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "cos-beta",
			image:       "projects/cos-cloud/global/images/family/cos-beta",
			pkgManager:  "cos",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
		{
			name:        "cos-dev",
			image:       "projects/cos-cloud/global/images/family/cos-dev",
			pkgManager:  "cos",
			machineType: "e2-standard-2",
			timeout:     15 * time.Minute,
		},
	}
}

// createRecipeTestVM creates a VM instance configured for recipe testing.
func createRecipeTestVM(test *testenv.Test, tc metadataPolicyTestCase, scriptKey, scriptContent string) (*gcp.VM, error) {
	metadata := map[string]string{
		// Disable unused subsystems so only guestpolicies (SoftwareRecipes) runs.
		"osconfig-disabled-features": "tasks,osinventory",
		// osconfig-poll-interval controls the main service loop ticker for both
		// guestpolicies and osinventory (default: 10m). Set to 1m for faster policy retries.
		"osconfig-poll-interval": "1",
		scriptKey:                scriptContent,
	}
	return test.CreateVM(tc.image, tc.machineType, metadata)
}

// TestMetadataPolicy verifies that the OS Config agent applies a configuration defined in instance metadata.
func TestMetadataPolicy(t *testing.T) {
	tests := metadataPolicyTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.RecipeInstallStartupScript(tt.image, defaultRecipeName)
				var err error
				vm, err = createRecipeTestVM(test, tt, scriptKey, scriptContent)
				return err
			})

			test.Step("wait for agent install", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeInstallDone)
			})

			test.Step("apply metadata policy and restart agent", func(ctx context.Context) error {
				mdJSON, err := testenv.BuildMetadataPolicyJSON(defaultRecipeName)
				if err != nil {
					return err
				}
				return test.AddMetadata(vm, map[string]string{
					"gce-software-declaration":  mdJSON,
					gcp.MetadataKeyRestartAgent: "true",
				})
			})

			test.Step("verify recipe installation", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeRecipeInstalled)
			})
		})
	}
}
