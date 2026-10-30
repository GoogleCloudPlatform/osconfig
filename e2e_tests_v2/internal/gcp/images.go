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

package gcp

import "time"

const (
	// DefaultLinuxTimeout is the default test execution timeout for Linux instances.
	DefaultLinuxTimeout = 45 * time.Minute
	// DefaultWindowsTimeout is the default test execution timeout for Windows instances.
	DefaultWindowsTimeout = 60 * time.Minute

	// Canonical Google Cloud Compute Engine image family paths for OS Config E2E tests.

	// Debian
	ImageDebian12 = "projects/debian-cloud/global/images/family/debian-12"
	ImageDebian13 = "projects/debian-cloud/global/images/family/debian-13"

	// Ubuntu
	ImageUbuntu2204 = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts"
	ImageUbuntu2404 = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64"

	// Enterprise Linux (RHEL, CentOS Stream, Rocky Linux)
	ImageRHEL8         = "projects/rhel-cloud/global/images/family/rhel-8"
	ImageRHEL9         = "projects/rhel-cloud/global/images/family/rhel-9"
	ImageCentOSStream9 = "projects/centos-cloud/global/images/family/centos-stream-9"
	ImageRockyLinux8   = "projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp"
	ImageRockyLinux9   = "projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp"

	// SUSE / openSUSE
	ImageSLES12       = "projects/suse-cloud/global/images/family/sles-12"
	ImageSLES15       = "projects/suse-cloud/global/images/family/sles-15"
	ImageOpenSUSELeap = "projects/opensuse-cloud/global/images/family/opensuse-leap"

	// Windows Server
	ImageWindows2016 = "projects/windows-cloud/global/images/family/windows-2016"
	ImageWindows2019 = "projects/windows-cloud/global/images/family/windows-2019"
	ImageWindows2022 = "projects/windows-cloud/global/images/family/windows-2022"

	// WindowsWSUSSetupScript configures local WSUS settings for Windows test VMs.
	WindowsWSUSSetupScript = `
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
)

var (
	// WindowsPatchMeta contains default Windows metadata required for patch tests.
	WindowsPatchMeta = map[string]string{
		"sysprep-specialize-script-ps1": WindowsWSUSSetupScript,
	}

	// AptTargets contains all Debian / Ubuntu test targets.
	AptTargets = []OSTarget{
		{Name: "debian-12", Image: ImageDebian12, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
		{Name: "debian-13", Image: ImageDebian13, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
		{Name: "ubuntu-2204-lts", Image: ImageUbuntu2204, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
		{Name: "ubuntu-2404-lts", Image: ImageUbuntu2404, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
	}

	// YumTargets contains all Enterprise Linux (RHEL, CentOS Stream, Rocky) test targets.
	YumTargets = []OSTarget{
		{Name: "rhel-8", Image: ImageRHEL8, MachineType: "e2-standard-4", Timeout: DefaultLinuxTimeout},
		{Name: "rhel-9", Image: ImageRHEL9, MachineType: "e2-standard-4", Timeout: DefaultLinuxTimeout},
		{Name: "centos-stream-9", Image: ImageCentOSStream9, MachineType: "e2-standard-4", Timeout: DefaultLinuxTimeout},
		{Name: "rocky-linux-8", Image: ImageRockyLinux8, MachineType: "e2-standard-4", Timeout: DefaultLinuxTimeout},
		{Name: "rocky-linux-9", Image: ImageRockyLinux9, MachineType: "e2-standard-4", Timeout: DefaultLinuxTimeout},
	}

	// ZypperTargets contains all SUSE / openSUSE test targets.
	ZypperTargets = []OSTarget{
		{Name: "sles-12", Image: ImageSLES12, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
		{Name: "sles-15", Image: ImageSLES15, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
		{Name: "opensuse-leap-15", Image: ImageOpenSUSELeap, MachineType: "e2-standard-2", Timeout: DefaultLinuxTimeout},
	}

	// WindowsTargets contains all Windows Server test targets.
	WindowsTargets = []OSTarget{
		{Name: "windows-2016", Image: ImageWindows2016, MachineType: "e2-standard-4", Timeout: DefaultWindowsTimeout, ExtraMeta: WindowsPatchMeta},
		{Name: "windows-2019", Image: ImageWindows2019, MachineType: "e2-standard-4", Timeout: DefaultWindowsTimeout, ExtraMeta: WindowsPatchMeta},
		{Name: "windows-2022", Image: ImageWindows2022, MachineType: "e2-standard-4", Timeout: DefaultWindowsTimeout, ExtraMeta: WindowsPatchMeta},
	}

	// AllTargets contains all supported OS targets across all families.
	AllTargets = ConcatOSTargets(AptTargets, YumTargets, ZypperTargets, WindowsTargets)
)

// OSTarget represents a standard VM image target for E2E tests.
type OSTarget struct {
	Name        string
	Image       string
	MachineType string
	Timeout     time.Duration
	ExtraMeta   map[string]string
}

// WithMeta returns a copy of a single OSTarget with additional metadata merged in.
func (t OSTarget) WithMeta(meta map[string]string) OSTarget {
	merged := make(map[string]string, len(t.ExtraMeta)+len(meta))
	for k, v := range t.ExtraMeta {
		merged[k] = v
	}
	for k, v := range meta {
		merged[k] = v
	}
	t.ExtraMeta = merged
	return t
}

// WithTargetsMeta accepts a slice of OSTarget and returns a new slice with additional metadata merged into each target.
func WithTargetsMeta(targets []OSTarget, meta map[string]string) []OSTarget {
	res := make([]OSTarget, len(targets))
	for i, t := range targets {
		res[i] = t.WithMeta(meta)
	}
	return res
}

// FamilyMeta defines metadata to be applied to each OS family.
type FamilyMeta struct {
	Apt     map[string]string
	Yum     map[string]string
	Zypper  map[string]string
	Windows map[string]string
}

// ConcatOSTargets concatenates multiple slices of OSTarget into a single slice.
func ConcatOSTargets(groups ...[]OSTarget) []OSTarget {
	var total []OSTarget
	for _, g := range groups {
		total = append(total, g...)
	}
	return total
}

// WithFamilyMeta applies family-specific metadata to all OS target groups.
func WithFamilyMeta(meta FamilyMeta) []OSTarget {
	return ConcatOSTargets(
		WithTargetsMeta(AptTargets, meta.Apt),
		WithTargetsMeta(YumTargets, meta.Yum),
		WithTargetsMeta(ZypperTargets, meta.Zypper),
		WithTargetsMeta(WindowsTargets, meta.Windows),
	)
}
