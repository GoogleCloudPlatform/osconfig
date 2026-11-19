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

const ensureLinuxExecTestStateScript = `systemctl stop google-osconfig-agent || true
echo 'if ls $1 >/dev/null; then
exit 100
fi
exit 101' > /validate_shell

echo '#!/bin/sh
touch $1
exit 100' > /enforce_none
chmod +x /enforce_none`

type ospoliciesExecLinuxTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyExecLinuxTestCases returns the list of test cases for Exec policy tests on Linux.
func osPolicyExecLinuxTestCases() []ospoliciesExecLinuxTestCase {
	return []ospoliciesExecLinuxTestCase{
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
	}
}

// buildLinuxExecOSPolicyAssignment constructs the OSPolicyAssignment for testing Exec policies on Linux.
// It validates and enforces execution from GCS, remote URI, local path, inline scripts, and output file capture.
func buildLinuxExecOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "linuxexecresource",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "exec-gcs",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/validate_shell",
												Generation: 1618251880260222,
											},
										},
										Args:        []string{"/path1"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/enforce_none",
												Generation: 1618246663082861,
											},
										},
										Args:        []string{"/path1"},
										Interpreter: "NONE",
									},
								},
							},
							{
								Id: "exec-uri",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/validate_shell",
												Sha256Checksum: "67837bf4b3be1ff84758e22d2eb46db4904dd57eb50f19de3f51a52be1c5b555",
											},
										},
										Args:        []string{"/path2"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/enforce_none",
												Sha256Checksum: "6b9f3936ddc557819281d9bd933aa42be513d31447ca683a9bc26e9f14d6abf1",
											},
										},
										Args:        []string{"/path2"},
										Interpreter: "NONE",
									},
								},
							},
							{
								Id: "exec-local",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											LocalPath: "/validate_shell",
										},
										Args:        []string{"/path3"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											LocalPath: "/enforce_none",
										},
										Args:        []string{"/path3"},
										Interpreter: "NONE",
									},
								},
							},
							{
								Id: "exec-script",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "if ls $1 >/dev/null; then\nexit 100\nfi\nexit 101",
										Args:        []string{"/path4"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "#!/bin/sh\ntouch $1\nexit 100",
										Args:        []string{"/path4"},
										Interpreter: "NONE",
									},
								},
							},
							{
								Id: "exec-output",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "if ls /path5 >/dev/null; then\nexit 100\nfi\nexit 101",
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:         "echo -n \"some output\" > /path5\nexit 100",
										Interpreter:    "SHELL",
										OutputFilePath: "/path5",
									},
								},
							},
							{
								Id: "exec-output-too-large",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "if ls /path6 >/dev/null; then\nexit 100\nfi\nexit 101",
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:         "head -c 1000KB /dev/zero > /path6\nexit 100",
										Interpreter:    "SHELL",
										OutputFilePath: "/path6",
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

// TestOSPolicyExecResourceLinux verifies that the OS Config agent enforces Exec resource policies on Linux:
// executing scripts and binaries from GCS, remote URI, local path, and inline content, and capturing output.
func TestOSPolicyExecResourceLinux(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyExecLinuxTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.StartupScript(tt.image, ensureLinuxExecTestStateScript, "")
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
				return test.CreateOSPolicyAssignment(vm, buildLinuxExecOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})
		})
	}
}
