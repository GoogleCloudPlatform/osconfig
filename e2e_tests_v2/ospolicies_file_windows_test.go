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

type ospoliciesFileWindowsTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyFileWindowsTestCases returns the list of test cases for File policy tests on Windows.
func osPolicyFileWindowsTestCases() []ospoliciesFileWindowsTestCase {
	return []ospoliciesFileWindowsTestCase{
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

// buildWindowsFileOSPolicyAssignment constructs the OSPolicyAssignment for testing File policies on Windows.
// It removes /enforce_absent and creates files from content, GCS bucket, remote URI, and local path.
func buildWindowsFileOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "files-absent",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "file-absent",
								File: &osconfig.OSPolicyResourceFileResource{
									State: "ABSENT",
									Path:  "/enforce_absent",
								},
							},
						},
					},
				},
			},
			{
				Id:   "files-present",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "file-present-from-content",
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "PRESENT",
									Path:    "/from_content",
									Content: "something",
								},
							},
							{
								Id: "file-present-from-gcs",
								File: &osconfig.OSPolicyResourceFileResource{
									State: "PRESENT",
									Path:  "/from_gcs",
									File: &osconfig.OSPolicyResourceFile{
										Gcs: &osconfig.OSPolicyResourceFileGcs{
											Bucket:     "osconfig-agent-end2end-test-resources",
											Object:     "OSPolicies/test_file",
											Generation: 1617666133905437,
										},
									},
								},
							},
							{
								Id: "file-present-from-uri",
								File: &osconfig.OSPolicyResourceFileResource{
									State: "PRESENT",
									Path:  "/from_uri",
									File: &osconfig.OSPolicyResourceFile{
										Remote: &osconfig.OSPolicyResourceFileRemote{
											Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/test_file",
											Sha256Checksum: "e0ef7229e64c61596d8be928397e19fcc542ac920c4132106fb1ec2295dd73d1",
										},
									},
								},
							},
							{
								Id: "file-present-from-local",
								File: &osconfig.OSPolicyResourceFileResource{
									State: "PRESENT",
									Path:  "/from_local",
									File: &osconfig.OSPolicyResourceFile{
										LocalPath: "/from_content",
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

// wantWindowsFileCompliances returns the expected compliance reports for File resource policies on Windows.
func wantWindowsFileCompliances() []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance {
	configSteps := []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceComplianceOSPolicyResourceConfigStep{
		{Type: "VALIDATION"},
		{Type: "DESIRED_STATE_CHECK"},
		{Type: "DESIRED_STATE_ENFORCEMENT"},
		{Type: "DESIRED_STATE_CHECK_POST_ENFORCEMENT"},
	}

	return []*osconfig.OSPolicyAssignmentReportOSPolicyCompliance{
		{
			OsPolicyId:      "files-absent",
			ComplianceState: "COMPLIANT",
			OsPolicyResourceCompliances: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "file-absent",
					ComplianceState:    "COMPLIANT",
					ConfigSteps:        configSteps,
				},
			},
		},
		{
			OsPolicyId:      "files-present",
			ComplianceState: "COMPLIANT",
			OsPolicyResourceCompliances: []*osconfig.OSPolicyAssignmentReportOSPolicyComplianceOSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "file-present-from-content",
					ComplianceState:    "COMPLIANT",
					ConfigSteps:        configSteps,
				},
				{
					OsPolicyResourceId: "file-present-from-gcs",
					ComplianceState:    "COMPLIANT",
					ConfigSteps:        configSteps,
				},
				{
					OsPolicyResourceId: "file-present-from-uri",
					ComplianceState:    "COMPLIANT",
					ConfigSteps:        configSteps,
				},
				{
					OsPolicyResourceId: "file-present-from-local",
					ComplianceState:    "COMPLIANT",
					ConfigSteps:        configSteps,
				},
			},
		},
	}
}

// TestOSPolicyFileResourceWindows verifies that the OS Config agent enforces File resource policies on Windows:
// removing an existing file and creating files from content, GCS bucket, remote URI, and local path.
func TestOSPolicyFileResourceWindows(t *testing.T) {
	tests := osPolicyFileWindowsTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyFileWindowsStartupScript(tt.image)
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

			test.Step("create os policy assignment", func(ctx context.Context) error {
				assignment := buildWindowsFileOSPolicyAssignment(vm.Name)
				_, err := test.CreateOSPolicyAssignment(vm, assignment)
				return err
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				want := wantWindowsFileCompliances()
				_, err := test.WaitForOSPolicyCompliance(vm, want)
				return err
			})

			test.Step("verify absent file removed via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeFileDNE)
			})

			test.Step("verify present files created via guest attribute", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeFileExists)
			})
		})
	}
}
