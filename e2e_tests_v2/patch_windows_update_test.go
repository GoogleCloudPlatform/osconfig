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

const syntheticWindowsKB = "9999999"

var (
	windowsPatchMeta = map[string]string{
		"sysprep-specialize-script-ps1": windowsSetWsus,
	}

	windowsPatchTestCases = []patchJobTestCase{
		{
			name:        "windows-2016",
			image:       "projects/windows-cloud/global/images/family/windows-2016",
			machineType: "e2-standard-4",
			timeout:     75 * time.Minute,
			extraMeta:   windowsPatchMeta,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     75 * time.Minute,
			extraMeta:   windowsPatchMeta,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     75 * time.Minute,
			extraMeta:   windowsPatchMeta,
		},
		{
			name:        "windows-2025",
			image:       "projects/windows-cloud/global/images/family/windows-2025",
			machineType: "e2-standard-4",
			timeout:     75 * time.Minute,
			extraMeta:   windowsPatchMeta,
		},
	}
)

// TestOSPatchWindowsClassifications verifies that Windows Update patching with specific classifications
// (CRITICAL and SECURITY) installs only updates belonging to those classifications while other updates
// such as definitions or drivers are skipped.
func TestOSPatchWindowsClassifications(t *testing.T) {
	for _, tc := range windowsPatchTestCases {
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
			var skippedKBs []string
			test.Step("wait for initial inventory", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				if err != nil {
					return err
				}
				skippedKBs = testenv.ExtractDefinitionOrDriverOnlyKBs(initialInv)
				return nil
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with CRITICAL and SECURITY classifications", func(ctx context.Context) error {
				var err error
				req := generateWindowsPatchJobRequest("e2e windows patch job classifications test", vm, &osconfig.WindowsUpdateSettings{
					Classifications: []string{"CRITICAL", "SECURITY"},
				}, tc.timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify definition and driver updates were skipped", func(ctx context.Context) error {
				postInv, err := test.WaitForInventoryUpdate(vm, initialInv.UpdateTime)
				if err != nil {
					return fmt.Errorf("failed to get post-patch inventory: %w", err)
				}
				return verifyKBsRemainPending(postInv, skippedKBs)
			})
		})
	}
}

// TestOSPatchWindowsExcludes verifies that Windows Update patching with Excludes skips
// the specified KB update, ensuring it is absent from installed inventory and remains available.
func TestOSPatchWindowsExcludes(t *testing.T) {
	for _, tc := range windowsPatchTestCases {
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
			var excludedKB string
			test.Step("wait for initial inventory and select KB to exclude", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				if err != nil {
					return err
				}
				excludedKB, _ = selectTargetKB(t, tc.name, initialInv)
				return nil
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with excluded KB", func(ctx context.Context) error {
				var err error
				req := generateWindowsPatchJobRequest("e2e windows patch job excludes test", vm, &osconfig.WindowsUpdateSettings{
					Excludes: []string{excludedKB},
				}, tc.timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify excluded KB was not installed", func(ctx context.Context) error {
				postInv, err := test.WaitForInventoryUpdate(vm, initialInv.UpdateTime)
				if err != nil {
					return fmt.Errorf("failed to get post-patch inventory: %w", err)
				}
				return verifyExcludedKB(postInv, excludedKB)
			})
		})
	}
}

// TestOSPatchWindowsExclusivePatches verifies that Windows Update patching with ExclusivePatches
// installs only the specified target KB while all other available updates remain pending.
func TestOSPatchWindowsExclusivePatches(t *testing.T) {
	for _, tc := range windowsPatchTestCases {
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
			var targetKB string
			var otherKBs []string
			test.Step("wait for initial inventory and select exclusive KB", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				if err != nil {
					return err
				}
				targetKB, otherKBs = selectTargetKB(t, tc.name, initialInv)
				return nil
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with exclusive patch", func(ctx context.Context) error {
				var err error
				req := generateWindowsPatchJobRequest("e2e windows patch job exclusive patches test", vm, &osconfig.WindowsUpdateSettings{
					ExclusivePatches: []string{targetKB},
				}, tc.timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify only target KB was installed and other updates remain pending", func(ctx context.Context) error {
				return verifyExclusiveKB(test, vm, initialInv.UpdateTime, targetKB, otherKBs)
			})
		})
	}
}

func generateWindowsPatchJobRequest(desc string, vm *gcp.VM, winSettings *osconfig.WindowsUpdateSettings, timeout time.Duration) *osconfig.ExecutePatchJobRequest {
	return generateExecutePatchJobRequest(desc, vm, &osconfig.PatchConfig{
		RebootConfig:  "DEFAULT",
		WindowsUpdate: winSettings,
	}, timeout)
}

// selectTargetKB returns the first available patchable KB along with any remaining KBs.
// If the image currently has no pending non-definition WUA updates, a synthetic KB ID is returned.
func selectTargetKB(t *testing.T, targetName string, inv *osconfig.Inventory) (string, []string) {
	t.Helper()
	availableKBs := testenv.ExtractPatchableWUAKBs(inv)
	if len(availableKBs) == 0 {
		t.Logf("no uninstalled WUA updates found on %s; using synthetic KB %s", targetName, syntheticWindowsKB)
		return syntheticWindowsKB, nil
	}
	t.Logf("selected KB %s on %s (remaining available KBs: %v)", availableKBs[0], targetName, availableKBs[1:])
	return availableKBs[0], availableKBs[1:]
}

func verifyKBsRemainPending(inv *osconfig.Inventory, kbs []string) error {
	installed := testenv.ExtractInstalledPackages(inv)
	available := testenv.ExtractAvailablePackages(inv)

	for _, kb := range kbs {
		if installed.HasKB(kb) {
			return fmt.Errorf("KB %s was installed, expected skipped", kb)
		}
		if !available.HasKB(kb) {
			return fmt.Errorf("KB %s should remain in available packages after patching", kb)
		}
	}
	return nil
}

func verifyExcludedKB(postInv *osconfig.Inventory, excludedKB string) error {
	if excludedKB == syntheticWindowsKB {
		if testenv.ExtractInstalledPackages(postInv).HasKB(excludedKB) {
			return fmt.Errorf("excluded KB %s was installed, expected absent from installed inventory", excludedKB)
		}
		return nil
	}
	return verifyKBsRemainPending(postInv, []string{excludedKB})
}

func verifyExclusiveKB(test *testenv.Test, vm *gcp.VM, prevUpdateTime, targetKB string, otherKBs []string) error {
	var (
		postInv *osconfig.Inventory
		err     error
	)
	if targetKB != syntheticWindowsKB {
		postInv, err = test.WaitForKBInstalled(vm, targetKB)
		if err != nil {
			return fmt.Errorf("exclusive target KB %s was not installed: %w", targetKB, err)
		}
	} else {
		postInv, err = test.WaitForInventoryUpdate(vm, prevUpdateTime)
		if err != nil {
			return fmt.Errorf("failed to get post-patch inventory: %w", err)
		}
	}
	return verifyKBsRemainPending(postInv, otherKBs)
}
