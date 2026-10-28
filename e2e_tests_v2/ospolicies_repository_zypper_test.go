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

type ospoliciesRepoZypperTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyRepositoryZypperTestCases returns the list of test cases for Zypper repository policy tests.
func osPolicyRepositoryZypperTestCases() []ospoliciesRepoZypperTestCase {
	return []ospoliciesRepoZypperTestCase{
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

// buildZypperRepositoryOSPolicyAssignment constructs the OSPolicyAssignment for testing Zypper repositories.
// It adds the gcsfuse Zypper repository and enforces that "gcsfuse" is INSTALLED on the target VM.
func buildZypperRepositoryOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "repositoryresourcezypper",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-repo",
								Repository: &osconfig.OSPolicyResourceRepositoryResource{
									Zypper: &osconfig.OSPolicyResourceRepositoryResourceZypperRepository{
										Id:          "gcsfuse",
										DisplayName: "gcsfuse",
										BaseUrl:     "https://packages.cloud.google.com/yum/repos/gcsfuse-el7-x86_64",
										GpgKeys: []string{
											"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
											"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
										},
									},
								},
							},
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Zypper: &osconfig.OSPolicyResourcePackageResourceZypper{
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

// TestOSPolicyRepositoryResourceZypper verifies that the OS Config agent configures a Zypper repository
// and installs a package ("gcsfuse") from it.
func TestOSPolicyRepositoryResourceZypper(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyRepositoryZypperTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("create os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildZypperRepositoryOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})

			test.Step("verify package installed via inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"gcsfuse"}, nil)
			})
		})
	}
}
