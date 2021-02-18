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
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	osconfigserver "github.com/GoogleCloudPlatform/osconfig/e2e_tests/osconfig_server"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/utils"
	"github.com/golang/protobuf/jsonpb"

	osconfigpb "google.golang.org/genproto/googleapis/cloud/osconfig/v1beta"
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

func buildPkgInstallTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 120 * time.Second
	testName := packageInstallFunction
	packageName := "ed"
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		packageName = "cowsay"
		machineType = "e2-standard-4"
	}
	if strings.Contains(image, "rhel-6") || strings.Contains(image, "centos-6") {
		packageName = "cowsay"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	gp := &osconfigpb.GuestPolicy{
		Packages:   osconfigserver.BuildPackagePolicy([]string{packageName}, nil, nil),
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
	}
	ss := getStartupScript(name, pkgManager, packageName)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, gp, ss, assertTimeout)
}

func addPackageInstallTest(key string) []*guestPolicyTestSetup {
	var pkgTestSetup []*guestPolicyTestSetup
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

func buildPkgUpdateTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 240 * time.Second
	testName := packageUpdateFunction
	packageName := "ed"
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		packageName = "cowsay"
		machineType = "e2-standard-4"
	}
	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	gp := &osconfigpb.GuestPolicy{
		Packages:   osconfigserver.BuildPackagePolicy(nil, nil, []string{packageName}),
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
	}
	ss := getUpdateStartupScript(name, pkgManager)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageNotInstalled, machineType, gp, ss, assertTimeout)
}

func addPackageUpdateTest(key string) []*guestPolicyTestSetup {
	var pkgTestSetup []*guestPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgUpdateTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgUpdateTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgUpdateTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgUpdateTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func buildPkgDoesNotUpdateTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 240 * time.Second
	testName := packageNoUpdateFunction
	packageName := "ed"
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		packageName = "cowsay"
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	gp := &osconfigpb.GuestPolicy{
		Packages:   osconfigserver.BuildPackagePolicy([]string{packageName}, nil, nil),
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
	}
	ss := getUpdateStartupScript(name, pkgManager)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, gp, ss, assertTimeout)
}

func addPackageDoesNotUpdateTest(key string) []*guestPolicyTestSetup {
	var pkgTestSetup []*guestPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgDoesNotUpdateTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgDoesNotUpdateTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgDoesNotUpdateTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		pkgTestSetup = append(pkgTestSetup, buildPkgDoesNotUpdateTestSetup(name, image, "googet", key))
	}
	return pkgTestSetup
}

func buildPkgRemoveTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 180 * time.Second
	testName := packageRemovalFunction
	packageName := "vim"
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		packageName = "certgen"
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	gp := &osconfigpb.GuestPolicy{
		Packages:   osconfigserver.BuildPackagePolicy(nil, []string{packageName}, nil),
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
	}
	ss := getStartupScript(name, pkgManager, packageName)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageNotInstalled, machineType, gp, ss, assertTimeout)
}

func addPackageRemovalTest(key string) []*guestPolicyTestSetup {
	var pkgTestSetup []*guestPolicyTestSetup
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

func buildPkgInstallFromNewRepoTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 120 * time.Second
	packageName := "osconfig-agent-test"
	testName := packageInstallFromNewRepoFunction
	machineType := "e2-standard-2"
	if pkgManager == "googet" {
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	gp := &osconfigpb.GuestPolicy{
		// Test that upgrade also installs.
		Packages:   osconfigserver.BuildPackagePolicy(nil, nil, []string{packageName}),
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
		PackageRepositories: []*osconfigpb.PackageRepository{
			{Repository: osconfigserver.BuildAptRepository(osconfigpb.AptRepository_DEB, aptTestRepoBaseURL, osconfigTestRepo, aptRaptureGpgKey, []string{"main"})},
			{Repository: osconfigserver.BuildYumRepository(osconfigTestRepo, "Google OSConfig Agent Test Repository", yumTestRepoBaseURL, yumRaptureGpgKeys)},
			{Repository: osconfigserver.BuildZypperRepository(osconfigTestRepo, "Google OSConfig Agent Test Repository", yumTestRepoBaseURL, yumRaptureGpgKeys)},
			{Repository: osconfigserver.BuildGooRepository("Google OSConfig Agent Test Repository", gooTestRepoURL)},
		},
	}
	ss := getStartupScript(name, pkgManager, packageName)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, gp, ss, assertTimeout)
}

func addPackageInstallFromNewRepoTest(key string) []*guestPolicyTestSetup {
	var pkgTestSetup []*guestPolicyTestSetup
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

func addRecipeInstallTest(key string) []*guestPolicyTestSetup {
	var recipeTestSetup []*guestPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeInstallTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeInstallTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeInstallTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeInstallTestSetup(name, image, "googet", key))
	}
	// This ensures we only run cos tests on the "head image" tests.
	if config.AgentRepo() == "" {
		for name, image := range utils.HeadCOSImages {
			recipeTestSetup = append(recipeTestSetup, buildRecipeInstallTestSetup(name, image, "cos", key))
		}
	}
	return recipeTestSetup
}

func addMetadataPolicyTest(key string) []*guestPolicyTestSetup {
	var policyTestSetup []*guestPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		policyTestSetup = append(policyTestSetup, buildMetadataPolicyTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		policyTestSetup = append(policyTestSetup, buildMetadataPolicyTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		policyTestSetup = append(policyTestSetup, buildMetadataPolicyTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		policyTestSetup = append(policyTestSetup, buildMetadataPolicyTestSetup(name, image, "googet", key))
	}
	// This ensures we only run cos tests on the "head image" tests.
	if config.AgentRepo() == "" {
		for name, image := range utils.HeadCOSImages {
			policyTestSetup = append(policyTestSetup, buildMetadataPolicyTestSetup(name, image, "cos", key))
		}
	}
	return policyTestSetup
}

func buildRecipeInstallTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 120 * time.Second
	testName := recipeInstallFunction
	recipeName := "testrecipe"
	machineType := "e2-standard-2"
	if strings.HasPrefix(image, "windows") {
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	gp := &osconfigpb.GuestPolicy{
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
		Recipes: []*osconfigpb.SoftwareRecipe{
			osconfigserver.BuildSoftwareRecipe(recipeName, "", nil, nil),
		},
	}
	ss := getRecipeInstallStartupScript(name, recipeName, pkgManager)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, gp, ss, assertTimeout)
}

func addRecipeStepsTest(key string) []*guestPolicyTestSetup {
	var recipeTestSetup []*guestPolicyTestSetup
	for name, image := range utils.HeadAptImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeStepsTestSetup(name, image, "apt", key))
	}
	for name, image := range utils.HeadELImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeStepsTestSetup(name, image, "yum", key))
	}
	for name, image := range utils.HeadSUSEImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeStepsTestSetup(name, image, "zypper", key))
	}
	for name, image := range utils.HeadWindowsImages {
		recipeTestSetup = append(recipeTestSetup, buildRecipeStepsTestSetup(name, image, "googet", key))
	}
	// This ensures we only run cos tests on the "head image" tests.
	if config.AgentRepo() == "" {
		for name, image := range utils.HeadCOSImages {
			recipeTestSetup = append(recipeTestSetup, buildRecipeStepsTestSetup(name, image, "cos", key))
		}
	}
	return recipeTestSetup
}

func buildRecipeStepsTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 120 * time.Second
	testName := recipeStepsFunction
	recipeName := "testrecipe"
	machineType := "e2-standard-2"
	if strings.HasPrefix(image, "windows") {
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		{
			AllowInsecure: true,
			Id:            "copy-test",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Remote_{
				Remote: &osconfigpb.SoftwareRecipe_Artifact_Remote{
					Uri: "https://example.com",
				},
			},
		},
		{
			AllowInsecure: true,
			Id:            "exec-test-sh",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Gcs_{
				Gcs: &osconfigpb.SoftwareRecipe_Artifact_Gcs{
					Bucket: testResourceBucket,
					Object: "software_recipes/exec_test.sh",
				},
			},
		},
		{
			AllowInsecure: true,
			Id:            "exec-test-cmd",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Gcs_{
				Gcs: &osconfigpb.SoftwareRecipe_Artifact_Gcs{
					Bucket: testResourceBucket,
					Object: "software_recipes/exec_test.cmd",
				},
			},
		},
		{
			AllowInsecure: true,
			Id:            "tar-test",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Gcs_{
				Gcs: &osconfigpb.SoftwareRecipe_Artifact_Gcs{
					Bucket: testResourceBucket,
					Object: "software_recipes/tar_test.tar.gz",
				},
			},
		},
		{
			AllowInsecure: true,
			Id:            "zip-test",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Gcs_{
				Gcs: &osconfigpb.SoftwareRecipe_Artifact_Gcs{
					Bucket: testResourceBucket,
					Object: "software_recipes/zip_test.zip",
				},
			},
		},
		{
			AllowInsecure: true,
			Id:            "dpkg-test",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Gcs_{
				Gcs: &osconfigpb.SoftwareRecipe_Artifact_Gcs{
					Bucket: testResourceBucket,
					Object: "software_recipes/ed_1.15-1_amd64.deb",
				},
			},
		},
		{
			AllowInsecure: true,
			Id:            "rpm-test",
			Artifact: &osconfigpb.SoftwareRecipe_Artifact_Gcs_{
				Gcs: &osconfigpb.SoftwareRecipe_Artifact_Gcs{
					Bucket: testResourceBucket,
					Object: "software_recipes/ed-1.1-3.3.el6.x86_64.rpm",
				},
			},
		},
	}

	pkgTest := &osconfigpb.SoftwareRecipe_Step{}
	switch pkgManager {
	case "apt":
		pkgTest = &osconfigpb.SoftwareRecipe_Step{Step: &osconfigpb.SoftwareRecipe_Step_DpkgInstallation{
			DpkgInstallation: &osconfigpb.SoftwareRecipe_Step_InstallDpkg{ArtifactId: "dpkg-test"},
		}}
	case "yum", "zypper":
		pkgTest = &osconfigpb.SoftwareRecipe_Step{Step: &osconfigpb.SoftwareRecipe_Step_RpmInstallation{
			RpmInstallation: &osconfigpb.SoftwareRecipe_Step_InstallRpm{ArtifactId: "rpm-test"},
		}}
	}

	gp := &osconfigpb.GuestPolicy{
		Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
		Recipes: []*osconfigpb.SoftwareRecipe{
			osconfigserver.BuildSoftwareRecipe(recipeName, "", artifacts,
				[]*osconfigpb.SoftwareRecipe_Step{
					&osconfigpb.SoftwareRecipe_Step{Step: &osconfigpb.SoftwareRecipe_Step_ScriptRun{
						ScriptRun: &osconfigpb.SoftwareRecipe_Step_RunScript{
							Script:      "echo 'hello world' > /tmp/osconfig-SoftwareRecipe_Step_RunScript_SHELL",
							Interpreter: osconfigpb.SoftwareRecipe_Step_RunScript_SHELL,
						},
					}},

					&osconfigpb.SoftwareRecipe_Step{Step: &osconfigpb.SoftwareRecipe_Step_FileCopy{
						FileCopy: &osconfigpb.SoftwareRecipe_Step_CopyFile{ArtifactId: "copy-test", Destination: "/tmp/osconfig-copy-test"},
					}},
					{Step: &osconfigpb.SoftwareRecipe_Step_ArchiveExtraction{
						ArchiveExtraction: &osconfigpb.SoftwareRecipe_Step_ExtractArchive{ArtifactId: "tar-test", Destination: "/tmp/tar-test", Type: osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP},
					}},
					{Step: &osconfigpb.SoftwareRecipe_Step_ArchiveExtraction{
						ArchiveExtraction: &osconfigpb.SoftwareRecipe_Step_ExtractArchive{ArtifactId: "zip-test", Destination: "/tmp/zip-test", Type: osconfigpb.SoftwareRecipe_Step_ExtractArchive_ZIP},
					}},
				},
			),
		},
	}
	// COS can not create files with the executable bit set on the root partition.
	if pkgManager != "cos" {
		gp.Recipes[0].InstallSteps = append(gp.Recipes[0].InstallSteps, &osconfigpb.SoftwareRecipe_Step{Step: &osconfigpb.SoftwareRecipe_Step_ScriptRun{
			ScriptRun: &osconfigpb.SoftwareRecipe_Step_RunScript{
				Script:      "#!/bin/sh\necho 'hello world' > /tmp/osconfig-SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED",
				Interpreter: osconfigpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED,
			},
		}}, &osconfigpb.SoftwareRecipe_Step{Step: &osconfigpb.SoftwareRecipe_Step_FileExec{
			FileExec: &osconfigpb.SoftwareRecipe_Step_ExecFile{LocationType: &osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId{ArtifactId: "exec-test-sh"}},
		}}, pkgTest)
	}

	if pkgManager == "googet" {
		gp = &osconfigpb.GuestPolicy{
			Assignment: &osconfigpb.Assignment{InstanceNamePrefixes: []string{instanceName}},
			Recipes: []*osconfigpb.SoftwareRecipe{
				osconfigserver.BuildSoftwareRecipe(recipeName, "", artifacts,
					[]*osconfigpb.SoftwareRecipe_Step{
						{Step: &osconfigpb.SoftwareRecipe_Step_ScriptRun{
							ScriptRun: &osconfigpb.SoftwareRecipe_Step_RunScript{
								Script:      "echo 'hello world' > c:\\osconfig-SoftwareRecipe_Step_RunScript_POWERSHELL",
								Interpreter: osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL,
							},
						}},
						{Step: &osconfigpb.SoftwareRecipe_Step_ScriptRun{
							ScriptRun: &osconfigpb.SoftwareRecipe_Step_RunScript{
								Script:      "echo 'hello world' > c:\\osconfig-SoftwareRecipe_Step_RunScript_SHELL",
								Interpreter: osconfigpb.SoftwareRecipe_Step_RunScript_SHELL,
							},
						}},
						{Step: &osconfigpb.SoftwareRecipe_Step_FileExec{
							FileExec: &osconfigpb.SoftwareRecipe_Step_ExecFile{LocationType: &osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId{ArtifactId: "exec-test-cmd"}},
						}},
						{Step: &osconfigpb.SoftwareRecipe_Step_FileCopy{
							FileCopy: &osconfigpb.SoftwareRecipe_Step_CopyFile{ArtifactId: "copy-test", Destination: "c:\\osconfig-copy-test"},
						}},
						{Step: &osconfigpb.SoftwareRecipe_Step_ArchiveExtraction{
							ArchiveExtraction: &osconfigpb.SoftwareRecipe_Step_ExtractArchive{ArtifactId: "tar-test", Destination: "c:\\tar-test", Type: osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP},
						}},
						{Step: &osconfigpb.SoftwareRecipe_Step_ArchiveExtraction{
							ArchiveExtraction: &osconfigpb.SoftwareRecipe_Step_ExtractArchive{ArtifactId: "zip-test", Destination: "c:\\zip-test", Type: osconfigpb.SoftwareRecipe_Step_ExtractArchive_ZIP},
						}},
					},
				),
			},
		}
	}

	ss := getRecipeStepsStartupScript(name, recipeName, pkgManager)
	return newGuestPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, gp, ss, assertTimeout)
}

func buildMetadataPolicyTestSetup(name, image, pkgManager, key string) *guestPolicyTestSetup {
	assertTimeout := 60 * time.Second
	testName := metadataPolicyFunction
	recipeName := "testrecipe"
	machineType := "e2-standard-2"
	if strings.HasPrefix(image, "windows") {
		machineType = "e2-standard-4"
	}

	instanceName := fmt.Sprintf("%s-%s-%s-%s", path.Base(name), testName, key, utils.RandString(3))

	ss := getRecipeInstallStartupScript(name, recipeName, pkgManager)
	ts := newGuestPolicyTestSetup(image, instanceName, testName, packageInstalled, machineType, nil, ss, assertTimeout)

	marshaler := jsonpb.Marshaler{}
	recipeString, err := marshaler.MarshalToString(osconfigserver.BuildSoftwareRecipe(recipeName, "", nil, nil))
	if err != nil {
		// An error in the test setup means something seriously wrong.
		panic(err)
	}
	rec := fmt.Sprintf(`{"softwareRecipes": [%s]}`, recipeString)
	ts.mdPolicy = compute.BuildInstanceMetadataItem("gce-software-declaration", rec)
	return ts

}

func generateAllTestSetup() []*guestPolicyTestSetup {
	key := utils.RandString(3)

	pkgTestSetup := []*guestPolicyTestSetup{}
	pkgTestSetup = append(pkgTestSetup, addPackageInstallTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addPackageRemovalTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addPackageInstallFromNewRepoTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addPackageUpdateTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addPackageDoesNotUpdateTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addRecipeInstallTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addRecipeStepsTest(key)...)
	pkgTestSetup = append(pkgTestSetup, addMetadataPolicyTest(key)...)
	return pkgTestSetup
}
