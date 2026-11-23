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
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/osconfig/v1"
)

const (
	execSetupDoneMarker    = "OSCONFIG_E2E_EXEC_SETUP_DONE"
	preStepFileMarker      = "OSCONFIG_E2E_PRE_STEP_FILE_EXISTS"
	postStepFileMarker     = "OSCONFIG_E2E_POST_STEP_FILE_EXISTS"
	windowsPreStepCmdPath  = `C:\ProgramData\Google\osconfig_e2e_pre_step.cmd`
	windowsPostStepPs1Path = `C:\ProgramData\Google\osconfig_e2e_post_step.ps1`
	execStepNoOpWindowsKB  = "9999999"

	windowsExecStepStartupScript = `
New-Item -ItemType Directory -Force -Path 'C:\ProgramData\Google' | Out-Null
$setupDone = 'C:\ProgramData\Google\osconfig_e2e_exec_setup_done'
if (-not (Test-Path $setupDone)) {
    Remove-Item -Path 'C:\pre_step_ran.txt', 'C:\post_step_ran.txt' -Force -ErrorAction SilentlyContinue

    $preStepScript = @'
@echo off
echo pre-step ran > C:\pre_step_ran.txt
exit 42
'@
    Set-Content -Path 'C:\ProgramData\Google\osconfig_e2e_pre_step.cmd' -Encoding Ascii -Value $preStepScript -Force

    $postStepScript = @'
Set-Content -Path 'C:\post_step_ran.txt' -Value 'post-step ran' -Force
exit 100
'@
    Set-Content -Path 'C:\ProgramData\Google\osconfig_e2e_post_step.ps1' -Encoding Ascii -Value $postStepScript -Force

    $svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -ne 'Running') {
        Start-Service google_osconfig_agent -ErrorAction SilentlyContinue
    }
    Set-Content -Path $setupDone -Value 'done' -Force
    Write-Output ("OSCONFIG_E2E_" + "EXEC_SETUP_DONE")
}

$preReported = $false
$postReported = $false
while (-not ($preReported -and $postReported)) {
    if (-not $preReported -and (Test-Path 'C:\pre_step_ran.txt')) {
        Write-Output ("OSCONFIG_E2E_" + "PRE_STEP_FILE_EXISTS")
        $preReported = $true
    }
    if (-not $postReported -and (Test-Path 'C:\post_step_ran.txt')) {
        Write-Output ("OSCONFIG_E2E_" + "POST_STEP_FILE_EXISTS")
        $postReported = $true
    }
    if (-not ($preReported -and $postReported)) {
        Start-Sleep -Seconds 2
    }
}
`
)

var (
	windowsDryRunExecMeta = map[string]string{
		"sysprep-specialize-script-ps1": windowsSetWsus,
	}

	windowsDryRunExecTestCases = []patchJobTestCase{
		{
			name:        "windows-2016",
			image:       "projects/windows-cloud/global/images/family/windows-2016",
			machineType: "e2-standard-4",
			timeout:     60 * time.Minute,
			extraMeta:   windowsDryRunExecMeta,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     60 * time.Minute,
			extraMeta:   windowsDryRunExecMeta,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     60 * time.Minute,
			extraMeta:   windowsDryRunExecMeta,
		},
		{
			name:        "windows-2025",
			image:       "projects/windows-cloud/global/images/family/windows-2025",
			machineType: "e2-standard-4",
			timeout:     60 * time.Minute,
			extraMeta:   windowsDryRunExecMeta,
		},
		{
			name:        "windows-2025-core",
			image:       "projects/windows-cloud/global/images/family/windows-2025-core",
			machineType: "e2-standard-4",
			timeout:     60 * time.Minute,
			extraMeta:   windowsDryRunExecMeta,
		},
	}
)

// TestOSPatchWindowsDryRun verifies that executing a PatchJob with DryRun=true on Windows
// reaches SUCCEEDED for both the job and instance details without modifying installed
// WUA, QFE, or GooGet packages.
func TestOSPatchWindowsDryRun(t *testing.T) {
	for _, tc := range windowsDryRunExecTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, generateVmMetadata(tc.extraMeta))
				return err
			})

			var initialInv *osconfig.Inventory
			var beforeSnapshot testenv.WindowsInstalledSnapshot
			test.Step("wait for initial inventory and snapshot installed packages", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				if err != nil {
					return err
				}
				beforeSnapshot = testenv.SnapshotWindowsInstalledPackages(initialInv)
				if len(beforeSnapshot.GooGet) == 0 {
					return fmt.Errorf("expected non-empty GooGet package snapshot before dry-run")
				}
				return nil
			})

			var job *osconfig.PatchJob
			test.Step("execute dry-run patch job", func(ctx context.Context) error {
				var err error
				req := generateExecutePatchJobRequest("e2e windows dry-run patch job test", vm, &osconfig.PatchConfig{
					RebootConfig: "DEFAULT",
				}, tc.timeout)
				req.DryRun = true
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for dry-run patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.State != testenv.PatchJobStateSucceeded {
					return fmt.Errorf("expected patch job state %s, got %s", testenv.PatchJobStateSucceeded, pj.State)
				}
				// DryRun never reboots, so if Windows background activity (e.g. MicrosoftEdgeUpdate)
				// sets PendingFileRenameOperations before patching, the instance legitimately completes
				// as SUCCEEDED_REBOOT_REQUIRED.
				succeededTotal := pj.InstanceDetailsSummary.SucceededInstanceCount + pj.InstanceDetailsSummary.SucceededRebootRequiredInstanceCount
				if succeededTotal != 1 {
					return fmt.Errorf("expected 1 succeeded instance after dry-run patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify patch job instance details succeeded", func(ctx context.Context) error {
				details, err := test.ListPatchJobInstanceDetails(job.Name)
				if err != nil {
					return fmt.Errorf("list patch job instance details: %w", err)
				}
				if len(details) != 1 {
					return fmt.Errorf("expected 1 instance detail for patch job %s, got %d", job.Name, len(details))
				}
				if details[0].State != testenv.PatchJobStateSucceeded && details[0].State != testenv.PatchJobInstanceStateSucceededRebootRequired {
					return fmt.Errorf("expected instance detail state %s or %s, got %s (failure: %q)",
						testenv.PatchJobStateSucceeded, testenv.PatchJobInstanceStateSucceededRebootRequired, details[0].State, details[0].FailureReason)
				}
				return nil
			})

			test.Step("verify installed packages are unchanged after dry-run", func(ctx context.Context) error {
				postInv, err := test.WaitForInventoryUpdate(vm, initialInv.UpdateTime)
				if err != nil {
					return fmt.Errorf("wait for post-patch inventory: %w", err)
				}
				afterSnapshot := testenv.SnapshotWindowsInstalledPackages(postInv)
				if diff := cmp.Diff(beforeSnapshot, afterSnapshot); diff != "" {
					return fmt.Errorf("installed packages changed after dry-run (-before +after):\n%s", diff)
				}
				return nil
			})
		})
	}
}

// TestOSPatchWindowsExecStepShellAndExitCodes verifies that Windows PatchConfig PreStep
// runs a .cmd script with Interpreter=SHELL exiting with 42, PostStep runs a .ps1 script
// with Interpreter=POWERSHELL exiting with 100, both succeed under AllowedSuccessCodes
// [0, 42, 100], and both marker files are created on the VM.
func TestOSPatchWindowsExecStepShellAndExitCodes(t *testing.T) {
	for _, tc := range windowsDryRunExecTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, execStepMetadata(tc.extraMeta))
				return err
			})

			test.Step("wait for exec step setup and OS Config agent ready", func(ctx context.Context) error {
				if err := test.WaitForSerialMarkers(vm, execSetupDoneMarker); err != nil {
					return err
				}
				_, err := test.WaitForInventory(vm)
				return err
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with SHELL pre-step and POWERSHELL post-step", func(ctx context.Context) error {
				var err error
				req := generateExecStepPatchJobRequest("e2e windows shell and exit codes test", vm, tc.timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after exec step patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify pre-step and post-step marker files exist on VM", func(ctx context.Context) error {
				return test.WaitForSerialMarkers(vm, preStepFileMarker, postStepFileMarker)
			})
		})
	}
}

func execStepMetadata(base map[string]string) map[string]string {
	meta := generateVmMetadata(base)
	meta[gcp.MetadataKeyWindowsStartupScript] = windowsExecStepStartupScript
	return meta
}

func generateExecStepPatchJobRequest(desc string, vm *gcp.VM, timeout time.Duration) *osconfig.ExecutePatchJobRequest {
	allowedCodes := []int64{0, 42, 100}
	return generateExecutePatchJobRequest(desc, vm, &osconfig.PatchConfig{
		RebootConfig: "DEFAULT",
		WindowsUpdate: &osconfig.WindowsUpdateSettings{
			ExclusivePatches: []string{execStepNoOpWindowsKB},
		},
		PreStep: &osconfig.ExecStep{
			WindowsExecStepConfig: &osconfig.ExecStepConfig{
				LocalPath:           windowsPreStepCmdPath,
				Interpreter:         "SHELL",
				AllowedSuccessCodes: allowedCodes,
			},
		},
		PostStep: &osconfig.ExecStep{
			WindowsExecStepConfig: &osconfig.ExecStepConfig{
				LocalPath:           windowsPostStepPs1Path,
				Interpreter:         "POWERSHELL",
				AllowedSuccessCodes: allowedCodes,
			},
		},
	}, timeout)
}
