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

const ensureWindowsExecTestStateScript = `$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Stop-Service google_osconfig_agent -Force -ErrorAction SilentlyContinue
}

@'
if exist %1 exit 100
exit 101
'@ | Out-File -Encoding ASCII /validate.cmd
@'
echo "" > %1
exit 100
'@ | Out-File -Encoding ASCII /enforce.cmd
'if (Test-Path $Args[0]) {exit 100}; exit 101' > /validate.ps1
'New-Item -ItemType File -Path $Args[0]; exit 100' > /enforce.ps1`

type ospoliciesExecWindowsTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

// osPolicyExecWindowsTestCases returns the list of test cases for Exec policy tests on Windows.
func osPolicyExecWindowsTestCases() []ospoliciesExecWindowsTestCase {
	return []ospoliciesExecWindowsTestCase{
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

// buildWindowsExecOSPolicyAssignment constructs the OSPolicyAssignment for testing Exec policies on Windows.
// It validates and enforces execution of cmd and PowerShell scripts from GCS, remote URI, local path, inline scripts, and output file capture.
func buildWindowsExecOSPolicyAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   "windowsexecresource",
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "exec-gcs-cmd",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/validate.cmd",
												Generation: 1618340275278922,
											},
										},
										Args:        []string{"/path1"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/enforce.cmd",
												Generation: 1619475023716779,
											},
										},
										Args:        []string{"/path1"},
										Interpreter: "NONE",
									},
								},
							},
							{
								Id: "exec-uri-cmd",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/validate.cmd",
												Sha256Checksum: "1635e97b142fa9dd21bb023093ede409d242f52a535ad779bb80539db95c8f77",
											},
										},
										Args:        []string{"/path2"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/enforce.cmd",
												Sha256Checksum: "08307e3de5baf1c7051c6901e798aaaf0c5f06350cc4518fbcd431dc6e6af003",
											},
										},
										Args:        []string{"/path2"},
										Interpreter: "NONE",
									},
								},
							},
							{
								Id: "exec-local-cmd",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											LocalPath: "/validate.cmd",
										},
										Args:        []string{"/path3"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											LocalPath: "/enforce.cmd",
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
										Script:      "if exist %1 exit 100\nexit 101",
										Args:        []string{"/path4"},
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "New-Item -ItemType File -Path $Args[0]; exit 100",
										Args:        []string{"/path4"},
										Interpreter: "POWERSHELL",
									},
								},
							},
							{
								Id: "exec-gcs-uri-ps1",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Gcs: &osconfig.OSPolicyResourceFileGcs{
												Bucket:     "osconfig-agent-end2end-test-resources",
												Object:     "OSPolicies/validate.ps1",
												Generation: 1617995966532645,
											},
										},
										Args:        []string{"/path5"},
										Interpreter: "POWERSHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											Remote: &osconfig.OSPolicyResourceFileRemote{
												Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/enforce.ps1",
												Sha256Checksum: "a5737e35f8a3a04785e4e0b9ffa90c5209db320c0ef9692672f5fb0b1dfe99d2",
											},
										},
										Args:        []string{"/path5"},
										Interpreter: "POWERSHELL",
									},
								},
							},
							{
								Id: "exec-local-powershell",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											LocalPath: "/validate.ps1",
										},
										Args:        []string{"/path6"},
										Interpreter: "POWERSHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										File: &osconfig.OSPolicyResourceFile{
											LocalPath: "/enforce.ps1",
										},
										Args:        []string{"/path6"},
										Interpreter: "POWERSHELL",
									},
								},
							},
							{
								Id: "exec-output",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "if exist /path7 exit 100\nexit 101",
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:         "echo|set /p=\"some output\" > /path7\nexit 100",
										Interpreter:    "SHELL",
										OutputFilePath: "/path7",
									},
								},
							},
							{
								Id: "exec-output-too-large",
								Exec: &osconfig.OSPolicyResourceExecResource{
									Validate: &osconfig.OSPolicyResourceExecResourceExec{
										Script:      "if exist /path8 exit 100\nexit 101",
										Interpreter: "SHELL",
									},
									Enforce: &osconfig.OSPolicyResourceExecResourceExec{
										Script:         "fsutil file createnew /path8 1000000\nexit 100",
										Interpreter:    "SHELL",
										OutputFilePath: "/path8",
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

// TestOSPolicyExecResourceWindows verifies that the OS Config agent enforces Exec resource policies on Windows:
// executing cmd and PowerShell scripts from GCS, remote URI, local path, and inline content, and capturing output.
func TestOSPolicyExecResourceWindows(t *testing.T) {
	const wantCompliances = 1
	tests := osPolicyExecWindowsTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.StartupScript(tt.image, ensureWindowsExecTestStateScript, "")
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
				return test.CreateOSPolicyAssignment(vm, buildWindowsExecOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})
		})
	}
}
