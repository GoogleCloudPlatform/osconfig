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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func setupTestDB(t *testing.T, recipes []Recipe) string {
	t.Helper()
	tmpDir := t.TempDir()

	utiltest.OverrideVariable(t, &dbDirWindows, tmpDir)
	utiltest.OverrideVariable(t, &dbDirUnix, tmpDir)

	if len(recipes) > 0 {
		dbBytes, err := json.Marshal(recipes)
		if err != nil {
			t.Fatalf("failed to marshal recipes: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), dbBytes, 0644); err != nil {
			t.Fatalf("failed to write recipes to file: %v", err)
		}
	}
	return tmpDir
}

func TestInstallRecipeDesiredStateHandling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe)
		recipe      *agentendpointpb.SoftwareRecipe
		wantRecipes []Recipe
	}{
		{
			name: "fresh install, expect success",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDB(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe1",
				Version: "1.0.0",
			},
			wantRecipes: []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}},
		},
		{
			name: "already installed same version, expect skip",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDB(t, []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}})
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "1.0.0",
				DesiredState: agentendpointpb.DesiredState_INSTALLED,
			},
			wantRecipes: []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}},
		},
		{
			name: "already installed older version but desired state INSTALLED, expect skip",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDB(t, []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}})
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:         "recipe1",
				Version:      "2.0.0",
				DesiredState: agentendpointpb.DesiredState_INSTALLED,
			},
			wantRecipes: []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}},
		},
		{
			name: "already installed needs update, expect success",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDB(t, []Recipe{{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true}})
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
			wantRecipes: []Recipe{{Name: "recipe1", Version: recipeVersion{2, 0, 0}, Success: true}},
		},
		{
			name: "multiple recipes in DB, expect target updated and others preserved",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDB(t, []Recipe{
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
			wantRecipes: []Recipe{
				{Name: "recipe1", Version: recipeVersion{2, 0, 0}, Success: true},
				{Name: "recipe2", Version: recipeVersion{1, 0, 0}, Success: true},
			},
		},
		{
			name: "valid run script step, expect success",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDB(t, nil)
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
			wantRecipes: []Recipe{{Name: "recipe-script-success", Version: recipeVersion{1, 0, 0}, Success: true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t, tt.recipe)

			gotErr := InstallRecipe(ctx, tt.recipe)
			utiltest.AssertErrorMatch(t, gotErr, nil)

			// Load the recipe database and assert that it contains only expected recipes.
			db, err := newRecipeDB()
			utiltest.AssertErrorMatch(t, err, nil)
			utiltest.AssertEquals(t, len(db), len(tt.wantRecipes))

			for _, wantRecipe := range tt.wantRecipes {
				recipe, ok := db.getRecipe(wantRecipe.Name)
				utiltest.AssertEquals(t, ok, true)
				utiltest.AssertEquals(t, recipe.Success, wantRecipe.Success)
				utiltest.AssertEquals(t, recipe.Version.String(), wantRecipe.Version.String())
			}
		})
	}
}
