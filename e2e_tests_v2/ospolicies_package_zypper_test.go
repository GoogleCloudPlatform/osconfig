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
	"google.golang.org/api/osconfig/v1"
)

// osPolicyPackageZypperTestCases returns the list of test cases for Zypper package policy tests.
func osPolicyPackageZypperTestCases() []ospoliciesTestCase {
	return []ospoliciesTestCase{
		// SLES
		{
			name:        "sles-15",
			image:       "projects/suse-cloud/global/images/family/sles-15",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		// openSUSE Leap
		{
			name:        "opensuse-leap-15",
			image:       "projects/opensuse-cloud/global/images/family/opensuse-leap",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
	}
}

// buildZypperPackageOSPolicyAssignment constructs the OSPolicyAssignment for testing Zypper packages.
// It enforces that "ed" is INSTALLED and "vim" is REMOVED on the target VM.
func buildZypperPackageOSPolicyAssignment(policyID, vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   policyID,
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Zypper: &osconfig.OSPolicyResourcePackageResourceZypper{
										Name: "ed",
									},
								},
							},
							{
								Id: "remove-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "REMOVED",
									Zypper: &osconfig.OSPolicyResourcePackageResourceZypper{
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

// wantZypperPackageCompliances returns the expected compliance structure for Zypper package policy test.
func wantZypperPackageCompliances(policyID string) []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance {
	return []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance{
		{
			OsPolicyId:      policyID,
			ComplianceState: "COMPLIANT",
			OsPolicyResourceCompliances: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "install-package",
					ComplianceState:    "COMPLIANT",
					ConfigSteps: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceComplianceOSPolicyResourceConfigStep{
						{Type: "VALIDATION"},
						{Type: "DESIRED_STATE_CHECK"},
						{Type: "DESIRED_STATE_ENFORCEMENT"},
						{Type: "DESIRED_STATE_CHECK_POST_ENFORCEMENT"},
					},
				},
				{
					OsPolicyResourceId: "remove-package",
					ComplianceState:    "COMPLIANT",
					ConfigSteps: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceComplianceOSPolicyResourceConfigStep{
						{Type: "VALIDATION"},
						{Type: "DESIRED_STATE_CHECK"},
						{Type: "DESIRED_STATE_ENFORCEMENT"},
						{Type: "DESIRED_STATE_CHECK_POST_ENFORCEMENT"},
					},
				},
			},
		},
	}
}

// TestOSPolicyPackageResourceZypper verifies that the OS Config agent enforces Zypper package policies:
// installing a missing package ("ed") and removing an installed package ("vim").
func TestOSPolicyPackageResourceZypper(t *testing.T) {
	tests := osPolicyPackageZypperTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyPackageZypperStartupScript(tt.image)
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies,osinventory",
					"osconfig-poll-interval":     "1",
					scriptKey:                    scriptContent,
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				if err != nil {
					return fmt.Errorf("failed to create VM: %w", err)
				}
				return nil
			})

			test.Step("wait for agent install and prerequisites", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeInstallDone)
			})

			policyID := "packageresourcezypper"
			test.Step("create os policy assignment", func(ctx context.Context) error {
				assignment := buildZypperPackageOSPolicyAssignment(policyID, vm.Name)
				_, err := test.CreateOSPolicyAssignment(vm, assignment)
				return err
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				want := wantZypperPackageCompliances(policyID)
				_, err := test.WaitForOSPolicyCompliance(vm, want)
				return err
			})

			test.Step("verify package installed via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributePkgInstalled)
			})

			test.Step("verify package removed via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributePkgNotInstalled)
			})
		})
	}
}
