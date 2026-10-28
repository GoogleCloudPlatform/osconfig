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

type ospoliciesFileLinuxTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyFileLinuxTestCases returns the list of test cases for File policy tests on Linux.
func osPolicyFileLinuxTestCases() []ospoliciesFileLinuxTestCase {
	return []ospoliciesFileLinuxTestCase{
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
		// SUSE
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

// buildLinuxFileOSPolicyAssignment constructs the OSPolicyAssignment for testing File resources on Linux.
// It creates two policies:
// 1. "files-absent": ensures /enforce_absent is ABSENT.
// 2. "files-present": ensures files are PRESENT from content, GCS, remote URI, and local path.
func buildLinuxFileOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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

// TestOSPolicyFileResourceLinux verifies that the OS Config agent enforces File resource policies on Linux:
// removing an existing file and creating files from content, GCS bucket, remote URI, and local path.
func TestOSPolicyFileResourceLinux(t *testing.T) {
	const wantCompliances = 2
	tests := osPolicyFileLinuxTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.OSPolicyFileLinuxStartupScript(tt.image)
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
					scriptKey:                    scriptContent,
				}
				var err error
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("create os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildLinuxFileOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})
		})
	}
}
