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

const windowsSetWsus = `
$wsusServer = '192.168.0.2'
$testConnection = Test-NetConnection -ComputerName $wsusServer -Port 8530
if ($testConnection.TcpTestSucceeded) {
  Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows\WindowsUpdate' -Name WUServer -Value "http://${wsusServer}:8530" -Force
  Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows\WindowsUpdate' -Name WUStatusServer -Value "http://${wsusServer}:8530" -Force
  Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows\WindowsUpdate' -Name ElevateNonAdmins -Value 0 -Type DWord -Force
  Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows\WindowsUpdate' -Name TargetGroupEnabled -Value 0 -Type DWord -Force
  Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows\WindowsUpdate\AU' -Name UseWUServer -Value 1 -Type DWord -Force
}
`

const (
	rebootSetupDoneMarker = "OSCONFIG_E2E_REBOOT_SETUP_DONE"

	aptRebootStartupScript = `#!/bin/bash
mkdir -p /var/lib/google
if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  apt-get update
  apt-get install -y google-osconfig-agent
}
touch /var/run/reboot-required
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
touch /var/lib/google/osconfig_e2e_reboot_setup_done
echo "OSCONFIG_E2E_""REBOOT_SETUP_DONE"
`

	elRebootStartupScript = `#!/bin/bash
mkdir -p /var/lib/google
if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  sed -i 's/repo_gpgcheck=1/repo_gpgcheck=0/g' /etc/yum.repos.d/google-cloud.repo 2>/dev/null || true
  dnf install -y --nogpgcheck google-osconfig-agent || yum install -y --nogpgcheck google-osconfig-agent
}
sleep 2
dnf reinstall -y --nogpgcheck gnutls || yum reinstall -y --nogpgcheck gnutls
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
touch /var/lib/google/osconfig_e2e_reboot_setup_done
echo "OSCONFIG_E2E_""REBOOT_SETUP_DONE"
`

	suseRebootStartupScript = `#!/bin/bash
mkdir -p /var/lib/google
if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  zypper --no-refresh -n -i --no-gpg-checks install google-osconfig-agent || zypper -n -i --no-gpg-checks install google-osconfig-agent
}
sleep 2
zypper -n --no-gpg-checks in -f dbus-1
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
touch /var/lib/google/osconfig_e2e_reboot_setup_done
echo "OSCONFIG_E2E_""REBOOT_SETUP_DONE"
`

	windowsRebootStartupScript = `
New-Item -ItemType Directory -Force -Path 'C:\ProgramData\Google' | Out-Null
$setupDone = 'C:\ProgramData\Google\osconfig_e2e_reboot_setup_done'
if (Test-Path $setupDone) {
    Remove-Item -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired' -Force -ErrorAction SilentlyContinue
    Remove-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager' -Name 'PendingFileRenameOperations' -Force -ErrorAction SilentlyContinue
    exit 0
}

New-Item -ItemType File -Force -Path 'C:\Windows\Temp\osconfig_reboot_test.tmp' | Out-Null
Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager' -Name 'PendingFileRenameOperations' -Type MultiString -Value @("\??\C:\Windows\Temp\osconfig_reboot_test.tmp", "") -Force

$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -ne 'Running') {
    Start-Service google_osconfig_agent -ErrorAction SilentlyContinue
}
Set-Content -Path $setupDone -Value "done" -Force
Write-Output ("OSCONFIG_E2E_" + "REBOOT_SETUP_DONE")
`
)

type patchJobTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
	meta        map[string]string
}

var (
	windowsExecutionMeta = map[string]string{
		"sysprep-specialize-script-ps1": windowsSetWsus,
	}

	aptRebootMeta = map[string]string{
		gcp.MetadataKeyLinuxStartupScript: aptRebootStartupScript,
	}
	elRebootMeta = map[string]string{
		gcp.MetadataKeyLinuxStartupScript: elRebootStartupScript,
	}
	suseRebootMeta = map[string]string{
		gcp.MetadataKeyLinuxStartupScript: suseRebootStartupScript,
	}
	windowsRebootMeta = map[string]string{
		"sysprep-specialize-script-ps1":     windowsSetWsus,
		gcp.MetadataKeyWindowsStartupScript: windowsRebootStartupScript,
	}

	executionTestCases = []patchJobTestCase{
		// Debian
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
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
			meta:        windowsExecutionMeta,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        windowsExecutionMeta,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        windowsExecutionMeta,
		},
	}

	rebootTestCases = []patchJobTestCase{
		// Debian
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
			machineType: "e2-standard-2",
			timeout:     40 * time.Minute,
			meta:        aptRebootMeta,
		},
		// Ubuntu
		{
			name:        "ubuntu-2204-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			machineType: "e2-standard-2",
			timeout:     40 * time.Minute,
			meta:        aptRebootMeta,
		},
		{
			name:        "ubuntu-2404-lts",
			image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			machineType: "e2-standard-2",
			timeout:     40 * time.Minute,
			meta:        aptRebootMeta,
		},
		// EL (RHEL, CentOS, Rocky)
		{
			name:        "rhel-8",
			image:       "projects/rhel-cloud/global/images/family/rhel-8",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        elRebootMeta,
		},
		{
			name:        "rhel-9",
			image:       "projects/rhel-cloud/global/images/family/rhel-9",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        elRebootMeta,
		},
		{
			name:        "centos-stream-9",
			image:       "projects/centos-cloud/global/images/family/centos-stream-9",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        elRebootMeta,
		},
		{
			name:        "rocky-linux-8",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        elRebootMeta,
		},
		{
			name:        "rocky-linux-9",
			image:       "projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			meta:        elRebootMeta,
		},
		// SUSE
		{
			name:        "sles-12",
			image:       "projects/suse-cloud/global/images/family/sles-12",
			machineType: "e2-standard-2",
			timeout:     40 * time.Minute,
			meta:        suseRebootMeta,
		},
		{
			name:        "sles-15",
			image:       "projects/suse-cloud/global/images/family/sles-15",
			machineType: "e2-standard-2",
			timeout:     40 * time.Minute,
			meta:        suseRebootMeta,
		},
		{
			name:        "opensuse-leap-15",
			image:       "projects/opensuse-cloud/global/images/family/opensuse-leap",
			machineType: "e2-standard-2",
			timeout:     40 * time.Minute,
			meta:        suseRebootMeta,
		},
		// Windows
		{
			name:        "windows-2016",
			image:       "projects/windows-cloud/global/images/family/windows-2016",
			machineType: "e2-standard-4",
			timeout:     60 * time.Minute,
			meta:        windowsRebootMeta,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     50 * time.Minute,
			meta:        windowsRebootMeta,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     50 * time.Minute,
			meta:        windowsRebootMeta,
		},
	}
)

// TestOSPatchJobExecution verifies basic OS Patch Job execution across supported
// OS images using the default PatchConfig.
// This migrates the "[Execute PatchJob]" tests from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchJobExecution(t *testing.T) {
	for _, tc := range executionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					// Enable both tasks (required for PatchJob execution) and osinventory (required for WaitForInventory readiness check).
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.meta {
					meta[k] = v
				}
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, meta)
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
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
		})
	}
}

// TestOSPatchJobDoesNotReboot verifies that RebootConfig=NEVER prevents rebooting
// even when a reboot is required, reporting SucceededRebootRequiredInstanceCount > 0.
// This migrates "[PatchJob does not reboot]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchJobDoesNotReboot(t *testing.T) {
	for _, tc := range rebootTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.meta {
					meta[k] = v
				}
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, meta)
				return err
			})

			test.Step("wait for reboot setup and OS Config agent ready", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, rebootSetupDoneMarker, 1); err != nil {
					return err
				}
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("patch job with RebootConfig NEVER does not reboot", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e patch job NEVER reboot test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "NEVER",
						Apt:          &osconfig.AptSettings{Type: "DIST"},
					},
					Duration: fmt.Sprintf("%ds", int(tc.timeout.Seconds())),
				}
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededRebootRequiredInstanceCount == 0 {
					return fmt.Errorf("expected SucceededRebootRequiredInstanceCount > 0 for RebootConfig=NEVER, got summary: %+v", pj.InstanceDetailsSummary)
				}

				agentStarts, err := test.SerialOutputRegexCount(vm, testenv.AgentStartedRegex)
				if err != nil {
					return fmt.Errorf("read agent start count from serial output after RebootConfig=NEVER: %w", err)
				}
				if agentStarts > 1 {
					return fmt.Errorf("instance should not have rebooted with RebootConfig=NEVER, found %d agent starts in serial output", agentStarts)
				}
				return nil
			})
		})
	}
}

// TestOSPatchJobTriggersReboot verifies that RebootConfig=DEFAULT triggers a VM reboot
// when a reboot is required, reporting SucceededInstanceCount > 0 and SucceededRebootRequiredInstanceCount == 0.
// This migrates "[PatchJob triggers reboot]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchJobTriggersReboot(t *testing.T) {
	for _, tc := range rebootTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.meta {
					meta[k] = v
				}
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, meta)
				return err
			})

			test.Step("wait for reboot setup and OS Config agent ready", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, rebootSetupDoneMarker, 1); err != nil {
					return err
				}
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("patch job with RebootConfig DEFAULT triggers reboot", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e patch job DEFAULT reboot test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt:          &osconfig.AptSettings{Type: "DIST"},
					},
					Duration: fmt.Sprintf("%ds", int(tc.timeout.Seconds())),
				}
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededRebootRequiredInstanceCount > 0 {
					return fmt.Errorf("expected SucceededRebootRequiredInstanceCount == 0 after RebootConfig=DEFAULT reboot, got summary: %+v", pj.InstanceDetailsSummary)
				}

				if _, err := test.WaitForSerialOutputRegexCount(vm, testenv.AgentStartedRegex, 2); err != nil {
					return fmt.Errorf("wait for agent restart in serial output after RebootConfig=DEFAULT: %w", err)
				}
				return nil
			})
		})
	}
}
