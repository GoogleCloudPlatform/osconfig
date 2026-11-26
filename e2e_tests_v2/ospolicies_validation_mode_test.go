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

type ospoliciesValidationModeTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyValidationModeTestCases returns the list of test cases for OSPolicy VALIDATION mode tests.
func osPolicyValidationModeTestCases() []ospoliciesValidationModeTestCase {
	return []ospoliciesValidationModeTestCase{
		// Debian
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		{
			name:        "debian-13",
			image:       "projects/debian-cloud/global/images/family/debian-13",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		// Ubuntu
		{
			name:        "ubuntu-2204-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		{
			name:        "ubuntu-2404-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		// RHEL
		{
			name:        "rhel-8",
			image:       "projects/rhel-cloud/global/images/family/rhel-8",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		{
			name:        "rhel-9",
			image:       "projects/rhel-cloud/global/images/family/rhel-9",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		// CentOS Stream
		{
			name:        "centos-stream-9",
			image:       "projects/centos-cloud/global/images/family/centos-stream-9",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		// Rocky Linux
		{
			name:        "rocky-linux-8",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		{
			name:        "rocky-linux-9",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		// SUSE / openSUSE
		{
			name:        "sles-15",
			image:       "projects/suse-cloud/global/images/family/sles-15",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
		{
			name:        "opensuse-leap-15",
			image:       "projects/opensuse-cloud/global/images/family/opensuse-leap",
			machineType: "e2-medium",
			timeout:     25 * time.Minute,
		},
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

// buildValidationModeOSPolicyAssignment constructs an OSPolicyAssignment in VALIDATION mode
// checking for a non-existent file (/file-not-exists).
func buildValidationModeOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "file-present",
				Mode: "VALIDATION",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "file-present",
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "PRESENT",
									Path:    "/file-not-exists",
									Content: "something",
								},
							},
						},
					},
				},
			},
		},
	}
}

func isValidationModeReportValid(rep *osconfig.OSPolicyAssignmentReport) (string, bool) {
	if rep == nil || len(rep.OsPolicyCompliances) != 1 {
		got := 0
		if rep != nil {
			got = len(rep.OsPolicyCompliances)
		}
		return fmt.Sprintf("waiting for 1 os policy compliance, got %d", got), false
	}

	comp := rep.OsPolicyCompliances[0]
	if comp.OsPolicyId != "file-present" {
		return fmt.Sprintf("unexpected os policy id %q, want %q", comp.OsPolicyId, "file-present"), false
	}
	if comp.ComplianceState != "NON_COMPLIANT" {
		return fmt.Sprintf("os policy %q in state %s, want NON_COMPLIANT", comp.OsPolicyId, comp.ComplianceState), false
	}
	if len(comp.OsPolicyResourceCompliances) != 1 {
		return fmt.Sprintf("waiting for 1 resource compliance, got %d", len(comp.OsPolicyResourceCompliances)), false
	}

	res := comp.OsPolicyResourceCompliances[0]
	if res.OsPolicyResourceId != "file-present" {
		return fmt.Sprintf("unexpected resource id %q, want %q", res.OsPolicyResourceId, "file-present"), false
	}
	if res.ComplianceState != "NON_COMPLIANT" {
		return fmt.Sprintf("resource %q in state %s, want NON_COMPLIANT", res.OsPolicyResourceId, res.ComplianceState), false
	}
	if len(res.ConfigSteps) != 2 {
		return fmt.Sprintf("expected 2 config steps (VALIDATION, DESIRED_STATE_CHECK), got %d", len(res.ConfigSteps)), false
	}
	if res.ConfigSteps[0].Type != "VALIDATION" || res.ConfigSteps[1].Type != "DESIRED_STATE_CHECK" {
		return fmt.Sprintf("unexpected config steps: [%s, %s]", res.ConfigSteps[0].Type, res.ConfigSteps[1].Type), false
	}

	return "validation mode NON_COMPLIANT report verified", true
}

func waitForValidationModeReport(ctx context.Context, test *testenv.Test, vm *gcp.VM) error {
	assignmentID := vm.Name
	return gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("validation mode report for %s on %s", assignmentID, vm.Name), func(ctx context.Context) (string, bool, error) {
		rep, err := test.Suite.Compute.GetOSPolicyAssignmentReport(ctx, vm.Project, vm.Zone, vm.Name, assignmentID)
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("report not yet available (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read os policy assignment report: %w", err)
		}

		msg, ok := isValidationModeReportValid(rep)
		return msg, ok, nil
	})
}

// TestOSPolicyValidationMode verifies that an OSPolicy in VALIDATION mode checks resource state
// without enforcing changes and reports NON_COMPLIANT when the desired state is not met.
func TestOSPolicyValidationMode(t *testing.T) {
	tests := osPolicyValidationModeTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var preAgentScript string
				if !strings.Contains(tt.image, "windows") {
					preAgentScript = "systemctl stop google-osconfig-agent || true"
				}
				scriptKey, scriptContent := gcp.StartupScript(tt.image, preAgentScript, "")
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
					scriptKey:                    scriptContent,
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("wait for agent inventory", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("create os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildValidationModeOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy validation report", func(ctx context.Context) error {
				return waitForValidationModeReport(ctx, test, vm)
			})
		})
	}
}
