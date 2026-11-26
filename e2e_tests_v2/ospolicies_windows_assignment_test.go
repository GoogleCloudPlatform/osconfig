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

const ensureWindowsAssignmentTestStateScript = `$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Stop-Service google_osconfig_agent -Force -ErrorAction SilentlyContinue
}
googet -noconfirm remove certgen`

type ospoliciesAssignmentWindowsTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyAssignmentWindowsTestCases returns the list of test cases for OSPolicyAssignment update and conflict tests on Windows.
func osPolicyAssignmentWindowsTestCases() []ospoliciesAssignmentWindowsTestCase {
	return []ospoliciesAssignmentWindowsTestCase{
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

// buildInitialWindowsOSPolicyAssignment constructs the initial OSPolicyAssignment for a Windows VM:
// enforcing C:\config.txt with content "v1" as PRESENT and GooGet package "certgen" as INSTALLED.
func buildInitialWindowsOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "assignment-update-policy",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "file-config",
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "PRESENT",
									Path:    `C:\config.txt`,
									Content: "v1",
								},
							},
							{
								Id: "pkg-certgen",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Googet: &osconfig.OSPolicyResourcePackageResourceGooGet{
										Name: "certgen",
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

// buildUpdatedWindowsOSPolicyAssignment constructs the updated OSPolicyAssignment for a Windows VM:
// 1. "assignment-update-policy" updates C:\config.txt to "v2" (CONTENTS_MATCH), verifies its content is "v2",
//    and changes GooGet package "certgen" to REMOVED.
// 2. "resource-conflict-policy" defines two conflicting FileResource entries targeting C:\conflict.txt.
func buildUpdatedWindowsOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "assignment-update-policy",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "file-config",
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "CONTENTS_MATCH",
									Path:    `C:\config.txt`,
									Content: "v2",
								},
							},
							{
								Id: "verify-config-v2",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      `if ((Get-Content C:\config.txt -Raw -ErrorAction SilentlyContinue) -eq "v2") { exit 100 }; exit 101`,
										Interpreter: "POWERSHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "exit 100",
										Interpreter: "POWERSHELL",
									},
								},
							},
							{
								Id: "pkg-certgen",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "REMOVED",
									Googet: &osconfig.OSPolicyResourcePackageResourceGooGet{
										Name: "certgen",
									},
								},
							},
						},
					},
				},
			},
			{
				Id:   "resource-conflict-policy",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "file-conflict-1",
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "CONTENTS_MATCH",
									Path:    `C:\conflict.txt`,
									Content: "v1",
								},
							},
							{
								Id: "file-conflict-2",
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "CONTENTS_MATCH",
									Path:    `C:\conflict.txt`,
									Content: "v2",
								},
							},
						},
					},
				},
			},
		},
	}
}

func isUpdatedWindowsAssignmentReportValid(rep *osconfig.OSPolicyAssignmentReport) (string, bool) {
	const wantPolicies = 2
	if rep == nil || len(rep.OsPolicyCompliances) != wantPolicies {
		got := 0
		if rep != nil {
			got = len(rep.OsPolicyCompliances)
		}
		return fmt.Sprintf("waiting for %d os policy compliances in updated report, got %d", wantPolicies, got), false
	}

	byID := make(map[string]*osconfig.OSPolicyAssignmentReportOSPolicyCompliance, len(rep.OsPolicyCompliances))
	for _, comp := range rep.OsPolicyCompliances {
		byID[comp.OsPolicyId] = comp
	}

	updatePolicy, ok := byID["assignment-update-policy"]
	if !ok {
		return "missing assignment-update-policy in report", false
	}
	if updatePolicy.ComplianceState != "COMPLIANT" {
		return fmt.Sprintf("os policy %q in state %s, want COMPLIANT", updatePolicy.OsPolicyId, updatePolicy.ComplianceState), false
	}
	if len(updatePolicy.OsPolicyResourceCompliances) != 3 {
		return fmt.Sprintf("waiting for 3 resource compliances in assignment-update-policy, got %d", len(updatePolicy.OsPolicyResourceCompliances)), false
	}
	for _, res := range updatePolicy.OsPolicyResourceCompliances {
		if res.ComplianceState != "COMPLIANT" {
			return fmt.Sprintf("resource %q in state %s, want COMPLIANT", res.OsPolicyResourceId, res.ComplianceState), false
		}
	}

	conflictPolicy, ok := byID["resource-conflict-policy"]
	if !ok {
		return "missing resource-conflict-policy in report", false
	}
	if conflictPolicy.ComplianceState != "NON_COMPLIANT" {
		return fmt.Sprintf("os policy %q in state %s, want NON_COMPLIANT", conflictPolicy.OsPolicyId, conflictPolicy.ComplianceState), false
	}
	if len(conflictPolicy.OsPolicyResourceCompliances) != 2 {
		return fmt.Sprintf("waiting for 2 resource compliances in resource-conflict-policy, got %d", len(conflictPolicy.OsPolicyResourceCompliances)), false
	}
	var hasNonCompliantResource bool
	for _, res := range conflictPolicy.OsPolicyResourceCompliances {
		if res.ComplianceState == "NON_COMPLIANT" {
			hasNonCompliantResource = true
			break
		}
	}
	if !hasNonCompliantResource {
		return "resource-conflict-policy has no NON_COMPLIANT resource", false
	}

	return "updated policy COMPLIANT and conflicting policy NON_COMPLIANT", true
}

func waitForUpdatedWindowsAssignmentReport(ctx context.Context, test *testenv.Test, vm *gcp.VM) error {
	assignmentID := vm.Name
	return gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("updated os policy report for %s on %s", assignmentID, vm.Name), func(ctx context.Context) (string, bool, error) {
		rep, err := test.Suite.Compute.GetOSPolicyAssignmentReport(ctx, vm.Project, vm.Zone, vm.Name, assignmentID)
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("report not yet available (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read os policy assignment report: %w", err)
		}

		msg, ok := isUpdatedWindowsAssignmentReportValid(rep)
		return msg, ok, nil
	})
}

// TestOSPolicyAssignmentUpdateWindows verifies updating an existing OSPolicyAssignment on a Windows VM
// (updating file content via CONTENTS_MATCH and removing a previously installed GooGet package)
// and detecting conflicting resources targeting the same file path in a policy.
func TestOSPolicyAssignmentUpdateWindows(t *testing.T) {
	const wantInitialCompliances = 1
	tests := osPolicyAssignmentWindowsTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.StartupScript(tt.image, ensureWindowsAssignmentTestStateScript, "")
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
					scriptKey:                    scriptContent,
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("wait for initial package state in inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, nil, []string{"certgen"})
			})

			test.Step("create initial os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildInitialWindowsOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for initial os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantInitialCompliances)
			})

			test.Step("verify package installed via inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, []string{"certgen"}, nil)
			})

			test.Step("update os policy assignment", func(ctx context.Context) error {
				return test.UpdateOSPolicyAssignment(vm, buildUpdatedWindowsOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for updated os policy compliance and conflict report", func(ctx context.Context) error {
				return waitForUpdatedWindowsAssignmentReport(ctx, test, vm)
			})

			test.Step("verify package removed via inventory", func(ctx context.Context) error {
				return test.WaitForPackages(vm, nil, []string{"certgen"})
			})
		})
	}
}
