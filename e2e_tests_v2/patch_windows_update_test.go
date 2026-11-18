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
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	"google.golang.org/api/osconfig/v1"
)

var (
	kbRegex = regexp.MustCompile(`(?i)KB(\d+)`)

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

func generateWindowsPatchJobRequest(desc string, vm *gcp.VM, winSettings *osconfig.WindowsUpdateSettings, timeout time.Duration) *osconfig.ExecutePatchJobRequest {
	return &osconfig.ExecutePatchJobRequest{
		Description: fmt.Sprintf("%s for %s", desc, vm.Name),
		InstanceFilter: &osconfig.PatchInstanceFilter{
			Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
		},
		PatchConfig: &osconfig.PatchConfig{
			RebootConfig:  "DEFAULT",
			WindowsUpdate: winSettings,
		},
		Duration: fmt.Sprintf("%ds", int(timeout.Seconds())),
	}
}

func windowsPatchMetadata(extraMeta map[string]string) map[string]string {
	meta := map[string]string{
		"osconfig-disabled-features": "guestpolicies",
	}
	for k, v := range extraMeta {
		meta[k] = v
	}
	return meta
}

func normalizeKB(kb string) string {
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(kb)), "KB")
}

func isDefenderOrDefinitionUpdate(wua *osconfig.InventoryWindowsUpdatePackage) bool {
	if wua == nil {
		return false
	}
	titleLower := strings.ToLower(wua.Title)
	if strings.Contains(titleLower, "defender") ||
		strings.Contains(titleLower, "security intelligence") ||
		strings.Contains(titleLower, "antivirus") {
		return true
	}
	for _, cat := range wua.Categories {
		if strings.EqualFold(cat.Name, "Definition Updates") || cat.Id == wuaCategoryDefinition {
			return true
		}
	}
	for _, kb := range wua.KbArticleIds {
		if normalizeKB(kb) == "2267602" {
			return true
		}
	}
	return false
}

func extractAvailableKBIDs(inv *osconfig.Inventory) []string {
	if inv == nil {
		return nil
	}
	seen := make(map[string]bool)
	var kbs []string

	for _, item := range inv.Items {
		if item.AvailablePackage == nil || item.AvailablePackage.WuaPackage == nil {
			continue
		}
		wua := item.AvailablePackage.WuaPackage
		// Skip Microsoft Defender / antivirus definition updates because:
		// 1) The WUA COM API often does not populate KBArticleIDs on definition updates,
		//    causing the agent's exclude filter to miss them.
		// 2) Defender background service (WinDefend) auto-installs definitions out-of-band.
		// 3) An older revision of KB2267602 is already present in installed inventory.
		if isDefenderOrDefinitionUpdate(wua) {
			continue
		}
		for _, kb := range wua.KbArticleIds {
			clean := normalizeKB(kb)
			if clean == "" || clean == "2267602" || seen[clean] {
				continue
			}
			// Only select KBs that are not already installed on the system.
			if isKBInstalled(inv, clean) {
				continue
			}
			seen[clean] = true
			kbs = append(kbs, clean)
		}
	}
	return kbs
}

func isKBInstalled(inv *osconfig.Inventory, kb string) bool {
	if inv == nil {
		return false
	}
	target := normalizeKB(kb)
	if target == "" {
		return false
	}

	for _, item := range inv.Items {
		if item.InstalledPackage == nil {
			continue
		}
		if qfe := item.InstalledPackage.QfePackage; qfe != nil {
			if normalizeKB(qfe.HotFixId) == target {
				return true
			}
		}
		if wua := item.InstalledPackage.WuaPackage; wua != nil {
			for _, k := range wua.KbArticleIds {
				if normalizeKB(k) == target {
					return true
				}
			}
			if matches := kbRegex.FindAllStringSubmatch(wua.Title, -1); len(matches) > 0 {
				for _, m := range matches {
					if normalizeKB(m[1]) == target {
						return true
					}
				}
			}
		}
	}
	return false
}

func isKBAvailable(inv *osconfig.Inventory, kb string) bool {
	if inv == nil {
		return false
	}
	target := normalizeKB(kb)
	if target == "" {
		return false
	}

	for _, item := range inv.Items {
		if item.AvailablePackage == nil || item.AvailablePackage.WuaPackage == nil {
			continue
		}
		wua := item.AvailablePackage.WuaPackage
		for _, k := range wua.KbArticleIds {
			if normalizeKB(k) == target {
				return true
			}
		}
		if matches := kbRegex.FindAllStringSubmatch(wua.Title, -1); len(matches) > 0 {
			for _, m := range matches {
				if normalizeKB(m[1]) == target {
					return true
				}
			}
		}
	}
	return false
}

const (
	wuaCategoryCritical   = "e6cf1350-c01b-414d-a61f-263d14d133b4"
	wuaCategorySecurity   = "0fa1201d-4330-4fa8-8ae9-b877473b6441"
	wuaCategoryDefinition = "e0789628-ce08-4437-be74-2495b842f43b"
	wuaCategoryDriver     = "ebfc1fc5-71a4-4f7b-9aca-3b9a503104a0"
)

func hasCategory(categories []*osconfig.InventoryWindowsUpdatePackageWindowsUpdateCategory, targetID, targetName string) bool {
	for _, cat := range categories {
		if strings.EqualFold(cat.Id, targetID) {
			return true
		}
		if strings.Contains(strings.ToLower(cat.Name), strings.ToLower(targetName)) {
			return true
		}
	}
	return false
}

func waitForInventoryUpdate(ctx context.Context, test *testenv.Test, vm *gcp.VM, initialUpdateTime string) (*osconfig.Inventory, error) {
	var postInv *osconfig.Inventory
	err := gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("inventory update on %s", vm.Name), func(pollCtx context.Context) (string, bool, error) {
		inv, err := test.Suite.Compute.GetInventory(pollCtx, vm.Project, vm.Zone, vm.Name, "FULL")
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("inventory not yet published (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read inventory: %w", err)
		}
		postInv = inv
		if initialUpdateTime != "" && inv.UpdateTime == initialUpdateTime {
			return fmt.Sprintf("inventory updateTime %s has not updated yet", inv.UpdateTime), false, nil
		}
		return fmt.Sprintf("inventory updated at %s", inv.UpdateTime), true, nil
	})
	if err != nil {
		return nil, err
	}
	return postInv, nil
}

func waitForKBInstalled(ctx context.Context, test *testenv.Test, vm *gcp.VM, targetKB string) (*osconfig.Inventory, error) {
	var postInv *osconfig.Inventory
	err := gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("KB %s installed in inventory on %s", targetKB, vm.Name), func(pollCtx context.Context) (string, bool, error) {
		inv, err := test.Suite.Compute.GetInventory(pollCtx, vm.Project, vm.Zone, vm.Name, "FULL")
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("inventory not yet published (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read inventory: %w", err)
		}
		postInv = inv
		if isKBInstalled(inv, targetKB) {
			return fmt.Sprintf("KB %s is installed", targetKB), true, nil
		}
		return fmt.Sprintf("waiting for KB %s in installed inventory", targetKB), false, nil
	})
	if err != nil {
		return nil, err
	}
	return postInv, nil
}

func verifyClassificationFilter(initialInv, postInv *osconfig.Inventory) error {
	if initialInv == nil || postInv == nil {
		return nil
	}

	// Identify any definition or driver updates that were available prior to patching.
	var skippedKBs []string
	for _, item := range initialInv.Items {
		if item.AvailablePackage == nil || item.AvailablePackage.WuaPackage == nil {
			continue
		}
		wua := item.AvailablePackage.WuaPackage
		isDefOrDriver := hasCategory(wua.Categories, wuaCategoryDefinition, "definition") ||
			hasCategory(wua.Categories, wuaCategoryDriver, "driver")
		isCritOrSec := hasCategory(wua.Categories, wuaCategoryCritical, "critical") ||
			hasCategory(wua.Categories, wuaCategorySecurity, "security")

		if isDefOrDriver && !isCritOrSec {
			for _, kb := range wua.KbArticleIds {
				clean := normalizeKB(kb)
				// Skip KBs already installed prior to patch job and skip Defender definitions (2267602)
				// which may auto-update out-of-band via the Defender background service.
				if clean != "" && clean != "2267602" && !isKBInstalled(initialInv, clean) {
					skippedKBs = append(skippedKBs, clean)
				}
			}
		}
	}

	// Verify that definition/driver updates were not installed and remain available.
	for _, kb := range skippedKBs {
		if isKBInstalled(postInv, kb) {
			return fmt.Errorf("definition/driver update KB %s was installed, expected skipped under CRITICAL/SECURITY classification filter", kb)
		}
		if !isKBAvailable(postInv, kb) {
			return fmt.Errorf("definition/driver update KB %s should remain in available packages after filtered patch job", kb)
		}
	}

	return nil
}

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
				vm, err = test.CreateVM(tc.image, tc.machineType, windowsPatchMetadata(tc.extraMeta))
				return err
			})

			var initialInv *osconfig.Inventory
			test.Step("wait for OS Config agent ready and get initial inventory", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with CRITICAL and SECURITY classifications", func(ctx context.Context) error {
				req := generateWindowsPatchJobRequest(
					"e2e windows patch job classifications test",
					vm,
					&osconfig.WindowsUpdateSettings{
						Classifications: []string{"CRITICAL", "SECURITY"},
					},
					tc.timeout,
				)
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify only critical and security updates were installed and other classifications skipped", func(ctx context.Context) error {
				var initialUpdateTime string
				if initialInv != nil {
					initialUpdateTime = initialInv.UpdateTime
				}
				postInv, err := waitForInventoryUpdate(ctx, test, vm, initialUpdateTime)
				if err != nil {
					return fmt.Errorf("failed to get post-patch inventory: %w", err)
				}
				return verifyClassificationFilter(initialInv, postInv)
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
				vm, err = test.CreateVM(tc.image, tc.machineType, windowsPatchMetadata(tc.extraMeta))
				return err
			})

			var initialInv *osconfig.Inventory
			var availableKBs []string
			test.Step("wait for OS Config agent ready and query available WUA updates", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				if err != nil {
					return err
				}
				availableKBs = extractAvailableKBIDs(initialInv)
				t.Logf("found %d available KB updates on %s: %v", len(availableKBs), tc.name, availableKBs)
				return nil
			})

			var excludedKB string
			var isRealKB bool
			test.Step("execute and await patch job with excluded KB", func(ctx context.Context) error {
				if len(availableKBs) > 0 {
					excludedKB = availableKBs[0]
					isRealKB = true
					t.Logf("testing excludes with available KB %s on %s", excludedKB, tc.name)
				} else {
					excludedKB = "9999999"
					isRealKB = false
					t.Logf("no uninstalled WUA updates with KBArticleIDs found on %s; using synthetic excluded KB %s", tc.name, excludedKB)
				}

				req := generateWindowsPatchJobRequest(
					"e2e windows patch job excludes test",
					vm,
					&osconfig.WindowsUpdateSettings{
						Excludes: []string{excludedKB},
					},
					tc.timeout,
				)
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
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
				var initialUpdateTime string
				if initialInv != nil {
					initialUpdateTime = initialInv.UpdateTime
				}
				postInv, err := waitForInventoryUpdate(ctx, test, vm, initialUpdateTime)
				if err != nil {
					return fmt.Errorf("failed to get post-patch inventory: %w", err)
				}

				if isKBInstalled(postInv, excludedKB) {
					return fmt.Errorf("excluded KB %s was installed, expected absent from installed inventory", excludedKB)
				}
				if isRealKB && !isKBAvailable(postInv, excludedKB) {
					return fmt.Errorf("excluded KB %s should remain in available packages after patching", excludedKB)
				}
				return nil
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
				vm, err = test.CreateVM(tc.image, tc.machineType, windowsPatchMetadata(tc.extraMeta))
				return err
			})

			var initialInv *osconfig.Inventory
			var availableKBs []string
			test.Step("wait for OS Config agent ready and query available WUA updates", func(ctx context.Context) error {
				var err error
				initialInv, err = test.WaitForInventory(vm)
				if err != nil {
					return err
				}
				availableKBs = extractAvailableKBIDs(initialInv)
				t.Logf("found %d available KB updates on %s: %v", len(availableKBs), tc.name, availableKBs)
				return nil
			})

			var targetKB string
			var otherKBs []string
			var isRealKB bool
			test.Step("execute and await patch job with exclusive patch", func(ctx context.Context) error {
				if len(availableKBs) > 0 {
					targetKB = availableKBs[0]
					isRealKB = true
					if len(availableKBs) > 1 {
						otherKBs = availableKBs[1:]
					}
					t.Logf("testing exclusive patch with available KB %s on %s (other pending KBs: %v)", targetKB, tc.name, otherKBs)
				} else {
					targetKB = "9999999"
					isRealKB = false
					t.Logf("no uninstalled WUA updates with KBArticleIDs found on %s; using synthetic target KB %s", tc.name, targetKB)
				}

				req := generateWindowsPatchJobRequest(
					"e2e windows patch job exclusive patches test",
					vm,
					&osconfig.WindowsUpdateSettings{
						ExclusivePatches: []string{targetKB},
					},
					tc.timeout,
				)
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
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
				var postInv *osconfig.Inventory
				if isRealKB {
					var err error
					postInv, err = waitForKBInstalled(ctx, test, vm, targetKB)
					if err != nil {
						return fmt.Errorf("exclusive target KB %s was not installed: %w", targetKB, err)
					}
				} else {
					var initialUpdateTime string
					if initialInv != nil {
						initialUpdateTime = initialInv.UpdateTime
					}
					var err error
					postInv, err = waitForInventoryUpdate(ctx, test, vm, initialUpdateTime)
					if err != nil {
						return fmt.Errorf("failed to get post-patch inventory: %w", err)
					}
				}

				if isRealKB && !isKBInstalled(postInv, targetKB) {
					return fmt.Errorf("exclusive target KB %s was not installed", targetKB)
				}

				for _, otherKB := range otherKBs {
					if isKBInstalled(postInv, otherKB) {
						return fmt.Errorf("non-exclusive KB %s was installed, expected only %s to be installed", otherKB, targetKB)
					}
					if !isKBAvailable(postInv, otherKB) {
						return fmt.Errorf("non-exclusive KB %s should remain pending in available packages", otherKB)
					}
				}
				return nil
			})
		})
	}
}
