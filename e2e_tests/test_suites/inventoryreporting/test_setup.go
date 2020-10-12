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
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	computeAPI "google.golang.org/api/compute/v1"
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
}

var (
	windowsSetup = &inventoryTestSetup{
		packageType: []string{"googet", "wua", "qfe"},
		shortName:   "windows",

		startup:     compute.BuildInstanceMetadataItem("windows-startup-script-ps1", utils.InstallOSConfigGooGet()),
		machineType: "e2-standard-4",
		timeout:     25 * time.Minute,
	}

	aptSetup = &inventoryTestSetup{
		packageType: []string{"deb"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigDeb()),
		machineType: "e2-standard-2",
		timeout:     10 * time.Minute,
	}

	el6Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigEL6()),
		machineType: "e2-standard-2",
		timeout:     10 * time.Minute,
	}

	el7Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigEL7()),
		machineType: "e2-standard-2",
		timeout:     10 * time.Minute,
	}

	el8Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigEL8()),
		machineType: "e2-standard-2",
		timeout:     10 * time.Minute,
	}

	suseSetup = &inventoryTestSetup{
		packageType: []string{"zypper"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigSUSE()),
		machineType: "e2-standard-2",
		timeout:     15 * time.Minute,
	}

	cosSetup = &inventoryTestSetup{
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.CurlPost),
		machineType: "e2-standard-2",
		timeout:     5 * time.Minute,
	}
)

func headImageTestSetup() (setup []*inventoryTestSetup) {
	// This maps a specific inventoryTestSetup to test setup names and associated images.
	headTestSetupMapping := map[*inventoryTestSetup]map[string]string{
		windowsSetup: utils.HeadWindowsImages,
		el6Setup:     utils.HeadEL6Images,
		el7Setup:     utils.HeadEL7Images,
		el8Setup:     utils.HeadEL8Images,
		aptSetup:     utils.HeadAptImages,
		suseSetup:    utils.HeadSUSEImages,
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
