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

// osPolicyPackageRpmTestCases returns the list of test cases for Rpm package policy tests.
func osPolicyPackageRpmTestCases() []ospoliciesTestCase {
	return []ospoliciesTestCase{
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
		// CentOS Stream
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
		// SUSE / openSUSE
		{
			name:        "sles-15",
			image:       "projects/suse-cloud/global/images/family/sles-15",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "opensuse-leap-15",
			image:       "projects/opensuse-cloud/global/images/family/opensuse-leap",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
	}
}

// buildRpmPackageOSPolicyAssignment constructs the OSPolicyAssignment for testing Rpm packages.
// It installs a GCS-hosted rpm package without dependencies and a remote rpm package with dependencies.
func buildRpmPackageOSPolicyAssignment(policyID, vmName string) *osconfig.OSPolicyAssignment {
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
									Rpm: &osconfig.OSPolicyResourcePackageResourceRPM{
										PullDeps: false,
										Source: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/285280405927e0f9255891926f08a7ff6afe22bfb85a162452000fb9e534585b-osconfig-agent-test-0.1.0-1.el6.x86_64.rpm",
												Generation: 1619119562326151,
											},
										},
									},
								},
							},
							{
								Id: "install-package-pull-deps",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Rpm: &osconfig.OSPolicyResourcePackageResourceRPM{
										PullDeps: true,
										Source: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/gcsfuse-2.0.1-1.x86_64.rpm",
												Sha256Checksum: "92475edeba03e6f7870d30a063d9c19767e6a4f7b0107a2af9191849d4b7a44f",
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

// wantRpmPackageCompliances returns the expected compliance structure for Rpm package policy test.
func wantRpmPackageCompliances(policyID string) []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance {
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

// TestOSPolicyPackageResourceRpm verifies that the OS Config agent installs local and remote rpm packages:
// an rpm package from GCS with PullDeps: false and an rpm package from Remote URI with PullDeps: true.
func TestOSPolicyPackageResourceRpm(t *testing.T) {
	tests := osPolicyPackageRpmTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyPackageRpmStartupScript(tt.image)
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

			policyID := "packageresourcerpm"
			test.Step("create os policy assignment", func(ctx context.Context) error {
				assignment := buildRpmPackageOSPolicyAssignment(policyID, vm.Name)
				_, err := test.CreateOSPolicyAssignment(vm, assignment)
				return err
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				want := wantRpmPackageCompliances(policyID)
				_, err := test.WaitForOSPolicyCompliance(vm, want)
				return err
			})

			test.Step("verify packages installed via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributePkgInstalled)
			})
		})
	}
}
