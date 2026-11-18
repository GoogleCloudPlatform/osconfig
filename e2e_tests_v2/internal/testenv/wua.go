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

package testenv

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"google.golang.org/api/osconfig/v1"
)

const (
	wuaCategoryCritical   = "e6cf1350-c01b-414d-a61f-263d14d133b4"
	wuaCategorySecurity   = "0fa1201d-4330-4fa8-8ae9-b877473b6441"
	wuaCategoryDefinition = "e0789628-ce08-4437-be74-2495b842f43b"
	wuaCategoryDriver     = "ebfc1fc5-71a4-4f7b-9aca-3b9a503104a0"

	// defenderKB is the recurring Microsoft Defender Antivirus definition KB,
	// which auto-installs out-of-band via the Defender background service.
	defenderKB = "2267602"
)

var kbRegex = regexp.MustCompile(`(?i)KB(\d+)`)

func extractWUAPackage(wua *osconfig.InventoryWindowsUpdatePackage) *WUAPackage {
	if wua == nil {
		return nil
	}
	var categories []WUACategory
	for _, cat := range wua.Categories {
		if cat != nil {
			categories = append(categories, WUACategory{
				ID:   cat.Id,
				Name: cat.Name,
			})
		}
	}
	return &WUAPackage{
		Title:        wua.Title,
		KBArticleIDs: append([]string(nil), wua.KbArticleIds...),
		Categories:   categories,
	}
}

func normalizeKB(kb string) string {
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(kb)), "KB")
}

// HasKB reports whether kb (with or without "KB" prefix) is present in QFE or WUA packages.
func (pkgs *InstalledPackages) HasKB(kb string) bool {
	if pkgs == nil {
		return false
	}
	target := normalizeKB(kb)
	if target == "" {
		return false
	}
	for _, qfe := range pkgs.QFE {
		if qfe != nil && normalizeKB(qfe.HotFixID) == target {
			return true
		}
	}
	for _, wua := range pkgs.WUA {
		if wua != nil && wua.matchesKB(target) {
			return true
		}
	}
	return false
}

func (wua *WUAPackage) matchesKB(normalizedTarget string) bool {
	for _, k := range wua.KBArticleIDs {
		if normalizeKB(k) == normalizedTarget {
			return true
		}
	}
	for _, m := range kbRegex.FindAllStringSubmatch(wua.Title, -1) {
		if normalizeKB(m[1]) == normalizedTarget {
			return true
		}
	}
	return false
}

func (wua *WUAPackage) hasCategory(targetID, targetNameSubstring string) bool {
	for _, cat := range wua.Categories {
		if strings.EqualFold(cat.ID, targetID) ||
			strings.Contains(strings.ToLower(cat.Name), strings.ToLower(targetNameSubstring)) {
			return true
		}
	}
	return false
}

// ExtractDefinitionOrDriverOnlyKBs returns available uninstalled KB IDs classified as
// DEFINITION or DRIVER that do not belong to CRITICAL or SECURITY classifications.
func ExtractDefinitionOrDriverOnlyKBs(inv *osconfig.Inventory) []string {
	available := ExtractAvailablePackages(inv)
	if available == nil {
		return nil
	}
	installed := ExtractInstalledPackages(inv)
	seen := make(map[string]bool)
	var kbs []string

	for _, wua := range available.WUA {
		if wua == nil {
			continue
		}
		isDefOrDriver := wua.hasCategory(wuaCategoryDefinition, "definition") ||
			wua.hasCategory(wuaCategoryDriver, "driver")
		isCritOrSec := wua.hasCategory(wuaCategoryCritical, "critical") ||
			wua.hasCategory(wuaCategorySecurity, "security")
		if !isDefOrDriver || isCritOrSec {
			continue
		}
		for _, kb := range wua.KBArticleIDs {
			clean := normalizeKB(kb)
			if clean == "" || clean == defenderKB || seen[clean] || installed.HasKB(clean) {
				continue
			}
			seen[clean] = true
			kbs = append(kbs, clean)
		}
	}
	return kbs
}

// WaitForInventoryUpdate polls the OS Config Inventory API until UpdateTime changes from prevUpdateTime.
func (test *Test) WaitForInventoryUpdate(vm *gcp.VM, prevUpdateTime string) (*osconfig.Inventory, error) {
	test.t.Helper()

	var result *osconfig.Inventory
	err := gcp.PollUntil(test.Context, test.Suite.Config.PollInterval, fmt.Sprintf("inventory update on %s", vm.Name), func(ctx context.Context) (string, bool, error) {
		inv, err := test.Suite.Compute.GetInventory(ctx, vm.Project, vm.Zone, vm.Name, "FULL")
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("inventory not yet published (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read inventory: %w", err)
		}
		if prevUpdateTime != "" && inv.UpdateTime == prevUpdateTime {
			return fmt.Sprintf("inventory updateTime %s has not updated yet", inv.UpdateTime), false, nil
		}
		result = inv
		return fmt.Sprintf("inventory updated at %s", inv.UpdateTime), true, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
