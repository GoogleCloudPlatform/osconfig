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
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	computeAPI "google.golang.org/api/compute/v1"

	osconfigpb "google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha"
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
		packageType: []string{"googet", "wua", "qfe"},
		shortName:   "windows",

		startup:     compute.BuildInstanceMetadataItem("windows-startup-script-ps1", utils.InstallOSConfigGooGet()),
		machineType: "e2-standard-2",
		timeout:     25 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			var foundGooget bool
			var qfeExists bool
			var wuaExists bool
			for _, item := range items {
				if item.GetInstalledPackage().GetGoogetPackage().GetPackageName() == "googet" {
					foundGooget = true
				}
				if item.GetInstalledPackage().GetQfePackage() != nil {
					qfeExists = true
				}
				if item.GetInstalledPackage().GetWuaPackage() != nil {
					wuaExists = true
				}
			}
			if !foundGooget {
				return errors.New("did not find 'googet' in installed packages")
			}
			if !qfeExists {
				return errors.New("did not find any QFE installed package")
			}
			if !wuaExists {
				return errors.New("did not find any WUA installed package")
			}
			return nil
		},
	}

	aptSetup = &inventoryTestSetup{
		packageType: []string{"deb"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigDeb()),
		machineType: "e2-medium",
		timeout:     10 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			for _, item := range items {
				if item.GetInstalledPackage().GetAptPackage().GetPackageName() == "bash" {
					return nil
				}
			}
			return errors.New("did not find 'bash' in installed packages")
		},
	}
	yumBashInstalledCheck = func(items map[string]*osconfigpb.Inventory_Item) error {
		for _, item := range items {
			if item.GetInstalledPackage().GetYumPackage().GetPackageName() == "bash" {
				return nil
			}
		}
		return errors.New("did not find 'bash' in installed packages")
	}

	el6Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigEL6()),
		machineType: "e2-medium",
		timeout:     10 * time.Minute,
		itemCheck:   yumBashInstalledCheck,
	}

	el7Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigEL7()),
		machineType: "e2-medium",
		timeout:     10 * time.Minute,
		itemCheck:   yumBashInstalledCheck,
	}

	el8Setup = &inventoryTestSetup{
		packageType: []string{"rpm"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigEL8()),
		machineType: "e2-medium",
		timeout:     10 * time.Minute,
		itemCheck:   yumBashInstalledCheck,
	}

	suseSetup = &inventoryTestSetup{
		packageType: []string{"zypper"},
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.InstallOSConfigSUSE()),
		machineType: "e2-medium",
		timeout:     15 * time.Minute,
		itemCheck: func(items map[string]*osconfigpb.Inventory_Item) error {
			for _, item := range items {
				if item.GetInstalledPackage().GetZypperPackage().GetPackageName() == "bash" {
					return nil
				}
			}
			return errors.New("did not find 'bash' in installed packages")
		},
	}

	cosSetup = &inventoryTestSetup{
		startup:     compute.BuildInstanceMetadataItem("startup-script", utils.CosSetup),
		machineType: "e2-medium",
		timeout:     5 * time.Minute,
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
