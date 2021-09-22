//  Copyright 2021 Google Inc. All Rights Reserved.
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

package ospolicies

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	computeApi "google.golang.org/api/compute/v1"
)

func getStartupScriptPackage(image, pkgManager string) *computeApi.MetadataItems {
	var ss, key string

	switch pkgManager {
	case "apt":
		wantInstall := "ed"
		wantRemove := "vim"
		ss = `set -x
# install the package we want removed
apt-get -y install %[2]s
# remove the package we want installed
apt-get -y remove %[3]s
# install agent
%[1]s
while true; do
  # make sure the package we want installed is installed
  isinstalled=$(/usr/bin/dpkg-query -s %[3]s)
  if [[ $isinstalled =~ "Status: install ok installed" ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    break
  fi
  sleep 10
done
while true; do
  # make sure the package we want removed is removed
  isinstalled=$(/usr/bin/dpkg-query -s %[2]s)
  if ! [[ $isinstalled =~ "Status: install ok installed" ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[5]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    break
  fi
  sleep 10
done`

		ss = fmt.Sprintf(ss, utils.InstallOSConfigDeb(), wantRemove, wantInstall, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "deb":
		wantInstall := []string{"google-chrome-stable", "osconfig-agent-test"}
		ss = `set -x
# install agent
%[1]s
while true; do
  isinstalled=$(/usr/bin/dpkg-query -s %[2]s)
  if [[ $isinstalled =~ "Status: install ok installed" ]]; then
    break
  fi
  sleep 10
done
while true; do
  isinstalled=$(/usr/bin/dpkg-query -s %[3]s)
  if [[ $isinstalled =~ "Status: install ok installed" ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    break
  fi
  sleep 10
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigDeb(), wantInstall[0], wantInstall[1], packageInstalled)
		key = "startup-script"

	case "yum":
		wantInstall := "ed"
		wantRemove := "nano"
		ss = `set -x
# install the package we want removed
yum -y install %[2]s
# remove the package we want installed
yum -y remove %[3]s
%[1]s
while true; do
  # make sure the package we want installed is installed
  if [[ -n $(/usr/bin/rpmquery -a %[3]s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    break
  fi
  sleep 10
done
while true; do
  # make sure the package we want removed is removed
  if [[ -z $(/usr/bin/rpmquery -a %[2]s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[5]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    break
  fi
  sleep 10
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigEL(image), wantRemove, wantInstall, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "rpm":
		wantInstall := []string{"google-chrome-stable", "osconfig-agent-test"}
		ss = `set -x
# install agent
%[1]s
rpm --import https://dl.google.com/linux/linux_signing_key.pub
while true; do
  if [[ -n $(/usr/bin/rpmquery -a %[2]s) ]]; then
    break
  fi
  sleep 10
done
while true; do
  if [[ -n $(/usr/bin/rpmquery -a %[3]s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    break
  fi
  sleep 10
done`
		install := utils.InstallOSConfigEL(image)
		if strings.Contains(image, "suse") {
			install = utils.InstallOSConfigSUSE()
		}
		ss = fmt.Sprintf(ss, install, wantInstall[0], wantInstall[1], packageInstalled)
		key = "startup-script"

	case "googet":
		wantInstall := "cowsay"
		wantRemove := "certgen"
		ss = `
googet addrepo test https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository
%s
while(1) {
  # make sure the package we want installed is installed
  $installed_packages = googet installed
  if ($installed_packages -like "*%[3]s*") {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[5]s'
    Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
	break
  }
  sleep 10
}
while(1) {
  # make sure the package we want removed is removed
  $installed_packages = googet installed
  if ($installed_packages -notlike "*%[2]s*") {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s'
    Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
	break
  }
  sleep 10
}`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigGooGet(), wantRemove, wantInstall, packageInstalled, packageNotInstalled)
		key = "windows-startup-script-ps1"

	case "msi":
		ss = `
%s
while(1) {
  if (Get-WmiObject -Class Win32_Product | Where-Object {$_.Name -like "*Chrome*"}) {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s'
    Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
	break
  }
  sleep 10
}`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigGooGet(), packageInstalled)
		key = "windows-startup-script-ps1"

	case "zypper":
		wantInstall := "ed"
		wantRemove := "vim"
		ss = `set -x
# install the package we want removed
zypper -n install %[2]s
# remove the package we want installed
zypper -n remove %[3]s
%[1]s
while true; do
  # make sure the package we want installed is installed
  if [[ -n $(/usr/bin/rpmquery -a %[3]s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
	break
  fi
  sleep 10
done
while true; do
  # make sure the package we want removed is removed
  if [[ -z $(/usr/bin/rpmquery -a %[2]s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[5]s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
	break
  fi
  sleep 10
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigSUSE(), wantRemove, wantInstall, packageInstalled, packageNotInstalled)
		key = "startup-script"

	default:
		fmt.Printf("Invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}

func getStartupScriptRepo(image, pkgManager, packageName string) *computeApi.MetadataItems {
	var ss, key string

	switch pkgManager {
	case "apt":
		ss = `set -x
%s
while true; do
  isinstalled=$(/usr/bin/dpkg-query -s %s)
  if [[ $isinstalled =~ "Status: install ok installed" ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    exit 0
  fi
  sleep 10
done`

		ss = fmt.Sprintf(ss, utils.InstallOSConfigDeb(), packageName, packageInstalled)
		key = "startup-script"

	case "yum":
		ss = `set -x
%s
while true; do
  if [[ -n $(/usr/bin/rpmquery -a %s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
    exit 0
  fi
  sleep 10
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigEL(image), packageName, packageInstalled)
		key = "startup-script"

	case "googet":
		ss = `%s
while(1) {
  $installed_packages = googet installed
  if ($installed_packages -like "*%s*") {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s'
    Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
    exit 0
  }
  sleep 10
}`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigGooGet(), packageName, packageInstalled)
		key = "windows-startup-script-ps1"

	case "zypper":
		ss = `set -x
sleep 10
# Update zypper since there were older versions with bugs.
zypper -n install zypper
%s
while true; do
  if [[ -n $(/usr/bin/rpmquery -a %s) ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
    curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
	exit 0
  fi
  sleep 10
done`
		// SLES 12 has an old version of Zypper which has bugs in gpg importing logic.
		if strings.Contains(image, "sles-12") {
			ss = "rpm --import https://packages.cloud.google.com/yum/doc/yum-key.gpg\n" + ss
		}
		ss = fmt.Sprintf(ss, utils.InstallOSConfigSUSE(), packageName, packageInstalled)
		key = "startup-script"

	default:
		fmt.Printf("Invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}

func getStartupScriptFile(image, pkgManager, dnePath string, wantPaths []string) *computeApi.MetadataItems {
	var ss, key string

	linux := `set -x
touch %[1]s
%[2]s
echo "Checking for %[1]s"
while [[ -f %[1]s ]]; do
  echo "%[1]s exists"
  sleep 5
done
echo "%[1]s DNE"
uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s
curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"`
	linuxCheck := `
echo "Checking for %[1]s"
while [[ ! -f %[1]s ]]; do
  echo "%[1]s DNE"
  sleep 10
done
echo "%[1]s exists"`

	linuxEnd := fmt.Sprintf(`
uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"`, fileExists)

	switch pkgManager {
	case "apt":
		ss = fmt.Sprintf(linux, dnePath, utils.InstallOSConfigDeb(), fileDNE)
		for _, p := range wantPaths {
			ss += fmt.Sprintf(linuxCheck, p)
		}
		ss += linuxEnd
		key = "startup-script"

	case "yum":
		ss = fmt.Sprintf(linux, dnePath, utils.InstallOSConfigEL(image), fileDNE)
		for _, p := range wantPaths {
			ss += fmt.Sprintf(linuxCheck, p)
		}
		ss += linuxEnd
		key = "startup-script"

	case "zypper":
		ss = fmt.Sprintf(linux, dnePath, utils.InstallOSConfigSUSE(), fileDNE)
		for _, p := range wantPaths {
			ss += fmt.Sprintf(linuxCheck, p)
		}
		ss += linuxEnd
		key = "startup-script"

	case "googet":
		ss = `
New-Item -ItemType File -Path %[1]s
%[2]s
Write-Host "Checking for %[1]s"
while (Test-Path %[1]s) {
  Write-Host "%[1]s exists"
  sleep 10
}
$uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s'
Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1`
		ss = fmt.Sprintf(ss, dnePath, utils.InstallOSConfigGooGet(), fileDNE)

		windows := `
Write-Host "Checking for %[1]s"
while ( ! (Test-Path %[1]s) ) {
  Write-Host "%[1]s DNE"
  sleep 10
}
Write-Host "%[1]s exists"`
		for _, p := range wantPaths {
			ss += fmt.Sprintf(windows, p)
		}
		ss += fmt.Sprintf(`
$uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s'
Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1`, fileExists)
		key = "windows-startup-script-ps1"

	default:
		fmt.Printf("Invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}

func getStartupScriptExec(image, pkgManager string, wantPaths []string) *computeApi.MetadataItems {
	var ss, key string

	linux := `set -x
echo 'if ls $1 >/dev/null; then
exit 100
fi
exit 101' > /validate_shell

echo '#!/bin/sh
touch $1
exit 100' > /enforce_none
chmod +x /enforce_none`

	linuxCheck := `
echo "Checking for %[1]s"
while [[ ! -f %[1]s ]]; do
  echo "%[1]s DNE"
  sleep 10
done
echo "%[1]s exists"`
	var linuxChecks string
	for _, p := range wantPaths {
		linuxChecks += fmt.Sprintf(linuxCheck, p)
	}

	linuxEnd := fmt.Sprintf(`
uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"`, fileExists)

	switch pkgManager {
	case "apt":
		ss = linux + utils.InstallOSConfigDeb() + linuxChecks + linuxEnd
		key = "startup-script"

	case "yum":
		ss = linux + utils.InstallOSConfigEL(image) + linuxChecks + linuxEnd
		key = "startup-script"

	case "zypper":
		ss = linux + utils.InstallOSConfigSUSE() + linuxChecks + linuxEnd
		key = "startup-script"

	case "googet":
		ss = `
@'
if exist %1 exit 100
exit 101
'@ | Out-File -Encoding ASCII /validate.cmd
@'
echo "" > %1
exit 100
'@ | Out-File -Encoding ASCII /enforce.cmd
'if (Test-Path $Args[0]) {exit 100}; exit 101' > /validate.ps1
'New-Item -ItemType File -Path $Args[0]; exit 100' > /enforce.ps1`
		ss += utils.InstallOSConfigGooGet()
		check := `
Write-Host "Checking for %[1]s"
while ( ! (Test-Path %[1]s) ) {
  Write-Host "%[1]s DNE"
  sleep 10
}
Write-Host "%[1]s exists"`
		for _, p := range wantPaths {
			ss += fmt.Sprintf(check, p)
		}
		ss += fmt.Sprintf(`
$uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s'
Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1`, fileExists)
		key = "windows-startup-script-ps1"

	default:
		fmt.Printf("Invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}
