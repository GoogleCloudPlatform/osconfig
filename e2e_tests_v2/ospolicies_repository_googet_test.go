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

type ospoliciesRepoGooGetTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyRepositoryGooGetTestCases returns the list of test cases for GooGet repository policy tests.
func osPolicyRepositoryGooGetTestCases() []ospoliciesRepoGooGetTestCase {
	return []ospoliciesRepoGooGetTestCase{
		// Windows Server
		{
			name:        "windows-2016",
			image:       "projects/windows-cloud/global/images/family/windows-2016",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
	}
}

// buildGooGetRepositoryOSPolicyAssignment constructs the OSPolicyAssignment for testing GooGet repositories.
// It adds the Google OSConfig Agent Test Repository and enforces that "osconfig-agent-test" is INSTALLED on the target VM.
func buildGooGetRepositoryOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "repositoryresourcegoo",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-repo",
								Repository: &osconfig.OSPolicyResourceRepositoryResource{
									Goo: &osconfig.OSPolicyResourceRepositoryResourceGooRepository{
										Name: "Google OSConfig Agent Test Repository",
										Url:  "https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository",
									},
								},
							},
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Googet: &osconfig.OSPolicyResourcePackageResourceGooGet{
										Name: "osconfig-agent-test",
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

// TestOSPolicyRepositoryResourceGooGet verifies that the OS Config agent configures a GooGet repository
// and installs a package ("osconfig-agent-test") from it on Windows instances.
func TestOSPolicyRepositoryResourceGooGet(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyRepositoryGooGetTestCases()

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
				return test.CreateOSPolicyAssignment(vm, buildGooGetRepositoryOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})

			test.Step("verify package installed via inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"osconfig-agent-test"}, nil)
			})
		})
	}
}
