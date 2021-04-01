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

func getStartupScript(image, pkgManager, packageName string) *computeApi.MetadataItems {
	var ss, key string

	switch pkgManager {
	case "apt":
		ss = `
apt-get -y remove %[2]s
%[1]s
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

		ss = fmt.Sprintf(ss, utils.InstallOSConfigDeb(), packageName, packageInstalled, packageNotInstalled)
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
while true; do
  isinstalled=$(/usr/bin/rpmquery -a %[2]s)
  if [[ $isinstalled =~ ^%[2]s-* ]]; then
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  else
    uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`
		ss = fmt.Sprintf(ss, yumStartupScripts[path.Base(image)], packageName, packageInstalled, packageNotInstalled)
		key = "startup-script"

	case "googet":
		ss = `
googet addrepo test https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository
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
		ss = fmt.Sprintf(ss, utils.InstallOSConfigGooGet(), packageName, packageInstalled, packageNotInstalled)
		key = "windows-startup-script-ps1"

	case "zypper":
		ss = `
zypper -n remove %[2]s
%[1]s
while true; do
  isinstalled=$(/usr/bin/rpmquery -a %[2]s)
  if [[ $isinstalled =~ ^%[2]s-* ]]; then
	  uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  else
  	uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/%s
  fi
  curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
  sleep 5
done`
		ss = fmt.Sprintf(ss, utils.InstallOSConfigSUSE(), packageName, packageInstalled, packageNotInstalled)
		key = "startup-script"

	default:
		fmt.Printf("Invalid package manager: %s", pkgManager)
	}

	return &computeApi.MetadataItems{
		Key:   key,
		Value: &ss,
	}
}
