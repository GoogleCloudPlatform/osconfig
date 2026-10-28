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
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	"google.golang.org/api/osconfig/v1"
)

type ospoliciesPackageMsiTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyPackageMsiTestCases returns the list of test cases for Msi package policy tests.
func osPolicyPackageMsiTestCases() []ospoliciesPackageMsiTestCase {
	return []ospoliciesPackageMsiTestCase{
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

// buildMsiPackageOSPolicyAssignment constructs the OSPolicyAssignment for testing MSI packages.
// It installs Chrome Enterprise MSI from GCS on the target VM.
func buildMsiPackageOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "packageresourcemsi",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Msi: &osconfig.OSPolicyResourcePackageResourceMSI{
										Source: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/googlechromestandaloneenterprise64.msi",
												Generation: 1619046639806899,
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

func waitForWindowsApplication(ctx context.Context, test *testenv.Test, vm *gcp.VM, appName string) error {
	return gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("windows application %s on %s", appName, vm.Name), func(ctx context.Context) (string, bool, error) {
		inv, err := test.Suite.Compute.GetInventory(ctx, vm.Project, vm.Zone, vm.Name, "FULL")
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("inventory not yet published (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read inventory: %w", err)
		}
		for _, item := range inv.Items {
			if item.InstalledPackage != nil && item.InstalledPackage.WindowsApplication != nil {
				if strings.Contains(strings.ToLower(item.InstalledPackage.WindowsApplication.DisplayName), strings.ToLower(appName)) {
					return fmt.Sprintf("found application %q", item.InstalledPackage.WindowsApplication.DisplayName), true, nil
				}
			}
		}
		return fmt.Sprintf("waiting for application %q in inventory", appName), false, nil
	})
}

// TestOSPolicyPackageResourceMsi verifies that the OS Config agent installs an MSI package
// (Chrome Enterprise MSI from GCS) on Windows VM instances.
func TestOSPolicyPackageResourceMsi(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyPackageMsiTestCases()

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
				return test.CreateOSPolicyAssignment(vm, buildMsiPackageOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})

			test.Step("verify package installed via inventory", func(ctx context.Context) error {
				return waitForWindowsApplication(ctx, test, vm, "Google Chrome")
			})
		})
	}
}
