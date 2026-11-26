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

const (
	ensureWindowsFileTestStateScript = `$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Stop-Service google_osconfig_agent -Force -ErrorAction SilentlyContinue
}

New-Item -ItemType File -Path /enforce_absent -Force -ErrorAction SilentlyContinue`

	windowsContentsMatchPolicyID   = "file-contents-match"
	windowsContentsMatchResourceID = "file-contents-match"
	windowsContentsMatchFilePath   = `C:\file_contents_match.txt`
	windowsContentsMatchExpected   = "expected-correct-content"

	contentsMatchSetupDoneMarker = "OSCONFIG_E2E_CONTENTS_MATCH_SETUP_DONE"
	contentsMatchVerifiedMarker  = "OSCONFIG_E2E_CONTENTS_MATCH_VERIFIED"

	windowsContentsMatchPreScript = `$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Stop-Service google_osconfig_agent -Force -ErrorAction SilentlyContinue
}

[System.IO.File]::WriteAllText('C:\file_contents_match.txt', 'initial-wrong-content')`

	windowsContentsMatchPostScript = `$path = 'C:\file_contents_match.txt'
if ([System.IO.File]::ReadAllText($path) -ceq 'initial-wrong-content') {
    for ($i = 0; $i -lt 5; $i++) {
        Write-Output ("OSCONFIG_E2E_" + "CONTENTS_MATCH_SETUP_DONE")
        Start-Sleep -Seconds 2
    }
}

while ($true) {
    try {
        if (Test-Path -LiteralPath $path) {
            $share = [System.IO.FileShare]([System.IO.FileShare]::ReadWrite -bor [System.IO.FileShare]::Delete)
            $fs = [System.IO.File]::Open($path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, $share)
            try {
                $sr = New-Object System.IO.StreamReader($fs, [System.Text.Encoding]::UTF8, $false)
                try {
                    $content = $sr.ReadToEnd()
                } finally {
                    $sr.Dispose()
                }
            } finally {
                $fs.Dispose()
            }
            if ($content -ceq 'expected-correct-content') {
                for ($i = 0; $i -lt 5; $i++) {
                    Write-Output ("OSCONFIG_E2E_" + "CONTENTS_MATCH_VERIFIED")
                    Start-Sleep -Seconds 2
                }
                break
            }
        }
    } catch {
        # Ignore transient read errors while the agent overwrites the file.
    }
    Start-Sleep -Seconds 2
}`
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
		{
			name:        "windows-2025",
			image:       "projects/windows-cloud/global/images/family/windows-2025",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2025-core",
			image:       "projects/windows-cloud/global/images/family/windows-2025-core",
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

// TestOSPolicyFileResourceWindows verifies that the OS Config agent enforces File resource policies on Windows:
// removing an existing file and creating files from content, GCS bucket, remote URI, and local path.
func TestOSPolicyFileResourceWindows(t *testing.T) {
	const wantCompliances = 2
	tests := osPolicyFileWindowsTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.StartupScript(tt.image, ensureWindowsFileTestStateScript, "")
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
				return test.CreateOSPolicyAssignment(vm, buildWindowsFileOSPolicyAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report", func(ctx context.Context) error {
				return test.WaitForOSPolicyCompliance(vm, wantCompliances)
			})
		})
	}
}

// buildWindowsFileContentsMatchAssignment constructs an OSPolicyAssignment with a FileResource
// targeting C:\file_contents_match.txt in CONTENTS_MATCH state with inline expected content.
func buildWindowsFileContentsMatchAssignment(vmName string) *osconfig.OSPolicyAssignment {
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
				Id:   windowsContentsMatchPolicyID,
				Mode: "ENFORCEMENT",
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: windowsContentsMatchResourceID,
								File: &osconfig.OSPolicyResourceFileResource{
									State:   "CONTENTS_MATCH",
									Path:    windowsContentsMatchFilePath,
									Content: windowsContentsMatchExpected,
								},
							},
						},
					},
				},
			},
		},
	}
}

// TestOSPolicyFileContentsMatchWindows verifies that the OS Config agent detects a SHA256 checksum
// mismatch on an existing Windows file (C:\file_contents_match.txt), overwrites it during enforcement
// when FileResource DesiredState is CONTENTS_MATCH, reports COMPLIANT with the expected config steps,
// and leaves the file containing the expected text on the VM.
func TestOSPolicyFileContentsMatchWindows(t *testing.T) {
	const wantCompliances = 1
	wantSteps := []string{
		testenv.OSPolicyConfigStepValidation,
		testenv.OSPolicyConfigStepDesiredStateCheck,
		testenv.OSPolicyConfigStepDesiredStateEnforcement,
		testenv.OSPolicyConfigStepDesiredStateCheckPostEnforcement,
	}

	for _, tt := range osPolicyFileWindowsTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var (
				vm     *gcp.VM
				report *osconfig.OSPolicyAssignmentReport
				err    error
			)

			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.StartupScript(tt.image, windowsContentsMatchPreScript, windowsContentsMatchPostScript)
				customMetadata := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
					scriptKey:                    scriptContent,
				}
				vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
				return err
			})

			test.Step("wait for initial file setup and agent inventory", func(ctx context.Context) error {
				if err = test.WaitForSerialMarkers(vm, contentsMatchSetupDoneMarker); err != nil {
					return err
				}
				_, err = test.WaitForInventory(vm)
				return err
			})

			test.Step("create os policy assignment", func(ctx context.Context) error {
				return test.CreateOSPolicyAssignment(vm, buildWindowsFileContentsMatchAssignment(vm.Name))
			})

			test.Step("wait for os policy compliance report and verify enforcement", func(ctx context.Context) error {
				report, err = test.WaitForOSPolicyComplianceReport(vm, wantCompliances)
				if err != nil {
					return err
				}
				return testenv.VerifyOSPolicyResourceConfigSteps(report, windowsContentsMatchPolicyID, windowsContentsMatchResourceID, wantSteps)
			})

			test.Step("verify overwritten file contents on VM", func(ctx context.Context) error {
				return test.WaitForSerialMarkers(vm, contentsMatchVerifiedMarker)
			})
		})
	}
}
