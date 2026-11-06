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
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	"google.golang.org/api/osconfig/v1"
)

const (
	linuxLocalPrePatchScript = `
cat << 'EOF' | tee /linux_local_pre_patch_script.sh ./linux_local_pre_patch_script.sh
#!/bin/sh
echo "pre-step ran"
exit 0
EOF
chmod +x /linux_local_pre_patch_script.sh ./linux_local_pre_patch_script.sh
`

	windowsLocalPostPatchScript = `
New-Item -Path 'C:\Windows\System32' -Name "windows_local_post_patch_script.ps1" -ItemType "file" -Value 'Write-Output "post-step ran"' -Force
`
)

var windowsPrePostMeta = map[string]string{
	"sysprep-specialize-script-ps1": windowsSetWsus,
}

func patchConfigWithPrePostSteps() *osconfig.PatchConfig {
	return &osconfig.PatchConfig{
		RebootConfig: "DEFAULT",
		PreStep: &osconfig.ExecStep{
			LinuxExecStepConfig: &osconfig.ExecStepConfig{
				LocalPath:   "./linux_local_pre_patch_script.sh",
				Interpreter: "SHELL",
			},
			WindowsExecStepConfig: &osconfig.ExecStepConfig{
				GcsObject: &osconfig.GcsObject{
					Bucket:           "osconfig-agent-end2end-test-resources",
					Object:           "OSPatch/windows_gcs_pre_patch_script.ps1",
					GenerationNumber: 1571249543230832,
				},
				Interpreter: "POWERSHELL",
			},
		},
		PostStep: &osconfig.ExecStep{
			LinuxExecStepConfig: &osconfig.ExecStepConfig{
				GcsObject: &osconfig.GcsObject{
					Bucket:           "osconfig-agent-end2end-test-resources",
					Object:           "OSPatch/linux_gcs_post_patch_script",
					GenerationNumber: 1570567792146617,
				},
			},
			WindowsExecStepConfig: &osconfig.ExecStepConfig{
				LocalPath:   `C:\Windows\System32\windows_local_post_patch_script.ps1`,
				Interpreter: "POWERSHELL",
			},
		},
	}
}

func prePostMetadata(tc patchJobTestCase) map[string]string {
	meta := patchTestMetadata(tc.extraMeta)

	scriptKey, defaultScript := gcp.DefaultStartupScript(tc.image)
	if strings.Contains(strings.ToLower(tc.image), "windows") {
		meta[scriptKey] = defaultScript + "\n" + windowsLocalPostPatchScript
	} else {
		meta[scriptKey] = defaultScript + "\n" + linuxLocalPrePatchScript
	}
	return meta
}

var prePostTestCases = []patchJobTestCase{
	// Debian
	{
		name:        "debian-12",
		image:       "projects/debian-cloud/global/images/family/debian-12",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	{
		name:        "debian-13",
		image:       "projects/debian-cloud/global/images/family/debian-13",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	// Ubuntu
	{
		name:        "ubuntu-2204-lts",
		image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	{
		name:        "ubuntu-2404-lts",
		image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	// EL (RHEL, CentOS, Rocky)
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
	// SUSE
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
	// Windows
	{
		name:        "windows-2016",
		image:       "projects/windows-cloud/global/images/family/windows-2016",
		machineType: "e2-standard-4",
		timeout:     60 * time.Minute,
		extraMeta:   windowsPrePostMeta,
	},
	{
		name:        "windows-2019",
		image:       "projects/windows-cloud/global/images/family/windows-2019",
		machineType: "e2-standard-4",
		timeout:     45 * time.Minute,
		extraMeta:   windowsPrePostMeta,
	},
	{
		name:        "windows-2022",
		image:       "projects/windows-cloud/global/images/family/windows-2022",
		machineType: "e2-standard-4",
		timeout:     45 * time.Minute,
		extraMeta:   windowsPrePostMeta,
	},
}

// TestOSPatchPrePostSteps verifies that pre-step and post-step execution succeeds
// for both Linux and Windows instances using local and GCS-hosted scripts.
// This migrates "[PatchJob runs pre-step and post-step]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchPrePostSteps(t *testing.T) {
	for _, tc := range prePostTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, prePostMetadata(tc))
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with pre and post steps", func(ctx context.Context) error {
				req := generateExecutePatchJobRequest("e2e pre/post steps patch job test", vm, patchConfigWithPrePostSteps(), tc.timeout)
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 {
					return fmt.Errorf("expected 1 succeeded instance after pre/post steps patch job, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})
		})
	}
}
