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

import "strings"

const (
	// MetadataKeyLinuxStartupScript is the metadata key for Linux startup scripts.
	MetadataKeyLinuxStartupScript = "startup-script"
	// MetadataKeyWindowsStartupScript is the metadata key for Windows PowerShell startup scripts.
	MetadataKeyWindowsStartupScript = "windows-startup-script-ps1"
)

// DebianStartupScript returns the agent bootstrap script for Debian and Ubuntu systems.
func DebianStartupScript() string {
	return `#!/bin/bash
if [ -f /var/lib/google/osconfig_e2e_bootstrapped ]; then
  exit 0
fi
mkdir -p /var/lib/google
touch /var/lib/google/osconfig_e2e_bootstrapped

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  apt-get update
  apt-get install -y google-osconfig-agent
}
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
`
}

// RHELStartupScript returns the agent bootstrap script for RHEL, CentOS, Rocky, and Fedora systems.
func RHELStartupScript() string {
	return `#!/bin/bash
if [ -f /var/lib/google/osconfig_e2e_bootstrapped ]; then
  exit 0
fi
mkdir -p /var/lib/google
touch /var/lib/google/osconfig_e2e_bootstrapped

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  sed -i 's/repo_gpgcheck=1/repo_gpgcheck=0/g' /etc/yum.repos.d/google-cloud.repo 2>/dev/null || true
  dnf install -y --nogpgcheck google-osconfig-agent || yum install -y --nogpgcheck google-osconfig-agent
}
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
`
}

// SUSEStartupScript returns the agent bootstrap script for SLES and openSUSE systems.
func SUSEStartupScript() string {
	return `#!/bin/bash
if [ -f /var/lib/google/osconfig_e2e_bootstrapped ]; then
  exit 0
fi
mkdir -p /var/lib/google
touch /var/lib/google/osconfig_e2e_bootstrapped

which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  zypper --no-refresh -n -i --no-gpg-checks install google-osconfig-agent || zypper -n -i --no-gpg-checks install google-osconfig-agent
}
systemctl daemon-reload
if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl enable --now google-osconfig-agent || true
fi
`
}

// COSStartupScript returns the agent bootstrap script for Container-Optimized OS.
func COSStartupScript() string {
	return `#!/bin/bash
if [ -f /var/lib/google/osconfig_e2e_bootstrapped ]; then
  exit 0
fi
mkdir -p /var/lib/google
touch /var/lib/google/osconfig_e2e_bootstrapped

if ! systemctl is-active --quiet google-osconfig-agent; then
  systemctl start google-osconfig-agent || true
fi
`
}

// WindowsStartupScript returns the PowerShell agent bootstrap script for Windows systems.
func WindowsStartupScript() string {
	return `$marker = 'C:\ProgramData\Google\osconfig_e2e_bootstrapped'
if (Test-Path $marker) {
    exit 0
}
New-Item -ItemType Directory -Force -Path 'C:\ProgramData\Google' | Out-Null
New-Item -ItemType File -Force -Path $marker | Out-Null

$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -ne 'Running') {
    Start-Service google_osconfig_agent -ErrorAction SilentlyContinue
}
`
}

// DefaultStartupScript determines the metadata key and startup script content based on the VM image path.
func DefaultStartupScript(image string) (key, content string) {
	img := strings.ToLower(image)
	switch {
	case strings.Contains(img, "windows"):
		return MetadataKeyWindowsStartupScript, WindowsStartupScript()
	case strings.Contains(img, "cos"):
		return MetadataKeyLinuxStartupScript, COSStartupScript()
	case strings.Contains(img, "suse") || strings.Contains(img, "sles") || strings.Contains(img, "opensuse"):
		return MetadataKeyLinuxStartupScript, SUSEStartupScript()
	case strings.Contains(img, "rhel") || strings.Contains(img, "centos") || strings.Contains(img, "rocky") || strings.Contains(img, "fedora"):
		return MetadataKeyLinuxStartupScript, RHELStartupScript()
	default:
		return MetadataKeyLinuxStartupScript, DebianStartupScript()
	}
}
