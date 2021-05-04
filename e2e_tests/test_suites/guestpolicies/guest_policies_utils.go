//  Copyright 2019 Google Inc. All Rights Reserved.
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

package guestpolicies

import (
	"fmt"
	"path"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	computeApi "google.golang.org/api/compute/v1"
)

var (
	yumStartupScripts = map[string]string{
		"rhel-6":   utils.InstallOSConfigEL6(),
		"rhel-7":   utils.InstallOSConfigEL7(),
		"rhel-8":   utils.InstallOSConfigEL8(),
		"centos-6": utils.InstallOSConfigEL6(),
		"centos-7": utils.InstallOSConfigEL7(),
		"centos-8": utils.InstallOSConfigEL8(),
	}
)

var waitForRestartLinux = `
echo 'Waiting for signal to restart agent'
while [[ -z $restarted ]]; do
  sleep 1
  restart=$(curl -f "http://metadata.google.internal/computeMetadata/v1/instance/attributes/restart-agent" -H "Metadata-Flavor: Google")
  if [[ -n $restart ]]; then
    systemctl restart google-osconfig-agent
    restart -q -n google-osconfig-agent  # required for EL6
    restarted=true
    sleep 30
  fi
done
`

var waitForRestartWin = `
echo 'Waiting for signal to restart agent'
while (! $restarted) {
  sleep 1
  $restart = Invoke-WebRequest -UseBasicParsing http://metadata.google.internal/computeMetadata/v1/instance/attributes/restart-agent -Headers @{"Metadata-Flavor"="Google"}
  if ($restart) {
    Restart-Service google_osconfig_agent
    $restarted = $true
    sleep 30
  }
}
`

func getStartupScript(image, pkgManager, packageName string) *computeApi.MetadataItems {
	var ss, key string

	switch pkgManager {
	case "apt":
		ss = `
apt-get -y remove %[3]s
%[1]s
%[2]s
while true; do
  isinstalled=$(/usr/bin/dpkg-query -s %s)
  if [[ $isinstalled =~ "Status: install ok installed" ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`

		ss = fmt.Sprintf(ss, utils.InstallOSConfigDeb(), waitForRestartLinux, packageName, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "yum":
		ss = `
while ! yum -y remove %[3]s; do
  if [[ n -gt 5 ]]; then
    exit 1
  fi
  n=$[$n+1]
  sleep 10
done
%[1]s
%[2]s
while true; do
  isinstalled=$(/usr/bin/rpmquery -a %[3]s)
  if [[ $isinstalled =~ ^%[3]s-* ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`
		ss = fmt.Sprintf(ss, yumStartupScripts[path.Base(image)], waitForRestartLinux, packageName, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "googet":
		ss = `
googet addrepo test https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository
%s
%s
while(1) {
  $installed_packages = googet installed
  if ($installed_packages -like "*%s*") {
	  $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s'
  } else {
	  $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s'
  }
  Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
  sleep 5
}`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigGooGet(), waitForRestartWin, packageName, packageInstalled, packageNotInstalled)
		key = "windows-startup-script-ps1"

	case "zypper":
		ss = `
sleep 10
# Update zypper since there were older versions with bugs.
zypper -n install zypper
zypper -n remove %[3]s
%[1]s
%[2]s
while true; do
  isinstalled=$(/usr/bin/rpmquery -a %[3]s)
  if [[ $isinstalled =~ ^%[3]s-* ]]; then
	  uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  else
  	uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigSUSE(), waitForRestartLinux, packageName, packageInstalled, packageNotInstalled)
		key = "startup-script"

	default:
		fmt.Printf("Invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}

func getUpdateStartupScript(image, pkgManager string) *computeApi.MetadataItems {
	var ss, key string

	switch pkgManager {
	case "apt":
		ss = `
echo 'Adding test repo'
echo 'deb http://packages.cloud.google.com/apt osconfig-agent-test-repository main' >> /etc/apt/sources.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
   sleep 5
done
apt-get update
apt-get -y remove ed || exit 1
apt-get -y install ed=1.9-2 || exit 1
%[1]s
%[2]s
while true; do
  isinstalled=$(/usr/bin/dpkg-query -f '${Version}' -W ed)
  if [[ $isinstalled == "1.9-2" ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5;
done`

		ss = fmt.Sprintf(ss, utils.InstallOSConfigDeb(), waitForRestartLinux, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "yum":
		ss = `
echo 'Adding test repo'
cat > /etc/yum.repos.d/google-osconfig-agent.repo <<EOM
[test-repo]
name=test repo
baseurl=https://packages.cloud.google.com/yum/repos/osconfig-agent-test-repository
enabled=1
gpgcheck=0
EOM
n=0
while ! yum -y remove ed; do
  if [[ n -gt 5 ]]; then
    exit 1
  fi
  n=$[$n+1]
  sleep 10
done
while ! yum -y install ed-0.2-39.el5_2; do
  if [[ n -gt 5 ]]; then
    exit 1
  fi
  n=$[$n+1]
  sleep 10
done
%[1]s
%[2]s
while true; do
  isinstalled=$(/usr/bin/rpmquery -a ed)
  if [[ $isinstalled =~ 0.2-39.el5_2 ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`
		ss = fmt.Sprintf(ss, yumStartupScripts[path.Base(image)], waitForRestartLinux, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "googet":
		ss = `
echo 'Adding test repo'
googet addrepo test https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository
googet -noconfirm remove cowsay
googet -noconfirm install cowsay.x86_64.0.1.0@1
%[1]s
%[2]s
while(1) {
  $installed_packages = googet installed cowsay
  Write-Host $installed_packages
  if ($installed_packages -like "*0.1.0@1*") {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s'
  } else {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s'
  }
  Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
  sleep 5
}`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigGooGet(), waitForRestartWin, packageInstalled, packageNotInstalled)
		key = "windows-startup-script-ps1"

	case "zypper":
		ss = `
echo 'Adding test repo'
cat > /etc/zypp/repos.d/google-osconfig-agent.repo <<EOM
[test-repo]
name=test repo
baseurl=https://packages.cloud.google.com/yum/repos/osconfig-agent-test-repository
enabled=1
gpgcheck=0
EOM
zypper -n remove ed
zypper -n --no-gpg-checks install ed-0.2-39.el5_2
%[1]s
%[2]s
while true; do
  isinstalled=$(/usr/bin/rpmquery -a ed)
  if [[ $isinstalled =~ 0.2-39.el5_2 ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[4]s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigSUSE(), waitForRestartLinux, packageInstalled, packageNotInstalled)
		key = "startup-script"

	default:
		fmt.Printf("invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}

func getRecipeInstallStartupScript(image, recipeName, pkgManager string) *computeApi.MetadataItems {
	scriptLinux := fmt.Sprintf(`
# loop and check for recipedb entry
while true; do
is_installed=$(grep '{"Name":"%[1]s","Version":\[0],"InstallTime":[0-9]*,"Success":true}' /var/lib/google/osconfig_recipedb)
  if [[ -n $is_installed ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[2]s
   else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done
`, recipeName, packageInstalled, packageNotInstalled)

	scriptWin := fmt.Sprintf(`
# loop and check for recipedb entry
while ($true) {
  $is_installed=$(cat 'C:\ProgramData\Google\osconfig_recipedb' | select-string '{"Name":"%[1]s","Version":\[0],"InstallTime":[0-9]+,"Success":true}' )
  if ($is_installed) {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[2]s'
  } else {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s'
  }
  Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
  sleep 5
}
`, recipeName, packageInstalled, packageNotInstalled)

	var script string
	key := "startup-script"
	switch pkgManager {
	case "cos":
		script = fmt.Sprintf("%s\n%s\n%s", utils.CurlPost, waitForRestartLinux, scriptLinux)
	case "apt":
		script = fmt.Sprintf("%s\n%s\n%s", utils.InstallOSConfigDeb(), waitForRestartLinux, scriptLinux)
	case "yum":
		script = fmt.Sprintf("%s\n%s\n%s", yumStartupScripts[path.Base(image)], waitForRestartLinux, scriptLinux)
	case "zypper":
		script = fmt.Sprintf("%s\n%s\n%s", utils.InstallOSConfigSUSE(), waitForRestartLinux, scriptLinux)
	case "googet":
		script = fmt.Sprintf("%s\n%s\n%s", utils.InstallOSConfigGooGet(), waitForRestartWin, scriptWin)
		key = "windows-startup-script-ps1"
	default:
		fmt.Printf("invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &script,
	}
}

func getRecipeStepsStartupScript(image, recipeName, pkgManager string) *computeApi.MetadataItems {
	scriptLinux := `
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
	if pkgManager != "cos" {
		scriptLinux = scriptLinux + `
while [[ ! -f /tmp/osconfig-SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED ]]; do
  sleep 1
done
while [[ ! -f /tmp/osconfig-exec-test ]]; do
  sleep 1
done
    `
	}
	scriptLinux = scriptLinux + fmt.Sprintf(`
while true; do
  isinstalled=$(grep '{"Name":"%[1]s","Version":\[0],"InstallTime":[0-9]*,"Success":true}' /var/lib/google/osconfig_recipedb)
  if [[ -n $isinstalled ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[2]s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 1
done
`, recipeName, packageInstalled, packageNotInstalled)

	scriptWin := fmt.Sprintf(`
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
#while ( ! (Test-Path c:\zip-test\zip\test.txt) ) {
#  sleep 1
#}
while ($true) {
  $is_installed=$(cat 'C:\ProgramData\Google\osconfig_recipedb' | select-string '{"Name":"%[1]s","Version":\[0],"InstallTime":[0-9]+,"Success":true}' )
  if ($is_installed) {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[2]s'
  } else {
    $uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%[3]s'
  }
  Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
  sleep 1
}
`, recipeName, packageInstalled, packageNotInstalled)

	var script string
	key := "startup-script"
	switch pkgManager {
	case "cos":
		script = fmt.Sprintf("%s\n%s\n%s", utils.CurlPost, waitForRestartLinux, scriptLinux)
	case "apt":
		script = fmt.Sprintf("%s\n%s\n%s", utils.InstallOSConfigDeb(), waitForRestartLinux, scriptLinux)
	case "yum":
		script = fmt.Sprintf("%s\n%s\n%s", yumStartupScripts[path.Base(image)], waitForRestartLinux, scriptLinux)
	case "zypper":
		script = fmt.Sprintf("%s\n%s\n%s", utils.InstallOSConfigSUSE(), waitForRestartLinux, scriptLinux)
	case "googet":
		script = fmt.Sprintf("%s\n%s\n%s", utils.InstallOSConfigGooGet(), waitForRestartWin, scriptWin)
		key = "windows-startup-script-ps1"
	default:
		fmt.Printf("invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &script,
	}
}
