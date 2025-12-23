//  Copyright 2020 Google Inc. All Rights Reserved.
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

package inventoryreporting

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	computeAPI "google.golang.org/api/compute/v1"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/google.golang.org/genproto/googleapis/cloud/osconfig/v1"
)

type inventoryTestSetup struct {
	testName    string
	hostname    string
	image       string
	packageType []string
	shortName   string
	startup     *computeAPI.MetadataItems
	machineType string
	timeout     time.Duration
	itemCheck   func(items map[string]*osconfigpb.Inventory_Item) error
}

var (
	windowsSetup = &inventoryTestSetup{
		packageType: []string{"googet", "wua", "qfe", "windowsapplication"},
		shortName:   "windows",

		startup:     compute.BuildInstanceMetadataItem("windows-startup-script-ps1", getStartupScriptGoo()),
		machineType: "e2-standard-2",
		timeout:     30 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			var foundGooget bool
			var foundGoogleComputeEngineDriverGvnic bool
			var qfeExists bool
			var wuaExists bool
			var windowsApplicationExist bool
			missingWindowsApplications := map[string]bool{
				"GooGet - google-osconfig-agent": false,
				"GooGet - googet":                false,
				"Google Cloud SDK":               false,
			}

			fmt.Println(items)
			for _, item := range items {
				if item.GetInstalledPackage().GetGoogetPackage().GetPackageName() == "googet" {
					foundGooget = true
				}
				if item.GetAvailablePackage().GetGoogetPackage().GetPackageName() == "google-compute-engine-windows" {
					foundGoogleComputeEngineDriverGvnic = true
				}
				if item.GetInstalledPackage().GetQfePackage() != nil {
					qfeExists = true
				}
				if item.GetInstalledPackage().GetWuaPackage() != nil {
					wuaExists = true
				}
				windowsApplication := item.GetInstalledPackage().GetWindowsApplication()
				if windowsApplication != nil {
					windowsApplicationExist = true
					displayName := windowsApplication.GetDisplayName()
					if _, ok := missingWindowsApplications[displayName]; ok {
						delete(missingWindowsApplications, displayName)
					}
				}
			}

			if !foundGooget {
				return errors.New("did not find 'googet' in installed packages")
			}
			if !foundGoogleComputeEngineDriverGvnic {
				return errors.New("did not find 'google-compute-engine-driver-gvnic' in available packages")
			}
			if !qfeExists {
				return errors.New("did not find any QFE installed package")
			}
			if !wuaExists {
				return errors.New("did not find any WUA installed package")
			}
			if !windowsApplicationExist {
				return errors.New("did not find any Windows Application installed package")
			}
			if len(missingWindowsApplications) != 0 {
				missingApplications := []string{}
				for app := range missingWindowsApplications {
					missingApplications = append(missingApplications, app)
				}
				return errors.New("did not find Windows Applications: " + strings.Join(missingApplications, ", "))
			}

			return nil
		},
	}

	// apt setup
	busterAptSetup   = createAptSetup("debian-10")
	bullseyeAptSetup = createAptSetup("debian-11")
	bookwormAptSetup = createAptSetup("debian-12")

	yumBashInstalledCheck = func(items map[string]*osconfigpb.Inventory_Item) error {
		var bashFound bool
		var gcsfuseFound bool
		for _, item := range items {
			if item.GetInstalledPackage().GetYumPackage().GetPackageName() == "bash" {
				bashFound = true
			}
			if item.GetInstalledPackage().GetYumPackage().GetPackageName() == "gcsfuse" {
				gcsfuseFound = true
			}
		}
		if !bashFound {
			return errors.New("did not find 'bash' in installed packages")
		}
		if !gcsfuseFound {
			return errors.New("did not find 'gcsfuse' in installed packages")
		}
		return nil
	}

	el8Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", getStartupScriptEL("8")),
		machineType: "e2-medium",
		timeout:     25 * time.Minute,
		itemCheck:   yumBashInstalledCheck,
	}

	el9Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", getStartupScriptEL("9")),
		machineType: "e2-medium",
		timeout:     25 * time.Minute,
		itemCheck:   yumBashInstalledCheck,
	}

	suseSetup = &inventoryTestSetup{
		packageType: []string{"zypper"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", getStartupScriptZypper()),
		machineType: "e2-medium",
		timeout:     25 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			var bashFound bool
			var gcsfuseFound bool
			for _, item := range items {
				if item.GetInstalledPackage().GetZypperPackage().GetPackageName() == "bash" {
					bashFound = true
				}
				if item.GetInstalledPackage().GetZypperPackage().GetPackageName() == "gcsfuse" {
					gcsfuseFound = true
				}
			}
			if !bashFound {
				return errors.New("did not find 'bash' in installed packages")
			}
			if !gcsfuseFound {
				return errors.New("did not find 'gcsfuse' in installed packages")
			}
			return nil
		},
	}

	cosSetup = &inventoryTestSetup{
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.CosSetup),
		machineType: "e2-medium",
		timeout:     25 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			for _, item := range items {
				if item.GetInstalledPackage().GetCosPackage().GetPackageName() == "app-shells/bash" {
					return nil
				}
			}
			return errors.New("did not find 'app-shells/bash' in installed packages")
		},
	}
)

func createAptSetup(image string) *inventoryTestSetup {
	return &inventoryTestSetup{
		packageType: []string{"deb"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", getStartupScriptDeb(image)),
		machineType: "e2-medium",
		timeout:     25 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			var bashFound bool
			var gcsfuseFound bool
			for _, item := range items {
				if item.GetInstalledPackage().GetAptPackage().GetPackageName() == "bash" {
					bashFound = true
				}
				if item.GetInstalledPackage().GetAptPackage().GetPackageName() == "gcsfuse" {
					gcsfuseFound = true
				}
			}
			if !bashFound {
				return errors.New("did not find 'bash' in installed packages")
			}
			if !gcsfuseFound {
				return errors.New("did not find 'gcsfuse' in installed packages")
			}
			return nil
		},
	}
}

func getStartupScriptEL(image string) string {
	ss := `
echo 'Adding gcsfuse repo'
cat > /etc/yum.repos.d/gcsfuse.repo <<EOM
[gcsfuse]
name=gcsfuse
baseurl=https://packages.cloud.google.com/yum/repos/gcsfuse-el7-x86_64
enabled=1
gpgcheck=0
EOM
n=0
while ! yum -y install gcsfuse; do
  if [[ n -gt 5 ]]; then
    exit 1
  fi
  n=$[$n+1]
  sleep 10
done
%s`
	return fmt.Sprintf(ss, utils.InstallOSConfigEL(image))
}

func getStartupScriptDeb(image string) string {
	gcsfuseAptRepoBane := fmt.Sprintf("gcsfuse-%s", utils.GetDebOsName(image))

	ss := `
echo 'Adding gcsfuse repo'

# install gnupg2 if not exist
apt-get update
apt-get install -y gnupg2

echo 'deb http://packages.cloud.google.com/apt %s main' >> /etc/apt/sources.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
   sleep 5
done
apt-get update
apt-get -y install gcsfuse || exit 1
%s`
	return fmt.Sprintf(ss, gcsfuseAptRepoBane, utils.InstallOSConfigDeb(image))
}

func getStartupScriptGoo() string {
	ss := `
echo 'Adding test repo'
googet addrepo test https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository
%s`
	return fmt.Sprintf(ss, utils.InstallOSConfigGooGet())
}

func getStartupScriptZypper() string {
	ss := `
echo 'Adding gcsfuse repo'
cat > /etc/zypp/repos.d/osconfig-agent-test.repo <<EOM
[gcsfuse]
name=gcsfuse
baseurl=https://packages.cloud.google.com/yum/repos/gcsfuse-el7-x86_64
enabled=1
gpgcheck=0
EOM
zypper -n --no-gpg-checks install gcsfuse
%s`
	return fmt.Sprintf(ss, utils.InstallOSConfigSUSE())
}

func headImageTestSetup() (setup []*inventoryTestSetup) {
	// This maps a specific inventoryTestSetup to test setup names and associated images.
	headTestSetupMapping := map[*inventoryTestSetup]map[string]string{
		windowsSetup:     utils.HeadWindowsImages,
		el8Setup:         utils.HeadEL8Images,
		el9Setup:         utils.HeadEL9Images,
		busterAptSetup:   utils.HeadBusterAptImages,
		bullseyeAptSetup: utils.HeadBullseyeAptImages,
		bookwormAptSetup: utils.HeadBookwormAptImages,
		suseSetup:        utils.HeadSUSEImages,
	}

	// TODO: remove this hack and setup specific test suites for each test type.
	// This ensures we only run cos tests on the "head image" tests.
	if config.AgentRepo() == "" {
		headTestSetupMapping[cosSetup] = utils.HeadCOSImages
	}

	for s, m := range headTestSetupMapping {
		for name, image := range m {
			new := inventoryTestSetup(*s)
			new.testName = name
			new.image = image
			if strings.Contains(name, "centos") {
				new.shortName = "centos"
			} else if strings.Contains(name, "rhel") {
				new.shortName = "rhel"
			} else if strings.Contains(name, "rocky") {
				new.shortName = "rocky"
			} else if strings.Contains(name, "debian") {
				new.shortName = "debian"
			} else if strings.Contains(name, "ubuntu") {
				new.shortName = "ubuntu"
			} else if strings.Contains(name, "sles") {
				new.shortName = "sles"
			} else if strings.Contains(name, "opensuse-leap") {
				new.shortName = "opensuse-leap"
			} else if strings.Contains(name, "cos") {
				new.shortName = "cos"
			}
			setup = append(setup, &new)
		}
	}
	return
}
