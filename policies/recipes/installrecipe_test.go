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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestInstallRecipe(t *testing.T) {
	ctx := t.Context()

	tarData := createTarArchive(t, []string{"file.txt"}).Bytes()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarData)
	}))
	t.Cleanup(ts.Close)

	tmpDir := t.TempDir()

	scriptPath := filepath.Join(tmpDir, "exec_test")
	if runtime.GOOS == "windows" {
		scriptPath += ".bat"
		os.WriteFile(scriptPath, []byte("exit 0"), 0755)
	} else {
		os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0"), 0755)
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T)
		recipe  *agentendpointpb.SoftwareRecipe
		wantDB  RecipeDB
		wantErr error
	}{
		{
			name: "fresh install, expect success",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe1",
				Version: "1.0.0",
			},
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "already installed same version, expect skip",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}})
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "1.0.0",
				DesiredState: agentendpointpb.DesiredState_INSTALLED,
			},
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "already installed older version but desired state INSTALLED, expect skip",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}})
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "2.0.0",
				DesiredState: agentendpointpb.DesiredState_INSTALLED,
			},
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "already installed needs update, expect success",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}})
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
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{2, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "multiple recipes in DB, expect target updated and others preserved",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, []Recipe{
					{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true},
					{Name: "recipe2", Version: recipeVersion{1, 0, 0}, Success: true},
				})
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
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{2, 0, 0}, Success: true},
				"recipe2": Recipe{Name: "recipe2", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "valid run script step, expect success",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
			},
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
			wantDB: RecipeDB{
				"recipe-script-success": Recipe{Name: "recipe-script-success", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "valid copy file step, expect success",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-copy-success",
				Version: "1.0.0",
				Artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
					{
						Id: "test-artifact",
						Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
							Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
								Uri: ts.URL,
							},
						},
					},
				},
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_FileCopy{
							FileCopy: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
								ArtifactId:  "test-artifact",
								Destination: filepath.Join(tmpDir, "dest.txt"),
							},
						},
					},
				},
			},
			wantDB: RecipeDB{
				"recipe-copy-success": Recipe{Name: "recipe-copy-success", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "valid exec file step, expect success",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-exec-success",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_FileExec{
							FileExec: &agentendpointpb.SoftwareRecipe_Step_ExecFile{
								LocationType: &agentendpointpb.SoftwareRecipe_Step_ExecFile_LocalPath{LocalPath: scriptPath},
							},
						},
					},
				},
			},
			wantDB: RecipeDB{
				"recipe-exec-success": Recipe{Name: "recipe-exec-success", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "valid extract archive step, expect success",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
				utiltest.OverrideVariable(t, &chown, func(file string, uid, gid int) error { return nil })
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-archive-success",
				Version: "1.0.0",
				Artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
					{
						Id: "test-archive-art",
						Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
							Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
								Uri: ts.URL,
							},
						},
					},
				},
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_ArchiveExtraction{
							ArchiveExtraction: &agentendpointpb.SoftwareRecipe_Step_ExtractArchive{
								ArtifactId:  "test-archive-art",
								Type:        agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR,
								Destination: filepath.Join(tmpDir, "archive_dest"),
							},
						},
					},
				},
			},
			wantDB: RecipeDB{
				"recipe-archive-success": Recipe{Name: "recipe-archive-success", Version: recipeVersion{1, 0, 0}, Success: true},
			},
			wantErr: nil,
		},
		{
			name: "step error, expect recipe recorded with success=false and error returned",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-script-failure",
				Version: "1.0.0",
				InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
					{
						Step: &agentendpointpb.SoftwareRecipe_Step_ScriptRun{
							ScriptRun: &agentendpointpb.SoftwareRecipe_Step_RunScript{
								Script:      "exit 1",
								Interpreter: agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL,
							},
						},
					},
				},
			},
			wantDB: RecipeDB{
				"recipe-script-failure": Recipe{Name: "recipe-script-failure", Version: recipeVersion{1, 0, 0}, Success: false},
			},
			wantErr: errors.New("error running step 0 (RunScript): exit status 1"),
		},
		{
			name: "fetch artifacts error, expect error and no db update",
			setup: func(t *testing.T) {
				setupTestDBFromRecipes(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe-artifact-error",
				Version: "1.0.0",
				Artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
					{
						Id: "no-artifact",
						Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
							Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
								Uri: "ftp://non-existent-host",
							},
						},
					},
				},
			},
			wantErr: errors.New("failed to obtain artifacts: error fetching artifact \"no-artifact\": error, unsupported protocol scheme ftp"),
			wantDB:  RecipeDB{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			gotErr := InstallRecipe(ctx, tt.recipe)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)

			gotDB := getTestDB(t)
			utiltest.AssertEquals(t, gotDB, tt.wantDB, cmpopts.IgnoreFields(Recipe{}, "InstallTime"))
		})
	}
}
