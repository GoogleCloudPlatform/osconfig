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

var zypperTestCases = []patchJobTestCase{
	{
		name:        "sles-12",
		image:       "projects/suse-cloud/global/images/family/sles-12",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	{
		name:        "sles-15",
		image:       "projects/suse-cloud/global/images/family/sles-15",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	{
		name:        "opensuse-leap-15",
		image:       "projects/opensuse-cloud/global/images/family/opensuse-leap",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
}

func runZypperPatchTest(t *testing.T, tc patchJobTestCase, stepName, reqDesc string, zypperSettings *osconfig.ZypperSettings) {
	t.Helper()

	test := testenv.New(t, tc.timeout)

	var vm *gcp.VM
	test.Step("create VM", func(ctx context.Context) error {
		meta := map[string]string{
			"osconfig-disabled-features": "guestpolicies",
		}
		var err error
		vm, err = test.CreateVM(tc.image, tc.machineType, meta)
		return err
	})

	test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
		_, err := test.WaitForInventory(vm)
		return err
	})

	test.Step(stepName, func(ctx context.Context) error {
		req := &osconfig.ExecutePatchJobRequest{
			Description: fmt.Sprintf("%s for %s", reqDesc, vm.Name),
			InstanceFilter: &osconfig.PatchInstanceFilter{
				Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
			},
			PatchConfig: &osconfig.PatchConfig{
				RebootConfig: "DEFAULT",
				Zypper:       zypperSettings,
			},
			Duration: fmt.Sprintf("%ds", int(tc.timeout.Seconds())),
		}
		job, err := test.ExecutePatchJob(req)
		if err != nil {
			return err
		}
		_, err = test.WaitForPatchJob(job.Name)
		return err
	})
}

// TestOSPatchZypperExcludes verifies that Zypper patching with excludes, WithOptional,
// WithUpdate, categories, and severities succeeds.
// This migrates "[Zypper excludes, WithOptional, WithUpdate, Categories and Severities]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchZypperExcludes(t *testing.T) {
	for _, tc := range zypperTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runZypperPatchTest(t, tc,
				"execute and await patch job with Zypper settings",
				"e2e zypper patch job test",
				&osconfig.ZypperSettings{
					Excludes:     []string{"patch-1", "/patch-2/"},
					WithOptional: true,
					WithUpdate:   true,
					Categories:   []string{"security", "recommended", "feature"},
					Severities:   []string{"critical", "important", "moderate", "low"},
				},
			)
		})
	}
}

// TestOSPatchZypperExclusivePatches verifies that Zypper patching with ExclusivePatches succeeds
// without error.
// This migrates "[Zypper exclusivePatches]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchZypperExclusivePatches(t *testing.T) {
	for _, tc := range zypperTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runZypperPatchTest(t, tc,
				"execute and await patch job with Zypper exclusive patches",
				"e2e zypper exclusive patches test",
				&osconfig.ZypperSettings{
					ExclusivePatches: []string{"patch-1"},
				},
			)
		})
	}
}

