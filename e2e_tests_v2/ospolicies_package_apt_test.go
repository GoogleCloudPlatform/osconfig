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
	"google.golang.org/api/osconfig/v1"
)

type ospoliciesTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyPackageAptTestCases returns the list of test cases for Apt package policy tests.
func osPolicyPackageAptTestCases() []ospoliciesTestCase {
	return []ospoliciesTestCase{
		// Debian
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "debian-13",
			image:       "projects/debian-cloud/global/images/family/debian-13",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		// Ubuntu
		{
			name:        "ubuntu-2204-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "ubuntu-2404-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
	}
}

// buildAptPackageOSPolicyAssignment constructs the OSPolicyAssignment for testing Apt packages.
// It enforces that "ed" is INSTALLED and "vim" is REMOVED on the target VM.
func buildAptPackageOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
	return &osconfig.OSPolicyAssignment{
		InstanceFilter: &osconfig.OSPolicyAssignmentInstanceFilter{
			InclusionLabels: []*osconfig.OSPolicyAssignmentLabelSet{
				{
					Labels: map[string]string{"name": vmName},
				},
			},
		},
		Rollout: &osconfig.OSPolicyAssignmentRollout{
			DisruptionBudget: &osconfig.FixedOrPercent{Percent: 100},
			MinWaitDuration:  "0s",
		},
		OsPolicies: []*osconfig.OSPolicy{
			{
				Id:   "packageresourceapt",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Apt: &osconfig.OSPolicyResourcePackageResourceAPT{
										Name: "ed",
									},
								},
							},
							{
								Id: "remove-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "REMOVED",
									Apt: &osconfig.OSPolicyResourcePackageResourceAPT{
										Name: "vim",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// TestOSPolicyPackageResourceApt verifies that the OS Config agent enforces APT package policies:
// installing a missing package ("ed") and removing an installed package ("vim").
func TestOSPolicyPackageResourceApt(t *testing.T) {
	tests := osPolicyPackageAptTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyPackageAptStartupScript(tt.image)
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
					scriptKey:                    scriptContent,
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("wait for initial package state in inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"vim"}, []string{"ed"})
			})

			test.Step("create os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildAptPackageOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm)
			})

			test.Step("verify packages via inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"ed"}, []string{"vim"})
			})
		})
	}
}
