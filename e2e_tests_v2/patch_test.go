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

type patchJobTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
	extraMeta   map[string]string
}

// TestOSPatchJobExecution verifies basic OS Patch Job execution across supported
// OS images using the default PatchConfig.
// This migrates the "[Execute PatchJob]" tests from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchJobExecution(t *testing.T) {
	windowsMeta := map[string]string{
		"sysprep-specialize-script-ps1": windowsSetWsus,
	}

	testCases := []patchJobTestCase{
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
			extraMeta:   windowsMeta,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			extraMeta:   windowsMeta,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     45 * time.Minute,
			extraMeta:   windowsMeta,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					// Enable both tasks (required for PatchJob execution) and osinventory (required for WaitForInventory readiness check).
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
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

var aptTestCases = []patchJobTestCase{
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
}

func debianDowngradeStartupScript() string {
	return `#!/bin/bash
if [ -f /var/lib/google/osconfig_e2e_downgrade_bootstrapped ]; then
  exit 0
fi
mkdir -p /var/lib/google
touch /var/lib/google/osconfig_e2e_downgrade_bootstrapped

systemctl stop google-osconfig-agent 2>/dev/null || true

# Remove backports and security repos which were archived and return 404 on debian-11
sed -i '/bullseye-backports/d' /etc/apt/sources.list /etc/apt/sources.list.d/*.list 2>/dev/null || true
sed -i '/debian-security/d' /etc/apt/sources.list /etc/apt/sources.list.d/*.list 2>/dev/null || true
sed -i '/bullseye-security/d' /etc/apt/sources.list /etc/apt/sources.list.d/*.list 2>/dev/null || true

# Add snapshot repository with older sudo version and pin it
echo 'deb [trusted=yes check-valid-until=no] http://snapshot.debian.org/archive/debian/20190801T025637Z/ buster main' >> /etc/apt/sources.list
echo 'Package: sudo' >> /etc/apt/preferences
echo 'Pin: version 1.8.27-1' >> /etc/apt/preferences
echo 'Pin-priority: 9999' >> /etc/apt/preferences

echo 'Acquire::Retries "3";' > /etc/apt/apt.conf.d/99retries
echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99check-valid-until

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  apt-get update
  apt-get install -y google-osconfig-agent
}

apt-get update || true

systemctl daemon-reload
systemctl enable --now google-osconfig-agent || true
`
}

var aptDowngradeTestCases = []patchJobTestCase{
	{
		name:        "debian-11",
		image:       "projects/debian-cloud/global/images/debian-11-bullseye-v20231010",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
		extraMeta: map[string]string{
			gcp.MetadataKeyLinuxStartupScript: debianDowngradeStartupScript(),
		},
	},
}

// TestOSPatchAptExcludes verifies that APT patching with dist-upgrade and package excludes succeeds.
// This migrates "[APT dist-upgrade, excludes]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchAptExcludes(t *testing.T) {
	for _, tc := range aptTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
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

			test.Step("execute and await patch job with APT excludes", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e apt excludes patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt: &osconfig.AptSettings{
							Type:     "DIST",
							Excludes: []string{"pkg1", "/pkg2/"},
						},
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

// TestOSPatchAptExclusivePackages verifies that APT patching with dist-upgrade and exclusive packages succeeds.
// This migrates "[APT dist-upgrade, exclusive packages]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchAptExclusivePackages(t *testing.T) {
	for _, tc := range aptTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
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

			test.Step("execute and await patch job with APT exclusive packages", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e apt exclusive packages patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt: &osconfig.AptSettings{
							Type:              "DIST",
							ExclusivePackages: []string{"pkg1"},
						},
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

// TestOSPatchAptDowngrade verifies that APT dist-upgrade patching succeeds even when a package needs to be downgraded.
// This migrates "[PatchJob apt-get doesn't fail on downgrades]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchAptDowngrade(t *testing.T) {
	for _, tc := range aptDowngradeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
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

			test.Step("execute and await patch job with APT downgrade", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e apt downgrade patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt: &osconfig.AptSettings{
							Type: "DIST",
						},
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
