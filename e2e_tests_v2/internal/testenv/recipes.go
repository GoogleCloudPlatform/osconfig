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

package testenv

import (
	"errors"
	"fmt"
	"strings"

	osconfigv1beta "google.golang.org/api/osconfig/v1beta"
)

const testResourceBucket = "osconfig-agent-end2end-test-resources"

// BuildRecipeInstallPolicy builds a GuestPolicy containing an empty SoftwareRecipe with the given name.
func BuildRecipeInstallPolicy(recipeName string) (*osconfigv1beta.GuestPolicy, error) {
	if strings.TrimSpace(recipeName) == "" {
		return nil, errors.New("recipe name is required")
	}
	return &osconfigv1beta.GuestPolicy{
		Recipes: []*osconfigv1beta.SoftwareRecipe{
			{
				Name: recipeName,
			},
		},
	}, nil
}

// BuildRecipeStepsPolicy builds a GuestPolicy containing a SoftwareRecipe with artifacts and steps tailored for pkgManager.
func BuildRecipeStepsPolicy(recipeName, pkgManager string) (*osconfigv1beta.GuestPolicy, error) {
	if strings.TrimSpace(recipeName) == "" {
		return nil, errors.New("recipe name is required")
	}

	artifacts := recipeTestArtifacts()
	pm := strings.ToLower(strings.TrimSpace(pkgManager))

	switch pm {
	case "googet":
		return &osconfigv1beta.GuestPolicy{
			Recipes: []*osconfigv1beta.SoftwareRecipe{
				{
					Name:         recipeName,
					Artifacts:    artifacts,
					InstallSteps: windowsRecipeSteps(),
				},
			},
		}, nil
	case "cos":
		return &osconfigv1beta.GuestPolicy{
			Recipes: []*osconfigv1beta.SoftwareRecipe{
				{
					Name:         recipeName,
					Artifacts:    artifacts,
					InstallSteps: baseLinuxRecipeSteps(),
				},
			},
		}, nil
	case "apt":
		steps := append(baseLinuxRecipeSteps(),
			unspecifiedInterpreterScriptStep(),
			execShStep(),
			&osconfigv1beta.SoftwareRecipeStep{
				DpkgInstallation: &osconfigv1beta.SoftwareRecipeStepInstallDpkg{
					ArtifactId: "dpkg-test",
				},
			},
		)
		return &osconfigv1beta.GuestPolicy{
			Recipes: []*osconfigv1beta.SoftwareRecipe{
				{
					Name:         recipeName,
					Artifacts:    artifacts,
					InstallSteps: steps,
				},
			},
		}, nil
	case "yum", "zypper":
		steps := append(baseLinuxRecipeSteps(),
			unspecifiedInterpreterScriptStep(),
			execShStep(),
			&osconfigv1beta.SoftwareRecipeStep{
				RpmInstallation: &osconfigv1beta.SoftwareRecipeStepInstallRpm{
					ArtifactId: "rpm-test",
				},
			},
		)
		return &osconfigv1beta.GuestPolicy{
			Recipes: []*osconfigv1beta.SoftwareRecipe{
				{
					Name:         recipeName,
					Artifacts:    artifacts,
					InstallSteps: steps,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported package manager: %q", pkgManager)
	}
}

func recipeTestArtifacts() []*osconfigv1beta.SoftwareRecipeArtifact {
	return []*osconfigv1beta.SoftwareRecipeArtifact{
		{
			AllowInsecure: true,
			Id:            "copy-test",
			Remote: &osconfigv1beta.SoftwareRecipeArtifactRemote{
				Uri: "https://example.com",
			},
		},
		{
			AllowInsecure: true,
			Id:            "exec-test-sh",
			Gcs: &osconfigv1beta.SoftwareRecipeArtifactGcs{
				Bucket: testResourceBucket,
				Object: "software_recipes/exec_test.sh",
			},
		},
		{
			AllowInsecure: true,
			Id:            "exec-test-cmd",
			Gcs: &osconfigv1beta.SoftwareRecipeArtifactGcs{
				Bucket: testResourceBucket,
				Object: "software_recipes/exec_test.cmd",
			},
		},
		{
			AllowInsecure: true,
			Id:            "tar-test",
			Gcs: &osconfigv1beta.SoftwareRecipeArtifactGcs{
				Bucket: testResourceBucket,
				Object: "software_recipes/tar_test.tar.gz",
			},
		},
		{
			AllowInsecure: true,
			Id:            "zip-test",
			Gcs: &osconfigv1beta.SoftwareRecipeArtifactGcs{
				Bucket: testResourceBucket,
				Object: "software_recipes/zip_test.zip",
			},
		},
		{
			AllowInsecure: true,
			Id:            "dpkg-test",
			Gcs: &osconfigv1beta.SoftwareRecipeArtifactGcs{
				Bucket: testResourceBucket,
				Object: "software_recipes/ed_1.15-1_amd64.deb",
			},
		},
		{
			AllowInsecure: true,
			Id:            "rpm-test",
			Gcs: &osconfigv1beta.SoftwareRecipeArtifactGcs{
				Bucket: testResourceBucket,
				Object: "software_recipes/ed-1.1-3.3.el6.x86_64.rpm",
			},
		},
	}
}

func baseLinuxRecipeSteps() []*osconfigv1beta.SoftwareRecipeStep {
	return []*osconfigv1beta.SoftwareRecipeStep{
		{
			ScriptRun: &osconfigv1beta.SoftwareRecipeStepRunScript{
				Script:      "echo 'hello world' > /tmp/osconfig-SoftwareRecipe_Step_RunScript_SHELL",
				Interpreter: "SHELL",
			},
		},
		{
			FileCopy: &osconfigv1beta.SoftwareRecipeStepCopyFile{
				ArtifactId:  "copy-test",
				Destination: "/tmp/osconfig-copy-test",
			},
		},
		{
			ArchiveExtraction: &osconfigv1beta.SoftwareRecipeStepExtractArchive{
				ArtifactId:  "tar-test",
				Destination: "/tmp/tar-test",
				Type:        "TAR_GZIP",
			},
		},
		{
			ArchiveExtraction: &osconfigv1beta.SoftwareRecipeStepExtractArchive{
				ArtifactId:  "zip-test",
				Destination: "/tmp/zip-test",
				Type:        "ZIP",
			},
		},
	}
}

func unspecifiedInterpreterScriptStep() *osconfigv1beta.SoftwareRecipeStep {
	return &osconfigv1beta.SoftwareRecipeStep{
		ScriptRun: &osconfigv1beta.SoftwareRecipeStepRunScript{
			Script:      "#!/bin/sh\necho 'hello world' > /tmp/osconfig-SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED",
			Interpreter: "INTERPRETER_UNSPECIFIED",
		},
	}
}

func execShStep() *osconfigv1beta.SoftwareRecipeStep {
	return &osconfigv1beta.SoftwareRecipeStep{
		FileExec: &osconfigv1beta.SoftwareRecipeStepExecFile{
			ArtifactId: "exec-test-sh",
		},
	}
}

func windowsRecipeSteps() []*osconfigv1beta.SoftwareRecipeStep {
	return []*osconfigv1beta.SoftwareRecipeStep{
		{
			ScriptRun: &osconfigv1beta.SoftwareRecipeStepRunScript{
				Script:      "echo 'hello world' > c:\\osconfig-SoftwareRecipe_Step_RunScript_POWERSHELL",
				Interpreter: "POWERSHELL",
			},
		},
		{
			ScriptRun: &osconfigv1beta.SoftwareRecipeStepRunScript{
				Script:      "echo 'hello world' > c:\\osconfig-SoftwareRecipe_Step_RunScript_SHELL",
				Interpreter: "SHELL",
			},
		},
		{
			FileExec: &osconfigv1beta.SoftwareRecipeStepExecFile{
				ArtifactId: "exec-test-cmd",
			},
		},
		{
			FileCopy: &osconfigv1beta.SoftwareRecipeStepCopyFile{
				ArtifactId:  "copy-test",
				Destination: "c:\\osconfig-copy-test",
			},
		},
		{
			ArchiveExtraction: &osconfigv1beta.SoftwareRecipeStepExtractArchive{
				ArtifactId:  "tar-test",
				Destination: "c:\\tar-test",
				Type:        "TAR_GZIP",
			},
		},
		{
			ArchiveExtraction: &osconfigv1beta.SoftwareRecipeStepExtractArchive{
				ArtifactId:  "zip-test",
				Destination: "c:\\zip-test",
				Type:        "ZIP",
			},
		},
	}
}
