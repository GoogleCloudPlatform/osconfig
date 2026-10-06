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

// osPolicyPackageDebTestCases returns the list of test cases for Deb package policy tests.
func osPolicyPackageDebTestCases() []ospoliciesTestCase {
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

// buildDebPackageOSPolicyAssignment constructs the OSPolicyAssignment for testing Deb packages.
// It installs a GCS-hosted deb package without dependencies and a remote deb package with dependencies.
func buildDebPackageOSPolicyAssignment(policyID, vmName string) *osconfig.OSPolicyAssignment {
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
									Deb: &osconfig.OSPolicyResourcePackageResourceDeb{
										PullDeps: false,
										Source: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/osconfig-agent-test_7.0_all_f88296edfb1ebcce2e99fb9381c456138c5db86552df6530d022841bf9ac30bf.deb",
												Generation: 1619046473027315,
											},
										},
									},
								},
							},
							{
								Id: "install-package-pull-deps",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Deb: &osconfig.OSPolicyResourcePackageResourceDeb{
										PullDeps: true,
										Source: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/google-chrome-stable_current_amd64.deb",
												Sha256Checksum: "3ec1cadbb55cf66cc51f0421eace324a88836ee2d982b945b8f67a3f131b0924",
											},
										},
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

// wantDebPackageCompliances returns the expected compliance structure for Deb package policy test.
func wantDebPackageCompliances(policyID string) []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance {
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
					OsPolicyResourceId: "install-package-pull-deps",
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

// TestOSPolicyPackageResourceDeb verifies that the OS Config agent installs local and remote deb packages:
// a deb package from GCS with PullDeps: false and a deb package from Remote URI with PullDeps: true.
func TestOSPolicyPackageResourceDeb(t *testing.T) {
	tests := osPolicyPackageDebTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyPackageDebStartupScript(tt.image)
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

			policyID := "packageresourcedeb"
			test.Step("create os policy assignment", func(ctx context.Context) error {
				assignment := buildDebPackageOSPolicyAssignment(policyID, vm.Name)
				_, err := test.CreateOSPolicyAssignment(vm, assignment)
				return err
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				want := wantDebPackageCompliances(policyID)
				_, err := test.WaitForOSPolicyCompliance(vm, want)
				return err
			})

			test.Step("verify packages installed via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributePkgInstalled)
			})
		})
	}
}
