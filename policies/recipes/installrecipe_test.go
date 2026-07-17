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
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestInstallRecipe(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name   string
		setup  func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe)
		recipe *agentendpointpb.SoftwareRecipe
		wantDB RecipeDB
	}{
		{
			name: "fresh install, expect success",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
				setupTestDBFromRecipes(t, nil)
			},
			recipe: &agentendpointpb.SoftwareRecipe{
				Name:    "recipe1",
				Version: "1.0.0",
			},
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{1, 0, 0}, Success: true},
			},
		},
		{
			name: "already installed same version, expect skip",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
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
		},
		{
			name: "already installed older version but desired state INSTALLED, expect skip",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
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
		},
		{
			name: "already installed needs update, expect success",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
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
		},
		{
			name: "multiple recipes in DB, expect target updated and others preserved",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
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
		},
		{
			name: "valid run script step, expect success",
			setup: func(t *testing.T, recipe *agentendpointpb.SoftwareRecipe) {
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t, tt.recipe)

			gotErr := InstallRecipe(ctx, tt.recipe)
			utiltest.AssertErrorMatch(t, gotErr, nil)

			gotDB := getTestDB(t)
			utiltest.AssertEquals(t, gotDB, tt.wantDB, cmpopts.IgnoreFields(Recipe{}, "InstallTime"))
		})
	}
}
