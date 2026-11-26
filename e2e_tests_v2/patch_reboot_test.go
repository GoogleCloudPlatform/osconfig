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

const (
	rebootConfigAlways  = "ALWAYS"
	rebootConfigDefault = "DEFAULT"
	rebootConfigNever   = "NEVER"

	bootRecord1Marker     = "OSCONFIG_E2E_BOOT_COUNT_1"
	bootRecord2Marker     = "OSCONFIG_E2E_BOOT_COUNT_2"
	rebootSetupDoneMarker = "OSCONFIG_E2E_REBOOT_SETUP_DONE"

	// Non-existent package/patch names passed in exclusive filters so no updates
	// are installed and no update triggers a reboot requirement.
	noOpLinuxPackage                = "osconfig-e2e-nonexistent-pkg"
	noOpLinuxPatch                  = "osconfig-e2e-nonexistent-patch"
	noOpWindowsKB                   = "9999999"
	linuxAlwaysRebootPostStepPath   = "/var/lib/google/osconfig_e2e_post_step.sh"
	windowsAlwaysRebootPostStepPath = `C:\ProgramData\Google\osconfig_e2e_post_step.ps1`
	postStepSuccessCode             = 42

	aptAgentInstallCmd = `for i in $(seq 1 30); do
    (apt-get update && apt-get install -y google-osconfig-agent) && break
    sleep 5
  done`

	elAgentInstallCmd = `sed -i 's/repo_gpgcheck=1/repo_gpgcheck=0/g' /etc/yum.repos.d/google-cloud.repo 2>/dev/null || true
  for i in $(seq 1 30); do
    (dnf install -y --nogpgcheck google-osconfig-agent || yum install -y --nogpgcheck google-osconfig-agent) && break
    sleep 5
  done`

	suseAgentInstallCmd = `for i in $(seq 1 30); do
    (zypper --no-refresh -n -i --no-gpg-checks install google-osconfig-agent || zypper -n -i --no-gpg-checks install google-osconfig-agent) && break
    sleep 5
  done`

	aptRebootStartupScript = `#!/bin/bash
mkdir -p /var/lib/google
boot_count=0
if [ -f /var/lib/google/osconfig_e2e_boot_count ]; then
  boot_count=$(cat /var/lib/google/osconfig_e2e_boot_count)
fi
boot_count=$((boot_count + 1))
echo "$boot_count" > /var/lib/google/osconfig_e2e_boot_count
echo "OSCONFIG_E2E_BOOT_COUNT_""$boot_count"

if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  rm -f /var/run/reboot-required
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  for i in $(seq 1 30); do
    (apt-get update && apt-get install -y google-osconfig-agent) && break
    sleep 5
  done
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
boot_count=0
if [ -f /var/lib/google/osconfig_e2e_boot_count ]; then
  boot_count=$(cat /var/lib/google/osconfig_e2e_boot_count)
fi
boot_count=$((boot_count + 1))
echo "$boot_count" > /var/lib/google/osconfig_e2e_boot_count
echo "OSCONFIG_E2E_BOOT_COUNT_""$boot_count"

if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  sed -i 's/repo_gpgcheck=1/repo_gpgcheck=0/g' /etc/yum.repos.d/google-cloud.repo 2>/dev/null || true
  for i in $(seq 1 30); do
    (dnf install -y --nogpgcheck google-osconfig-agent || yum install -y --nogpgcheck google-osconfig-agent) && break
    sleep 5
  done
}
sleep 2
for i in $(seq 1 30); do
  (dnf reinstall -y --nogpgcheck gnutls || yum reinstall -y --nogpgcheck gnutls) && break
  sleep 5
done
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
touch /var/lib/google/osconfig_e2e_reboot_setup_done
echo "OSCONFIG_E2E_""REBOOT_SETUP_DONE"
`

	suseRebootStartupScript = `#!/bin/bash
mkdir -p /var/lib/google
boot_count=0
if [ -f /var/lib/google/osconfig_e2e_boot_count ]; then
  boot_count=$(cat /var/lib/google/osconfig_e2e_boot_count)
fi
boot_count=$((boot_count + 1))
echo "$boot_count" > /var/lib/google/osconfig_e2e_boot_count
echo "OSCONFIG_E2E_BOOT_COUNT_""$boot_count"

if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  for i in $(seq 1 30); do
    (zypper --no-refresh -n -i --no-gpg-checks install google-osconfig-agent || zypper -n -i --no-gpg-checks install google-osconfig-agent) && break
    sleep 5
  done
}
sleep 2
for i in $(seq 1 30); do
  zypper -n --no-gpg-checks in -f dbus-1 && break
  sleep 5
done
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
touch /var/lib/google/osconfig_e2e_reboot_setup_done
echo "OSCONFIG_E2E_""REBOOT_SETUP_DONE"
`

	windowsRebootStartupScript = `
New-Item -ItemType Directory -Force -Path 'C:\ProgramData\Google' | Out-Null
$bootFile = 'C:\ProgramData\Google\osconfig_e2e_boot_count'
$bootCount = 0
if (Test-Path $bootFile) {
    $bootCount = [int](Get-Content $bootFile -Raw)
}
$bootCount++
Set-Content -Path $bootFile -Value "$bootCount" -Force
Write-Output ("OSCONFIG_E2E_BOOT_COUNT_" + "$bootCount")

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

	windowsAlwaysRebootStartupScript = `
New-Item -ItemType Directory -Force -Path 'C:\ProgramData\Google' | Out-Null
$bootFile = 'C:\ProgramData\Google\osconfig_e2e_boot_count'
$bootCount = 0
if (Test-Path $bootFile) {
    $bootCount = [int](Get-Content $bootFile -Raw)
}
$bootCount++
Set-Content -Path $bootFile -Value "$bootCount" -Force
Write-Output ("OSCONFIG_E2E_BOOT_COUNT_" + "$bootCount")

$setupDone = 'C:\ProgramData\Google\osconfig_e2e_reboot_setup_done'
if (Test-Path $setupDone) {
    Remove-Item -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired' -Force -ErrorAction SilentlyContinue
    Remove-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager' -Name 'PendingFileRenameOperations' -Force -ErrorAction SilentlyContinue
    exit 0
}

Remove-Item -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired' -Force -ErrorAction SilentlyContinue
Remove-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager' -Name 'PendingFileRenameOperations' -Force -ErrorAction SilentlyContinue

$boot1Ticks = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToFileTimeUtc()
Set-Content -Path 'C:\ProgramData\Google\osconfig_e2e_boot1_ticks' -Value "$boot1Ticks" -Force

$postStepScript = @'
$ErrorActionPreference = 'Stop'
$bootTicksFile = 'C:\ProgramData\Google\osconfig_e2e_boot1_ticks'
if (-not (Test-Path $bootTicksFile)) {
    Write-Error "Missing boot1 ticks file: $bootTicksFile"
    exit 2
}
$boot1Ticks = [int64](Get-Content $bootTicksFile -Raw).Trim()
$currentBootTicks = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToFileTimeUtc()
if ($currentBootTicks -le $boot1Ticks) {
    Write-Error "Post-step ran before reboot completed (boot1Ticks=$boot1Ticks, currentBootTicks=$currentBootTicks)"
    exit 1
}
Write-Output "Post-step ran after reboot (boot1Ticks=$boot1Ticks, currentBootTicks=$currentBootTicks)"
exit 42
'@
Set-Content -Path 'C:\ProgramData\Google\osconfig_e2e_post_step.ps1' -Value $postStepScript -Force

$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -ne 'Running') {
    Start-Service google_osconfig_agent -ErrorAction SilentlyContinue
}
Set-Content -Path $setupDone -Value "done" -Force
Write-Output ("OSCONFIG_E2E_" + "REBOOT_SETUP_DONE")
`
)

var (
	aptRebootMetadata = map[string]string{
		gcp.MetadataKeyLinuxStartupScript: aptRebootStartupScript,
	}
	elRebootMetadata = map[string]string{
		gcp.MetadataKeyLinuxStartupScript: elRebootStartupScript,
	}
	suseRebootMetadata = map[string]string{
		gcp.MetadataKeyLinuxStartupScript: suseRebootStartupScript,
	}
	windowsRebootMetadata = map[string]string{
		"sysprep-specialize-script-ps1":     windowsSetWsus,
		gcp.MetadataKeyWindowsStartupScript: windowsRebootStartupScript,
	}

	rebootTargets = gcp.TargetsWithFamilyMetadata(gcp.FamilyMetadata{
		Apt:     aptRebootMetadata,
		Yum:     elRebootMetadata,
		Zypper:  suseRebootMetadata,
		Windows: windowsRebootMetadata,
	})

	alwaysRebootTargets = gcp.TargetsWithFamilyMetadata(gcp.FamilyMetadata{
		Apt: map[string]string{
			gcp.MetadataKeyLinuxStartupScript: linuxAlwaysRebootStartupScript(aptAgentInstallCmd),
		},
		Yum: map[string]string{
			gcp.MetadataKeyLinuxStartupScript: linuxAlwaysRebootStartupScript(elAgentInstallCmd),
		},
		Zypper: map[string]string{
			gcp.MetadataKeyLinuxStartupScript: linuxAlwaysRebootStartupScript(suseAgentInstallCmd),
		},
		Windows: map[string]string{
			"sysprep-specialize-script-ps1":     windowsSetWsus,
			gcp.MetadataKeyWindowsStartupScript: windowsAlwaysRebootStartupScript,
		},
	})
)

func linuxAlwaysRebootStartupScript(agentInstallCmd string) string {
	return fmt.Sprintf(`#!/bin/bash
mkdir -p /var/lib/google
boot_count=0
if [ -f /var/lib/google/osconfig_e2e_boot_count ]; then
  boot_count=$(cat /var/lib/google/osconfig_e2e_boot_count)
fi
boot_count=$((boot_count + 1))
echo "$boot_count" > /var/lib/google/osconfig_e2e_boot_count
echo "OSCONFIG_E2E_BOOT_COUNT_""$boot_count"

if [ -f /var/lib/google/osconfig_e2e_reboot_setup_done ]; then
  rm -f /var/run/reboot-required
  exit 0
fi

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  %s
}
rm -f /var/run/reboot-required
cat /proc/sys/kernel/random/boot_id > /var/lib/google/osconfig_e2e_boot1_id

cat << 'EOF' > /var/lib/google/osconfig_e2e_post_step.sh
#!/bin/sh
set -eu
if [ ! -f /var/lib/google/osconfig_e2e_boot1_id ]; then
  echo "Missing boot1 id file" >&2
  exit 2
fi
boot1_id=$(cat /var/lib/google/osconfig_e2e_boot1_id)
current_boot_id=$(cat /proc/sys/kernel/random/boot_id)
if [ "$current_boot_id" = "$boot1_id" ]; then
  echo "Post-step ran before reboot completed (boot_id=$current_boot_id)" >&2
  exit 1
fi
echo "Post-step ran after reboot (boot1=$boot1_id, current=$current_boot_id)"
exit 42
EOF
chmod +x /var/lib/google/osconfig_e2e_post_step.sh

systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
touch /var/lib/google/osconfig_e2e_reboot_setup_done
echo "OSCONFIG_E2E_""REBOOT_SETUP_DONE"
`, agentInstallCmd)
}

func generateRebootPatchJobRequest(desc string, vm *gcp.VM, rebootConfig string, timeout time.Duration) *osconfig.ExecutePatchJobRequest {
	return generateExecutePatchJobRequest(desc, vm, &osconfig.PatchConfig{
		RebootConfig: rebootConfig,
		Apt:          &osconfig.AptSettings{Type: "DIST"},
	}, timeout)
}

func generateAlwaysRebootPatchJobRequest(desc string, vm *gcp.VM, timeout time.Duration) *osconfig.ExecutePatchJobRequest {
	return generateExecutePatchJobRequest(desc, vm, &osconfig.PatchConfig{
		RebootConfig: rebootConfigAlways,
		Apt: &osconfig.AptSettings{
			ExclusivePackages: []string{noOpLinuxPackage},
		},
		Yum: &osconfig.YumSettings{
			ExclusivePackages: []string{noOpLinuxPackage},
		},
		Zypper: &osconfig.ZypperSettings{
			ExclusivePatches: []string{noOpLinuxPatch},
		},
		WindowsUpdate: &osconfig.WindowsUpdateSettings{
			ExclusivePatches: []string{noOpWindowsKB},
		},
		PostStep: &osconfig.ExecStep{
			LinuxExecStepConfig: &osconfig.ExecStepConfig{
				LocalPath:           linuxAlwaysRebootPostStepPath,
				Interpreter:         "SHELL",
				AllowedSuccessCodes: []int64{postStepSuccessCode},
			},
			WindowsExecStepConfig: &osconfig.ExecStepConfig{
				LocalPath:           windowsAlwaysRebootPostStepPath,
				Interpreter:         "POWERSHELL",
				AllowedSuccessCodes: []int64{postStepSuccessCode},
			},
		},
	}, timeout)
}

// TestOSPatchJobDoesNotReboot verifies that RebootConfig=NEVER prevents rebooting
// even when a reboot is required, reporting SucceededRebootRequiredInstanceCount == 1.
// This migrates "[PatchJob does not reboot]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchJobDoesNotReboot(t *testing.T) {
	for _, tc := range rebootTargets {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.Timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.Image, tc.MachineType, generateVmMetadata(tc.ExtraMetadata))
				return err
			})

			test.Step("wait for reboot setup and OS Config agent ready", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, rebootSetupDoneMarker, 1); err != nil {
					return err
				}
				_, err := test.WaitForInventory(vm)
				return err
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with RebootConfig NEVER", func(ctx context.Context) error {
				var err error
				req := generateRebootPatchJobRequest("e2e patch job NEVER reboot test", vm, rebootConfigNever, tc.Timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededRebootRequiredInstanceCount != 1 || pj.InstanceDetailsSummary.SucceededInstanceCount != 0 {
					return fmt.Errorf("expected SucceededRebootRequiredInstanceCount == 1 and SucceededInstanceCount == 0 for RebootConfig=NEVER, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify VM did not reboot", func(ctx context.Context) error {
				boot2Count, err := test.SerialOutputCount(vm, bootRecord2Marker)
				if err != nil {
					return fmt.Errorf("read boot 2 count from serial output after RebootConfig=NEVER: %w", err)
				}
				if boot2Count > 0 {
					return fmt.Errorf("instance should not have rebooted with RebootConfig=NEVER, found %d boot 2 records in serial output", boot2Count)
				}
				boot1Count, err := test.SerialOutputCount(vm, bootRecord1Marker)
				if err != nil {
					return fmt.Errorf("read boot 1 count from serial output after RebootConfig=NEVER: %w", err)
				}
				if boot1Count == 0 {
					return fmt.Errorf("expected at least 1 boot 1 record in serial output, found 0")
				}
				return nil
			})
		})
	}
}

// TestOSPatchJobTriggersReboot verifies that RebootConfig=DEFAULT triggers a VM reboot
// when a reboot is required, reporting SucceededInstanceCount == 1 and SucceededRebootRequiredInstanceCount == 0.
// This migrates "[PatchJob triggers reboot]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchJobTriggersReboot(t *testing.T) {
	for _, tc := range rebootTargets {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.Timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.Image, tc.MachineType, generateVmMetadata(tc.ExtraMetadata))
				return err
			})

			test.Step("wait for reboot setup and OS Config agent ready", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, rebootSetupDoneMarker, 1); err != nil {
					return err
				}
				_, err := test.WaitForInventory(vm)
				return err
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with RebootConfig DEFAULT", func(ctx context.Context) error {
				var err error
				req := generateRebootPatchJobRequest("e2e patch job DEFAULT reboot test", vm, rebootConfigDefault, tc.Timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job completion", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 || pj.InstanceDetailsSummary.SucceededRebootRequiredInstanceCount != 0 {
					return fmt.Errorf("expected SucceededInstanceCount == 1 and SucceededRebootRequiredInstanceCount == 0 after RebootConfig=DEFAULT reboot, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify VM rebooted", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, bootRecord2Marker, 1); err != nil {
					return fmt.Errorf("wait for reboot in serial output after RebootConfig=DEFAULT: %w", err)
				}
				return nil
			})
		})
	}
}

// TestOSPatchJobAlwaysReboots verifies that RebootConfig=ALWAYS forces a VM reboot
// even when no updates require a reboot, transitions through the REBOOTING state,
// increments boot count from 1 to 2, and runs the configured post-patch step only after reboot.
func TestOSPatchJobAlwaysReboots(t *testing.T) {
	for _, tc := range alwaysRebootTargets {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.Timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.Image, tc.MachineType, generateVmMetadata(tc.ExtraMetadata))
				return err
			})

			test.Step("wait for reboot setup and OS Config agent ready", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, rebootSetupDoneMarker, 1); err != nil {
					return err
				}
				boot1Count, err := test.SerialOutputCount(vm, bootRecord1Marker)
				if err != nil {
					return fmt.Errorf("read boot 1 count from serial output: %w", err)
				}
				if boot1Count == 0 {
					return fmt.Errorf("expected boot 1 record in serial output before patch job, found 0")
				}
				_, err = test.WaitForInventory(vm)
				return err
			})

			var job *osconfig.PatchJob
			test.Step("execute patch job with RebootConfig ALWAYS and post-step", func(ctx context.Context) error {
				var err error
				req := generateAlwaysRebootPatchJobRequest("e2e patch job ALWAYS reboot test", vm, tc.Timeout)
				job, err = test.ExecutePatchJob(req)
				return err
			})

			test.Step("wait for patch job REBOOTING state", func(ctx context.Context) error {
				return test.WaitForPatchJobRebooting(job.Name)
			})

			test.Step("wait for patch job completion and post-step verification", func(ctx context.Context) error {
				pj, err := test.WaitForPatchJob(job.Name)
				if err != nil {
					return err
				}
				if pj.InstanceDetailsSummary.SucceededInstanceCount != 1 || pj.InstanceDetailsSummary.SucceededRebootRequiredInstanceCount != 0 {
					return fmt.Errorf("expected SucceededInstanceCount == 1 and SucceededRebootRequiredInstanceCount == 0 after RebootConfig=ALWAYS, got summary: %+v", pj.InstanceDetailsSummary)
				}
				return nil
			})

			test.Step("verify VM rebooted from boot 1 to boot 2", func(ctx context.Context) error {
				if _, err := test.WaitForSerialOutputCount(vm, bootRecord2Marker, 1); err != nil {
					return fmt.Errorf("wait for reboot in serial output after RebootConfig=ALWAYS: %w", err)
				}
				return nil
			})
		})
	}
}
