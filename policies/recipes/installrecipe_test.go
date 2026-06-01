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
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func setupTestDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	utiltest.OverrideVariable(t, &dbDirWindows, tmpDir)
	utiltest.OverrideVariable(t, &dbDirUnix, tmpDir)

	return tmpDir
}

type initRecipe struct {
	name    string
	version string
	success bool
}

func populateInitialRecipes(t *testing.T, initialRecipes []initRecipe) {
	t.Helper()
	if len(initialRecipes) == 0 {
		return
	}
	db, err := newRecipeDB()
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	for _, r := range initialRecipes {
		if err := db.addRecipe(r.name, r.version, r.success); err != nil {
			t.Fatalf("failed to add initial recipe: %v", err)
		}
	}
}

func getRecipeFromDB(t *testing.T, recipeName string) Recipe {
	t.Helper()
	db, err := newRecipeDB()
	if err != nil {
		t.Fatalf("failed to read db after install: %v", err)
	}
	r, ok := db.getRecipe(recipeName)
	if !ok {
		t.Fatalf("recipe %q not found in DB", recipeName)
	}
	return r
}

func TestInstallRecipeDesiredStateHandling(t *testing.T) {
	ctx := context.Background()

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
			setupTestDB(t)
			populateInitialRecipes(t, tt.initialRecipes)

			err := InstallRecipe(ctx, tt.recipe)
			utiltest.AssertErrorMatch(t, err, nil)

			r := getRecipeFromDB(t, tt.recipe.Name)
			utiltest.AssertEquals(t, r.Success, true)
			utiltest.AssertEquals(t, r.Version.String(), tt.wantVersion)
		})
	}
}
