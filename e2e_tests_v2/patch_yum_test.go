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

var yumTestCases = []patchJobTestCase{
	{
		name:        "rhel-8",
		image:       "projects/rhel-cloud/global/images/family/rhel-8",
		machineType: "e2-standard-4",
		timeout:     40 * time.Minute,
	},
	{
		name:        "rhel-9",
		image:       "projects/rhel-cloud/global/images/family/rhel-9",
		machineType: "e2-standard-4",
		timeout:     40 * time.Minute,
	},
	{
		name:        "centos-stream-9",
		image:       "projects/centos-cloud/global/images/family/centos-stream-9",
		machineType: "e2-standard-4",
		timeout:     40 * time.Minute,
	},
	{
		name:        "rocky-linux-8",
		image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp",
		machineType: "e2-standard-4",
		timeout:     40 * time.Minute,
	},
	{
		name:        "rocky-linux-9",
		image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp",
		machineType: "e2-standard-4",
		timeout:     40 * time.Minute,
	},
}

func generateYumPatchJobRequest(desc string, vm *gcp.VM, yum *osconfig.YumSettings, timeout time.Duration) *osconfig.ExecutePatchJobRequest {
	return generateExecutePatchJobRequest(desc, vm, &osconfig.PatchConfig{
		RebootConfig: "DEFAULT",
		Yum:          yum,
	}, timeout)
}

// TestOSPatchYumExcludes verifies that YUM patching with security, minimal, and package excludes succeeds.
// This migrates "[YUM security, minimal and excludes]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchYumExcludes(t *testing.T) {
	for _, tc := range yumTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, generateVmMetadata(tc.extraMeta))
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with YUM security, minimal, and excludes", func(ctx context.Context) error {
				req := generateYumPatchJobRequest("e2e yum patch job test", vm, &osconfig.YumSettings{
					Security: true,
					Minimal:  true,
					Excludes: []string{"pkg1", "pkg2", "/pkg3/"},
				}, tc.timeout)
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after YUM patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})
		})
	}
}

// TestOSPatchYumExclusivePackages verifies that YUM patching with ExclusivePackages succeeds.
// This migrates "[YUM exclusive patches]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchYumExclusivePackages(t *testing.T) {
	for _, tc := range yumTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, generateVmMetadata(tc.extraMeta))
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with YUM exclusive packages", func(ctx context.Context) error {
				req := generateYumPatchJobRequest("e2e yum exclusive packages test", vm, &osconfig.YumSettings{
					ExclusivePackages: []string{"pkg1", "pk3"},
				}, tc.timeout)
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after YUM exclusive packages patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})
		})
	}
}
