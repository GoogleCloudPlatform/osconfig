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

package recipes

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func setupTestDB(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := ioutil.TempDir("", "recipedb_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origWindows := dbDirWindows
	origUnix := dbDirUnix
	dbDirWindows = tmpDir
	dbDirUnix = tmpDir

	cleanup := func() {
		os.RemoveAll(tmpDir)
		dbDirWindows = origWindows
		dbDirUnix = origUnix
	}
	return tmpDir, cleanup
}

func TestInstallRecipeDesiredStateHandling(t *testing.T) {
	ctx := context.Background()

	type initRecipe struct {
		name    string
		version string
		success bool
	}

	tests := []struct {
		name           string
		initialRecipes []initRecipe
		recipe         *agentendpointpb.SoftwareRecipe
		wantVersion    string
	}{
		{
			name: "fresh install, expect success",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe1",
				Version: "1.0.0",
			},
			wantVersion: "1.0.0",
		},
		{
			name: "already installed same version, expect skip",
			initialRecipes: []initRecipe{
				{name: "recipe1", version: "1.0.0", success: true},
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "1.0.0",
				DesiredState: agentendpointpb.DesiredState_INSTALLED,
			},
			wantVersion: "1.0.0",
		},
		{
			name: "already installed older version but desired state INSTALLED, expect skip",
			initialRecipes: []initRecipe{
				{name: "recipe1", version: "1.0.0", success: true},
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "2.0.0",
				DesiredState: agentendpointpb.DesiredState_INSTALLED,
			},
			wantVersion: "1.0.0",
		},
		{
			name: "already installed needs update, expect success",
			initialRecipes: []initRecipe{
				{name: "recipe1", version: "1.0.0", success: true},
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "2.0.0",
				DesiredState: agentendpointpb.DesiredState_UPDATED,
				UpdateSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_ScriptRun{
							ScriptRun: &agentendpointpb.SoftwareRecipe_Step_RunScript{
								Script:      "echo 'update'",
								Interpreter: agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL,
							},
						},
					},
				},
			},
			wantVersion: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup := setupTestDB(t)
			defer cleanup()

			if len(tt.initialRecipes) > 0 {
				db, err := newRecipeDB()
				if err != nil {
					t.Fatalf("failed to init db: %v", err)
				}
				for _, r := range tt.initialRecipes {
					if err := db.addRecipe(r.name, r.version, r.success); err != nil {
						t.Fatalf("failed to add initial recipe: %v", err)
					}
				}
			}

			err := InstallRecipe(ctx, tt.recipe)

			db, err := newRecipeDB()
			if err != nil {
				t.Fatalf("failed to read db after install: %v", err)
			}

			r, ok := db.getRecipe(tt.recipe.Name)
			if !ok {
				t.Fatalf("recipe %q not found in DB", tt.recipe.Name)
			}

			utiltest.AssertEquals(t, r.Success, true)
			utiltest.AssertEquals(t, r.Version.String(), tt.wantVersion)
		})
	}
}

func TestInstallRecipeStepExecution(t *testing.T) {
	ctx := context.Background()

	expectedMsiErr := errors.New(`error running step 0 (InstallMsi): SoftwareRecipe_Step_InstallMsi only applicable on Windows`)
	if runtime.GOOS == "windows" {
		expectedMsiErr = errors.New(`error running step 0 (InstallMsi): "missing-msi" not found in artifact map`)
	}

	tests := []struct {
		name        string
		recipe      *agentendpointpb.SoftwareRecipe
		setupFunc   func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe)
		wantErr     error
		wantSuccess bool
		wantVersion string
	}{
		{
			name: "valid run script step, expect success",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-script-success",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_ScriptRun{
							ScriptRun: &agentendpointpb.SoftwareRecipe_Step_RunScript{
								Script:      "echo 'success'",
								Interpreter: agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL,
							},
						},
					},
				},
			},
			wantErr:     nil,
			wantSuccess: true,
			wantVersion: "1.0.0",
		},
		{
			name: "valid copy file step, expect success",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-copy-success",
				Version: "1.0.0",
				Artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
					{
						Id: "test-artifact",
						Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
							Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{},
						},
					},
				},
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_FileCopy{
							FileCopy: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
								ArtifactId: "test-artifact",
							},
						},
					},
				},
			},
			setupFunc: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("artifact content"))
				}))
				t.Cleanup(ts.Close)

				recipe.Artifacts[0].GetRemote().Uri = ts.URL
				recipe.InstallSteps[0].GetFileCopy().Destination = filepath.Join(t.TempDir(), "dest.txt")
			},
			wantErr:     nil,
			wantSuccess: true,
			wantVersion: "1.0.0",
		},
		{
			name: "valid exec file step, expect success",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-exec-success",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_FileExec{
							FileExec: &agentendpointpb.SoftwareRecipe_Step_ExecFile{
								LocationType: &agentendpointpb.SoftwareRecipe_Step_ExecFile_LocalPath{},
							},
						},
					},
				},
			},
			setupFunc: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				scriptPath := filepath.Join(t.TempDir(), "exec_test")
				if runtime.GOOS == "windows" {
					scriptPath += ".bat"
					os.WriteFile(scriptPath, []byte("exit 0"), 0755)
				} else {
					os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0"), 0755)
				}
				recipe.InstallSteps[0].GetFileExec().LocationType = &agentendpointpb.SoftwareRecipe_Step_ExecFile_LocalPath{LocalPath: scriptPath}
			},
			wantErr:     nil,
			wantSuccess: true,
			wantVersion: "1.0.0",
		},
		{
			name: "extract archive step with non-existing artifact, expect error and marked failed",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-archive",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_ArchiveExtraction{
							ArchiveExtraction: &agentendpointpb.SoftwareRecipe_Step_ExtractArchive{
								ArtifactId: "missing-archive",
							},
						},
					},
				},
			},
			wantErr:     errors.New(`error running step 0 (ExtractArchive): "missing-archive" not found in artifact map`),
			wantSuccess: false,
			wantVersion: "1.0.0",
		},
		{
			name: "dpkg install step with non-existing artifact, expect error and marked failed",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-dpkg",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_DpkgInstallation{
							DpkgInstallation: &agentendpointpb.SoftwareRecipe_Step_InstallDpkg{
								ArtifactId: "missing-dpkg",
							},
						},
					},
				},
			},
			setupFunc: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				utiltest.OverrideVariable(t, &packages.DpkgExists, true)
			},
			wantErr:     errors.New(`error running step 0 (InstallDpkg): "missing-dpkg" not found in artifact map`),
			wantSuccess: false,
			wantVersion: "1.0.0",
		},
		{
			name: "rpm install step with non-existing artifact, expect error and marked failed",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-rpm",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_RpmInstallation{
							RpmInstallation: &agentendpointpb.SoftwareRecipe_Step_InstallRpm{
								ArtifactId: "missing-rpm",
							},
						},
					},
				},
			},
			setupFunc: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				utiltest.OverrideVariable(t, &packages.RPMExists, true)
			},
			wantErr:     errors.New(`error running step 0 (InstallRpm): "missing-rpm" not found in artifact map`),
			wantSuccess: false,
			wantVersion: "1.0.0",
		},
		{
			name: "msi install step with non-existing artifact or unsupported OS, expect error and marked failed",
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-msi",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_MsiInstallation{
							MsiInstallation: &agentendpointpb.SoftwareRecipe_Step_InstallMsi{
								ArtifactId: "missing-msi",
							},
						},
					},
				},
			},
			wantErr:     expectedMsiErr,
			wantSuccess: false,
			wantVersion: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc(t, tt.recipe)
			}

			_, cleanup := setupTestDB(t)
			defer cleanup()

			err := InstallRecipe(ctx, tt.recipe)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			db, err := newRecipeDB()
			if err != nil {
				t.Fatalf("failed to read db after install: %v", err)
			}

			r, ok := db.getRecipe(tt.recipe.Name)
			if !ok {
				t.Fatalf("recipe %q not found in DB", tt.recipe.Name)
			}

			utiltest.AssertEquals(t, r.Success, tt.wantSuccess)
			utiltest.AssertEquals(t, r.Version.String(), tt.wantVersion)
		})
	}
}
