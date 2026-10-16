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

type ospoliciesRepoAptTestCase struct {
	name        string
	image       string
	distro      string
	machineType string
	timeout     time.Duration
}

// osPolicyRepositoryAptTestCases returns the list of test cases for Apt repository policy tests.
func osPolicyRepositoryAptTestCases() []ospoliciesRepoAptTestCase {
	return []ospoliciesRepoAptTestCase{
		// Debian
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
			distro:      "gcsfuse-bookworm",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "debian-13",
			image:       "projects/debian-cloud/global/images/family/debian-13",
			distro:      "gcsfuse-trixie",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		// Ubuntu
		{
			name:        "ubuntu-2204-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			distro:      "gcsfuse-jammy",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
		{
			name:        "ubuntu-2404-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			distro:      "gcsfuse-noble",
			machineType: "e2-medium",
			timeout:     15 * time.Minute,
		},
	}
}

// buildAptRepositoryOSPolicyAssignment constructs the OSPolicyAssignment for testing Apt repositories.
// It adds the gcsfuse Apt repository and enforces that "gcsfuse" is INSTALLED on the target VM.
func buildAptRepositoryOSPolicyAssignment(policyID, vmName, distro string) *osconfig.OSPolicyAssignment {
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
								Id: "install-repo",
								Repository: &osconfig.OSPolicyResourceRepositoryResource{
									Apt: &osconfig.OSPolicyResourceRepositoryResourceAptRepository{
										ArchiveType:  "DEB",
										Uri:          "http://packages.cloud.google.com/apt",
										Distribution: distro,
										Components:   []string{"main"},
										GpgKey:       "https://packages.cloud.google.com/apt/doc/apt-key.gpg",
									},
								},
							},
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Apt: &osconfig.OSPolicyResourcePackageResourceAPT{
										Name: "gcsfuse",
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

// wantAptRepositoryCompliances returns the expected compliance structure for Apt repository policy test.
func wantAptRepositoryCompliances(policyID string) []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance {
	return []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance{
		{
			OsPolicyId:      policyID,
			ComplianceState: "COMPLIANT",
			OsPolicyResourceCompliances: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "install-repo",
					ComplianceState:    "COMPLIANT",
					ConfigSteps: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceComplianceOSPolicyResourceConfigStep{
						{Type: "VALIDATION"},
						{Type: "DESIRED_STATE_CHECK"},
						{Type: "DESIRED_STATE_ENFORCEMENT"},
						{Type: "DESIRED_STATE_CHECK_POST_ENFORCEMENT"},
					},
				},
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
			},
		},
	}
}

// TestOSPolicyRepositoryResourceApt verifies that the OS Config agent configures an APT repository
// and installs a package ("gcsfuse") from it.
func TestOSPolicyRepositoryResourceApt(t *testing.T) {
	tests := osPolicyRepositoryAptTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyRepositoryAptStartupScript(tt.image)
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

			policyID := "repositoryresourceapt"
			test.Step("create os policy assignment", func(ctx context.Context) error {
				assignment := buildAptRepositoryOSPolicyAssignment(policyID, vm.Name, tt.distro)
				_, err := test.CreateOSPolicyAssignment(vm, assignment)
				return err
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				want := wantAptRepositoryCompliances(policyID)
				_, err := test.WaitForOSPolicyCompliance(vm, want)
				return err
			})

			test.Step("verify package installed via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributePkgInstalled)
			})
		})
	}
}
