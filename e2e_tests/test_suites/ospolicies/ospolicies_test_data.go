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
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	"google.golang.org/protobuf/types/known/durationpb"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha"
)

const (
	packageInstalled    = "osconfig_tests/pkg_installed"
	packageNotInstalled = "osconfig_tests/pkg_not_installed"
	osconfigTestRepo    = "osconfig-agent-test-repository"
	testResourceBucket  = "osconfig-agent-end2end-test-resources"
	yumTestRepoBaseURL  = "https://packages.cloud.google.com/yum/repos/osconfig-agent-test-repository"
	aptTestRepoBaseURL  = "http://packages.cloud.google.com/apt"
	gooTestRepoURL      = "https://packages.cloud.google.com/yuck/repos/osconfig-agent-test-repository"
	aptRaptureGpgKey    = "https://packages.cloud.google.com/apt/doc/apt-key.gpg"
)

var (
	yumRaptureGpgKeys = []string{"https://packages.cloud.google.com/yum/doc/yum-key.gpg", "https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg"}
)

func buildPkgInstallTestSetup(name, image, pkgManager, key string) *osPolicyTestSetup {
	assertTimeout := 120 * time.Second
	testName := packageInstallFunction
	packageName := "ed"
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		packageName = "cowsay"
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	ospa := &osconfigpb.OSPolicyAssignment{
		InstanceFilter: &osconfigpb.OSPolicyAssignment_InstanceFilter{
			InclusionLabels: []*osconfigpb.OSPolicyAssignment_LabelSet{{
				Labels: map[string]string{"name": instanceName}},
			},
		},
		Rollout: &osconfigpb.OSPolicyAssignment_Rollout{
			DisruptionBudget: &osconfigpb.FixedOrPercent{Mode: &osconfigpb.FixedOrPercent_Percent{Percent: 100}},
			MinWaitDuration:  &durationpb.Duration{Seconds: 0},
		},
		OsPolicies: []*osconfigpb.OSPolicy{
			&osconfigpb.OSPolicy{
				Id:   "install-packages",
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					&osconfigpb.OSPolicy_ResourceGroup{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "debian"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							&osconfigpb.OSPolicy_Resource{
								Id: "install-debian",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Apt{
											Apt: &osconfigpb.OSPolicy_Resource_PackageResource_APT{Name: packageName},
										},
									},
								},
							},
						},
					},
					&osconfigpb.OSPolicy_ResourceGroup{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "ubuntu"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							&osconfigpb.OSPolicy_Resource{
								Id: "install-ubuntu",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Apt{
											Apt: &osconfigpb.OSPolicy_Resource_PackageResource_APT{Name: packageName},
										},
									},
								},
							},
						},
					},
					&osconfigpb.OSPolicy_ResourceGroup{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "rhel"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							&osconfigpb.OSPolicy_Resource{
								Id: "install-rhel",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Yum{
											Yum: &osconfigpb.OSPolicy_Resource_PackageResource_YUM{Name: packageName},
										},
									},
								},
							},
						},
					},
					&osconfigpb.OSPolicy_ResourceGroup{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "centos"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							&osconfigpb.OSPolicy_Resource{
								Id: "install-centos",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Yum{
											Yum: &osconfigpb.OSPolicy_Resource_PackageResource_YUM{Name: packageName},
										},
									},
								},
							},
						},
					},
					&osconfigpb.OSPolicy_ResourceGroup{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "windows"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							&osconfigpb.OSPolicy_Resource{
								Id: "install-windows",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Googet{
											Googet: &osconfigpb.OSPolicy_Resource_PackageResource_GooGet{Name: packageName},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ss := getStartupScript(name, pkgManager, packageName)
	return newOsPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, ospa, ss, assertTimeout)
}

func addPackageInstallTest(key string) []*osPolicyTestSetup {
	var pkgTestSetup []*osPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func generateAllTestSetup() []*osPolicyTestSetup {
	key := utils.RandString(3)

	pkgTestSetup := []*osPolicyTestSetup{}
	pkgTestSetup = append(pkgTestSetup, addPackageInstallTest(key)...)
	return pkgTestSetup
}
