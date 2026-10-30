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

type ospoliciesPackageYumTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyPackageYumTestCases returns the list of test cases for Yum package policy tests.
func osPolicyPackageYumTestCases() []ospoliciesPackageYumTestCase {
	return []ospoliciesPackageYumTestCase{
		// RHEL
		{
			name:        "rhel-8",
			image:       "projects/rhel-cloud/global/images/family/rhel-8",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "rhel-9",
			image:       "projects/rhel-cloud/global/images/family/rhel-9",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		// CentOS
		{
			name:        "centos-stream-9",
			image:       "projects/centos-cloud/global/images/family/centos-stream-9",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		// Rocky Linux
		{
			name:        "rocky-linux-8",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "rocky-linux-9",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
	}
}

// buildYumPackageOSPolicyAssignment constructs the OSPolicyAssignment for testing Yum packages.
// It enforces that "ed" is INSTALLED and "nano" is REMOVED on the target VM.
func buildYumPackageOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "packageresourceyum",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Yum: &osconfig.OSPolicyResourcePackageResourceYUM{
										Name: "ed",
									},
								},
							},
							{
								Id: "remove-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "REMOVED",
									Yum: &osconfig.OSPolicyResourcePackageResourceYUM{
										Name: "nano",
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

// TestOSPolicyPackageResourceYum verifies that the OS Config agent enforces YUM package policies:
// installing a missing package ("ed") and removing an installed package ("nano").
func TestOSPolicyPackageResourceYum(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyPackageYumTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyPackageYumStartupScript(tt.image)
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
					scriptKey:                    scriptContent,
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("wait for initial package state in inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"nano"}, []string{"ed"})
			})

			test.Step("create os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildYumPackageOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})

			test.Step("verify packages via inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"ed"}, []string{"nano"})
			})
		})
	}
}
