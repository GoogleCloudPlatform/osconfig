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
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	"google.golang.org/protobuf/types/known/durationpb"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/e2e_tests/internal/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha"
)

const (
	packageInstalled    = "osconfig_tests/pkg_installed"
	packageNotInstalled = "osconfig_tests/pkg_not_installed"
	fileExists          = "osconfig_tests/file_exists"
	fileDNE             = "osconfig_tests/file_does_not_exist"
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
			{
				Id:   "install-packages",
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "debian"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "ubuntu"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "rhel"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "centos"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "sles"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-sles",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Zypper_{
											Zypper: &osconfigpb.OSPolicy_Resource_PackageResource_Zypper{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "windows"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
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

	var os string
	switch {
	case strings.Contains(image, "debian"):
		os = "debian"
	case strings.Contains(image, "windows"):
		os = "windows"
	case strings.Contains(image, "rhel"):
		os = "rhel"
	case strings.Contains(image, "centos"):
		os = "centos"
	case strings.Contains(image, "sles"):
		os = "sles"
	}
	wantCompliances := []*osconfigpb.InstanceOSPoliciesCompliance_OSPolicyCompliance{
		{
			OsPolicyId: "install-packages",
			State:      osconfigpb.OSPolicyComplianceState_COMPLIANT,
			OsPolicyResourceCompliances: []*osconfigpb.OSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "install-" + os,
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
			},
		},
	}
	ss := getStartupScriptPackageInstall(name, pkgManager, packageName)
	return newOsPolicyTestSetup(image, name, instanceName, testName, packageInstalled, machineType, ospa, ss, assertTimeout, wantCompliances)
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

func buildPkgRemoveTestSetup(name, image, pkgManager, key string) *osPolicyTestSetup {
	assertTimeout := 180 * time.Second
	testName := packageRemovalFunction
	packageName := "vim"
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		packageName = "certgen"
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
			{
				Id:   "remove-packages",
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "debian"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-debian",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_REMOVED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Apt{
											Apt: &osconfigpb.OSPolicy_Resource_PackageResource_APT{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "ubuntu"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-ubuntu",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_REMOVED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Apt{
											Apt: &osconfigpb.OSPolicy_Resource_PackageResource_APT{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "rhel"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-rhel",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_REMOVED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Yum{
											Yum: &osconfigpb.OSPolicy_Resource_PackageResource_YUM{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "centos"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-centos",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_REMOVED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Yum{
											Yum: &osconfigpb.OSPolicy_Resource_PackageResource_YUM{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "sles"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-sles",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_REMOVED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Zypper_{
											Zypper: &osconfigpb.OSPolicy_Resource_PackageResource_Zypper{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "windows"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-windows",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_REMOVED,
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

	var os string
	switch {
	case strings.Contains(image, "debian"):
		os = "debian"
	case strings.Contains(image, "windows"):
		os = "windows"
	case strings.Contains(image, "rhel"):
		os = "rhel"
	case strings.Contains(image, "centos"):
		os = "centos"
	case strings.Contains(image, "sles"):
		os = "sles"
	}
	wantCompliances := []*osconfigpb.InstanceOSPoliciesCompliance_OSPolicyCompliance{
		{
			OsPolicyId: "remove-packages",
			State:      osconfigpb.OSPolicyComplianceState_COMPLIANT,
			OsPolicyResourceCompliances: []*osconfigpb.OSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "remove-" + os,
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
			},
		},
	}
	ss := getStartupScriptPackageRemove(name, pkgManager, packageName)
	return newOsPolicyTestSetup(image, name, instanceName, testName, packageNotInstalled, machineType, ospa, ss, assertTimeout, wantCompliances)
}

func addPackageRemovalTest(key string) []*osPolicyTestSetup {
	var pkgTestSetup []*osPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgRemoveTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgRemoveTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgRemoveTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgRemoveTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func buildPkgInstallFromNewRepoTestSetup(name, image, pkgManager, key string) *osPolicyTestSetup {
	assertTimeout := 120 * time.Second
	packageName := "osconfig-agent-test"
	testName := packageInstallFromNewRepoFunction
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
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
			{
				Id:   "install-packages-from-repo",
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "debian"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-repo-debian",
								ResourceType: &osconfigpb.OSPolicy_Resource_Repository{
									Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource{
										Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource_Apt{
											Apt: &osconfigpb.OSPolicy_Resource_RepositoryResource_AptRepository{
												ArchiveType:  osconfigpb.OSPolicy_Resource_RepositoryResource_AptRepository_DEB,
												Uri:          aptTestRepoBaseURL,
												Distribution: osconfigTestRepo,
												Components:   []string{"main"},
												GpgKey:       aptRaptureGpgKey,
											},
										},
									},
								},
							},
							{
								Id: "install-package-debian",
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "ubuntu"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-repo-ubuntu",
								ResourceType: &osconfigpb.OSPolicy_Resource_Repository{
									Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource{
										Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource_Apt{
											Apt: &osconfigpb.OSPolicy_Resource_RepositoryResource_AptRepository{
												ArchiveType:  osconfigpb.OSPolicy_Resource_RepositoryResource_AptRepository_DEB,
												Uri:          aptTestRepoBaseURL,
												Distribution: osconfigTestRepo,
												Components:   []string{"main"},
												GpgKey:       aptRaptureGpgKey,
											},
										},
									},
								},
							},
							{
								Id: "install-package-ubuntu",
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "rhel"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-repo-rhel",
								ResourceType: &osconfigpb.OSPolicy_Resource_Repository{
									Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource{
										Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource_Yum{
											Yum: &osconfigpb.OSPolicy_Resource_RepositoryResource_YumRepository{
												Id:          osconfigTestRepo,
												DisplayName: "Google OSConfig Agent Test Repository",
												BaseUrl:     yumTestRepoBaseURL,
												GpgKeys:     yumRaptureGpgKeys,
											},
										},
									},
								},
							},
							{
								Id: "install-package-rhel",
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "centos"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-repo-centos",
								ResourceType: &osconfigpb.OSPolicy_Resource_Repository{
									Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource{
										Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource_Yum{
											Yum: &osconfigpb.OSPolicy_Resource_RepositoryResource_YumRepository{
												Id:          osconfigTestRepo,
												DisplayName: "Google OSConfig Agent Test Repository",
												BaseUrl:     yumTestRepoBaseURL,
												GpgKeys:     yumRaptureGpgKeys,
											},
										},
									},
								},
							},
							{
								Id: "install-package-centos",
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
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "sles"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-repo-sles",
								ResourceType: &osconfigpb.OSPolicy_Resource_Repository{
									Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource{
										Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource_Zypper{
											Zypper: &osconfigpb.OSPolicy_Resource_RepositoryResource_ZypperRepository{
												Id:          osconfigTestRepo,
												DisplayName: "Google OSConfig Agent Test Repository",
												BaseUrl:     yumTestRepoBaseURL,
												GpgKeys:     yumRaptureGpgKeys,
											},
										},
									},
								},
							},
							{
								Id: "install-package-sles",
								ResourceType: &osconfigpb.OSPolicy_Resource_Pkg{
									Pkg: &osconfigpb.OSPolicy_Resource_PackageResource{
										DesiredState: osconfigpb.OSPolicy_Resource_PackageResource_INSTALLED,
										SystemPackage: &osconfigpb.OSPolicy_Resource_PackageResource_Zypper_{
											Zypper: &osconfigpb.OSPolicy_Resource_PackageResource_Zypper{Name: packageName},
										},
									},
								},
							},
						},
					},
					{
						OsFilter: &osconfigpb.OSPolicy_OSFilter{OsShortName: "windows"},
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "install-windows-repo-windows",
								ResourceType: &osconfigpb.OSPolicy_Resource_Repository{
									Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource{
										Repository: &osconfigpb.OSPolicy_Resource_RepositoryResource_Goo{
											Goo: &osconfigpb.OSPolicy_Resource_RepositoryResource_GooRepository{
												Name: "Google OSConfig Agent Test Repository",
												Url:  gooTestRepoURL,
											},
										},
									},
								},
							},
							{
								Id: "install-package-windows",
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
	var os string
	switch {
	case strings.Contains(image, "debian"):
		os = "debian"
	case strings.Contains(image, "windows"):
		os = "windows"
	case strings.Contains(image, "rhel"):
		os = "rhel"
	case strings.Contains(image, "centos"):
		os = "centos"
	case strings.Contains(image, "sles"):
		os = "sles"
	}
	wantCompliances := []*osconfigpb.InstanceOSPoliciesCompliance_OSPolicyCompliance{
		{
			OsPolicyId: "install-packages-from-repo",
			State:      osconfigpb.OSPolicyComplianceState_COMPLIANT,
			OsPolicyResourceCompliances: []*osconfigpb.OSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "install-repo-" + os,
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
				{
					OsPolicyResourceId: "install-package-" + os,
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
			},
		},
	}
	ss := getStartupScriptPackageInstall(name, pkgManager, packageName)
	return newOsPolicyTestSetup(image, name, instanceName, testName, packageNotInstalled, machineType, ospa, ss, assertTimeout, wantCompliances)
}

func addPackageInstallFromNewRepoTest(key string) []*osPolicyTestSetup {
	var pkgTestSetup []*osPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallFromNewRepoTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallFromNewRepoTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallFromNewRepoTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgInstallFromNewRepoTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func buildFileRemoveTestSetup(name, image, pkgManager, key string) *osPolicyTestSetup {
	assertTimeout := 180 * time.Second
	testName := fileAbsentFunction
	machineType := "e2-standard-2"
	filePath := "/enforce_absent"

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
			{
				Id:   "remove-files",
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					{
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "remove-files",
								ResourceType: &osconfigpb.OSPolicy_Resource_File_{
									File: &osconfigpb.OSPolicy_Resource_FileResource{
										State: osconfigpb.OSPolicy_Resource_FileResource_ABSENT,
										Path:  filePath,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	wantCompliances := []*osconfigpb.InstanceOSPoliciesCompliance_OSPolicyCompliance{
		{
			OsPolicyId: "remove-files",
			State:      osconfigpb.OSPolicyComplianceState_COMPLIANT,
			OsPolicyResourceCompliances: []*osconfigpb.OSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "remove-files",
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
			},
		},
	}
	ss := getStartupScriptFileDNE(name, pkgManager, filePath)
	return newOsPolicyTestSetup(image, name, instanceName, testName, fileDNE, machineType, ospa, ss, assertTimeout, wantCompliances)
}

func addFileRemovalTest(key string) []*osPolicyTestSetup {
	var pkgTestSetup []*osPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildFileRemoveTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildFileRemoveTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildFileRemoveTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildFileRemoveTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func buildFilePresentTestSetup(name, image, pkgManager, key string) *osPolicyTestSetup {
	assertTimeout := 180 * time.Second
	testName := filePresentFunction
	machineType := "e2-standard-2"
	filePaths := []string{"/from_content", "/from_gcs", "/from_uri", "/from_local"}

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
			{
				Id:   "files-present",
				Mode: osconfigpb.OSPolicy_ENFORCEMENT,
				ResourceGroups: []*osconfigpb.OSPolicy_ResourceGroup{
					{
						Resources: []*osconfigpb.OSPolicy_Resource{
							{
								Id: "file-present-from-content",
								ResourceType: &osconfigpb.OSPolicy_Resource_File_{
									File: &osconfigpb.OSPolicy_Resource_FileResource{
										State:  osconfigpb.OSPolicy_Resource_FileResource_PRESENT,
										Path:   filePaths[0],
										Source: &osconfigpb.OSPolicy_Resource_FileResource_Content{Content: "something"},
									},
								},
							},
							{
								Id: "file-present-from-gcs",
								ResourceType: &osconfigpb.OSPolicy_Resource_File_{
									File: &osconfigpb.OSPolicy_Resource_FileResource{
										State: osconfigpb.OSPolicy_Resource_FileResource_PRESENT,
										Path:  filePaths[1],
										Source: &osconfigpb.OSPolicy_Resource_FileResource_File{
											File: &osconfigpb.OSPolicy_Resource_File{
												Type: &osconfigpb.OSPolicy_Resource_File_Gcs_{
													Gcs: &osconfigpb.OSPolicy_Resource_File_Gcs{
														Bucket:     testResourceBucket,
														Object:     "OSPolicies/test_file",
														Generation: 1617666133905437,
													},
												},
											},
										},
									},
								},
							},
							{
								Id: "file-present-from-uri",
								ResourceType: &osconfigpb.OSPolicy_Resource_File_{
									File: &osconfigpb.OSPolicy_Resource_FileResource{
										State: osconfigpb.OSPolicy_Resource_FileResource_PRESENT,
										Path:  filePaths[2],
										Source: &osconfigpb.OSPolicy_Resource_FileResource_File{
											File: &osconfigpb.OSPolicy_Resource_File{
												Type: &osconfigpb.OSPolicy_Resource_File_Remote_{
													Remote: &osconfigpb.OSPolicy_Resource_File_Remote{
														Uri:            "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/test_file",
														Sha256Checksum: "e0ef7229e64c61596d8be928397e19fcc542ac920c4132106fb1ec2295dd73d1",
													},
												},
											},
										},
									},
								},
							},
							{
								Id: "file-present-from-local",
								ResourceType: &osconfigpb.OSPolicy_Resource_File_{
									File: &osconfigpb.OSPolicy_Resource_FileResource{
										State: osconfigpb.OSPolicy_Resource_FileResource_PRESENT,
										Path:  filePaths[3],
										Source: &osconfigpb.OSPolicy_Resource_FileResource_File{
											File: &osconfigpb.OSPolicy_Resource_File{
												Type: &osconfigpb.OSPolicy_Resource_File_LocalPath{
													LocalPath: filePaths[0],
												},
											},
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
	wantCompliances := []*osconfigpb.InstanceOSPoliciesCompliance_OSPolicyCompliance{
		{
			OsPolicyId: "files-present",
			State:      osconfigpb.OSPolicyComplianceState_COMPLIANT,
			OsPolicyResourceCompliances: []*osconfigpb.OSPolicyResourceCompliance{
				{
					OsPolicyResourceId: "file-present-from-content",
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
				{
					OsPolicyResourceId: "file-present-from-gcs",
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
				{
					OsPolicyResourceId: "file-present-from-uri",
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
				{
					OsPolicyResourceId: "file-present-from-local",
					ConfigSteps: []*osconfigpb.OSPolicyResourceConfigStep{
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_VALIDATION,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
						{
							Type:    osconfigpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
							Outcome: osconfigpb.OSPolicyResourceConfigStep_SUCCEEDED,
						},
					},
					State: osconfigpb.OSPolicyComplianceState_COMPLIANT,
				},
			},
		},
	}
	ss := getStartupScriptFileExists(name, pkgManager, filePaths)
	return newOsPolicyTestSetup(image, name, instanceName, testName, fileExists, machineType, ospa, ss, assertTimeout, wantCompliances)
}

func addFilePresentTest(key string) []*osPolicyTestSetup {
	var pkgTestSetup []*osPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildFilePresentTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildFilePresentTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildFilePresentTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildFilePresentTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func generateAllTestSetup() []*osPolicyTestSetup {
	key := utils.RandString(3)

	pkgTestSetup := []*osPolicyTestSetup{}
	pkgTestSetup = append(pkgTestSetup, addPackageInstallTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addPackageRemovalTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addPackageInstallFromNewRepoTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addFileRemovalTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addFilePresentTest(key)...)
	return pkgTestSetup
}
