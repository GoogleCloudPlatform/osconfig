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

import (
	"fmt"
	"strings"
)

const (
	// MetadataKeyLinuxStartupScript is the metadata key for Linux startup scripts.
	MetadataKeyLinuxStartupScript = "startup-script"
	// MetadataKeyWindowsStartupScript is the metadata key for Windows PowerShell startup scripts.
	MetadataKeyWindowsStartupScript = "windows-startup-script-ps1"
	// MetadataKeyRestartAgent is the instance metadata key used to signal the startup script to restart the agent.
	MetadataKeyRestartAgent = "restart-agent"
	// GuestAttributeInstallDone is written when VM prerequisites and agent setup are complete.
	GuestAttributeInstallDone = "osconfig_tests/install_done"
	// GuestAttributeRecipeInstalled is written when recipe verification succeeds.
	GuestAttributeRecipeInstalled = "osconfig_tests/pkg_installed"
	// GuestAttributeRecipeNotInstalled is written when recipe verification has not yet succeeded.
	GuestAttributeRecipeNotInstalled = "osconfig_tests/pkg_not_installed"
)

// DebianStartupScript returns the agent bootstrap script for Debian and Ubuntu systems.
func DebianStartupScript() string {
	return `#!/bin/bash
which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  apt-get update
  apt-get install -y google-osconfig-agent
}
uri="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated"
curl -X DELETE "$uri" -H "Metadata-Flavor: Google" || true
systemctl daemon-reload
systemctl enable --now google-osconfig-agent || true
systemctl restart google-osconfig-agent || true
`
}

// RHELStartupScript returns the agent bootstrap script for RHEL, CentOS, Rocky, and Fedora systems.
func RHELStartupScript() string {
	return `#!/bin/bash
which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  sed -i 's/repo_gpgcheck=1/repo_gpgcheck=0/g' /etc/yum.repos.d/google-cloud.repo 2>/dev/null || true
  dnf install -y --nogpgcheck google-osconfig-agent || yum install -y --nogpgcheck google-osconfig-agent
}
uri="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated"
curl -X DELETE "$uri" -H "Metadata-Flavor: Google" || true
systemctl daemon-reload
systemctl enable --now google-osconfig-agent || true
systemctl restart google-osconfig-agent || true
`
}

// SUSEStartupScript returns the agent bootstrap script for SLES and openSUSE systems.
func SUSEStartupScript() string {
	return `#!/bin/bash
which google_osconfig_agent >/dev/null 2>&1 || which google-osconfig-agent >/dev/null 2>&1 || {
  zypper --no-refresh -n -i --no-gpg-checks install google-osconfig-agent || zypper -n -i --no-gpg-checks install google-osconfig-agent
}
uri="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated"
curl -X DELETE "$uri" -H "Metadata-Flavor: Google" || true
systemctl daemon-reload
systemctl enable --now google-osconfig-agent || true
systemctl restart google-osconfig-agent || true
`
}

// COSStartupScript returns the agent bootstrap script for Container-Optimized OS.
func COSStartupScript() string {
	return `#!/bin/bash
uri="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated"
curl -X DELETE "$uri" -H "Metadata-Flavor: Google" || true
systemctl restart google-osconfig-agent || true
`
}

// WindowsStartupScript returns the PowerShell agent bootstrap script for Windows systems.
func WindowsStartupScript() string {
	return `$uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated'
Invoke-RestMethod -Method DELETE -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -ErrorAction SilentlyContinue
$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Restart-Service google_osconfig_agent -ErrorAction SilentlyContinue
} else {
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

const linuxWaitForRestartScript = `
uri_done="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/osconfig_tests/install_done"
curl -X PUT --data "1" "$uri_done" -H "Metadata-Flavor: Google" || true

echo 'Waiting for signal to restart agent'
while [[ -z $restarted ]]; do
  sleep 1
  restart=$(curl -f -s "http://metadata.google.internal/computeMetadata/v1/instance/attributes/restart-agent" -H "Metadata-Flavor: Google")
  if [[ -n $restart ]]; then
    systemctl restart google-osconfig-agent
    restarted=true
    sleep 30
  fi
done
`

const windowsWaitForRestartScript = `
Start-Sleep 10
$uri_done = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/osconfig_tests/install_done'
Invoke-RestMethod -Method PUT -Uri $uri_done -Headers @{"Metadata-Flavor" = "Google"} -Body 1 -ErrorAction SilentlyContinue

echo 'Waiting for signal to restart agent'
while (! $restarted) {
  sleep 1
  $restart = Invoke-WebRequest -UseBasicParsing http://metadata.google.internal/computeMetadata/v1/instance/attributes/restart-agent -Headers @{"Metadata-Flavor"="Google"} -ErrorAction SilentlyContinue
  if ($restart) {
    Restart-Service google_osconfig_agent -ErrorAction SilentlyContinue
    $restarted = $true
    sleep 30
  }
}
`

func linuxRecipeDBLoopScript(recipeName string) string {
	return fmt.Sprintf(`
while true; do
  is_installed=$(grep '{"Name":"%[1]s","Version":\[0],"InstallTime":[0-9]*,"Success":true}' /var/lib/google/osconfig_recipedb 2>/dev/null)
  if [[ -n $is_installed ]]; then
    uri="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[2]s"
  else
    uri="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s"
  fi
  curl -X PUT --data "1" "$uri" -H "Metadata-Flavor: Google" || true
  sleep 5
done
`, recipeName, GuestAttributeRecipeInstalled, GuestAttributeRecipeNotInstalled)
}

func windowsRecipeDBLoopScript(recipeName string) string {
	return fmt.Sprintf(`
while ($true) {
  $is_installed = (Get-Content 'C:\ProgramData\Google\osconfig_recipedb' -ErrorAction SilentlyContinue | Select-String '{"Name":"%[1]s","Version":\[0],"InstallTime":[0-9]+,"Success":true}')
  if ($is_installed) {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[2]s'
  } else {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s'
  }
  Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1 -ErrorAction SilentlyContinue
  sleep 5
}
`, recipeName, GuestAttributeRecipeInstalled, GuestAttributeRecipeNotInstalled)
}

// RecipeInstallStartupScript returns the metadata key and startup script for verifying basic SoftwareRecipe installation.
func RecipeInstallStartupScript(image, recipeName string) (key, content string) {
	baseKey, baseScript := DefaultStartupScript(image)
	if baseKey == MetadataKeyWindowsStartupScript {
		return baseKey, baseScript + windowsWaitForRestartScript + windowsRecipeDBLoopScript(recipeName)
	}
	return baseKey, baseScript + linuxWaitForRestartScript + linuxRecipeDBLoopScript(recipeName)
}

// RecipeStepsStartupScript returns the metadata key and startup script for verifying SoftwareRecipe step execution.
func RecipeStepsStartupScript(image, recipeName string) (key, content string) {
	img := strings.ToLower(image)
	baseKey, baseScript := DefaultStartupScript(image)

	if baseKey == MetadataKeyWindowsStartupScript {
		stepsCheck := `
while ( ! (Test-Path c:\osconfig-SoftwareRecipe_Step_RunScript_SHELL) ) {
  sleep 1
}
while ( ! (Test-Path c:\osconfig-SoftwareRecipe_Step_RunScript_POWERSHELL) ) {
  sleep 1
}
while ( ! (Test-Path c:\osconfig-exec-test) ) {
  sleep 1
}
while ( ! (Test-Path c:\osconfig-copy-test) ) {
  sleep 1
}
while ( ! (Test-Path c:\tar-test\tar\test.txt) ) {
  sleep 1
}
`
		return baseKey, baseScript + windowsWaitForRestartScript + stepsCheck + windowsRecipeDBLoopScript(recipeName)
	}

	var prereq string
	switch {
	case strings.Contains(img, "rhel") || strings.Contains(img, "centos") || strings.Contains(img, "rocky") || strings.Contains(img, "fedora"):
		prereq = "\ndnf install -y info || yum install -y info\n"
	case strings.Contains(img, "suse") || strings.Contains(img, "sles") || strings.Contains(img, "opensuse"):
		prereq = "\nzypper -n remove ed || true\n"
	}

	stepsCheck := `
while [[ ! -f /tmp/osconfig-SoftwareRecipe_Step_RunScript_SHELL ]]; do
  sleep 1
done
while [[ ! -f /tmp/osconfig-copy-test ]]; do
  sleep 1
done
while [[ ! -f /tmp/tar-test/tar/test.txt ]]; do
  sleep 1
done
while [[ ! -f /tmp/zip-test/zip/test.txt ]]; do
  sleep 1
done
`
	if !strings.Contains(img, "cos") {
		stepsCheck += `
while [[ ! -f /tmp/osconfig-SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED ]]; do
  sleep 1
done
while [[ ! -f /tmp/osconfig-exec-test ]]; do
  sleep 1
done
`
	}

	return baseKey, prereq + baseScript + linuxWaitForRestartScript + stepsCheck + linuxRecipeDBLoopScript(recipeName)
}
