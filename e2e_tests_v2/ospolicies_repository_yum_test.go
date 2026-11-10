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

type ospoliciesRepoYumTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyRepositoryYumTestCases returns the list of test cases for Yum repository policy tests.
func osPolicyRepositoryYumTestCases() []ospoliciesRepoYumTestCase {
	return []ospoliciesRepoYumTestCase{
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

// buildYumRepositoryOSPolicyAssignment constructs the OSPolicyAssignment for testing Yum repositories.
// It adds the gcsfuse Yum repository and enforces that "gcsfuse" is INSTALLED on the target VM.
func buildYumRepositoryOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "repositoryresourceyum",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-repo",
								Repository: &osconfig.OSPolicyResourceRepositoryResource{
									Yum: &osconfig.OSPolicyResourceRepositoryResourceYumRepository{
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
									Yum: &osconfig.OSPolicyResourcePackageResourceYUM{
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

// TestOSPolicyRepositoryResourceYum verifies that the OS Config agent configures a YUM repository
// and installs a package ("gcsfuse") from it.
func TestOSPolicyRepositoryResourceYum(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyRepositoryYumTestCases()

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
				return test.CreateOSPolicyAssignment(vm, buildYumRepositoryOSPolicyAssignment(vm.Name))
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
