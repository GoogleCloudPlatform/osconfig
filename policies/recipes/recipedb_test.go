//  Copyright 2026 Google Inc. All Rights Reserved.
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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// setupTestDB sets up a temp directory for the test db and initializes it with the given content.
func setupTestDB(t *testing.T, content []byte) {
	t.Helper()
	tmpDir := t.TempDir()
	utiltest.OverrideVariable(t, &dbDirWindows, tmpDir)
	utiltest.OverrideVariable(t, &dbDirUnix, tmpDir)

	if content != nil {
		if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), content, 0644); err != nil {
			t.Fatalf("failed to write recipes to file: %v", err)
		}
	}
}

func setupTestDBFromRecipes(t *testing.T, recipes []Recipe) {
	t.Helper()
	content, err := json.Marshal(recipes)
	if err != nil {
		t.Fatalf("failed to marshal recipes: %v", err)
	}
	setupTestDB(t, content)
}

func getTestDB(t *testing.T) RecipeDB {
	t.Helper()
	db, err := newRecipeDB()
	if err != nil {
		t.Fatalf("failed to load recipe DB: %v", err)
	}
	return db
}

func TestNewRecipeDB(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr error
		wantDB  RecipeDB
	}{
		{
			name:    "no file exists, expect no error and empty db",
			setup:   func(t *testing.T) { setupTestDB(t, nil) },
			wantErr: nil,
			wantDB:  RecipeDB{},
		},
		{
			name:    "empty JSON array, expect no error and empty db",
			setup:   func(t *testing.T) { setupTestDB(t, []byte(`[]`)) },
			wantErr: nil,
			wantDB:  RecipeDB{},
		},
		{
			name: "valid JSON, expect no error and db content",
			setup: func(t *testing.T) {
				setupTestDB(t, []byte(`[{"Name":"recipe1","Version":[1],"InstallTime":1,"Success":true},{"Name":"recipe2","Version":[2],"InstallTime":2,"Success":false}]`))
			},
			wantErr: nil,
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{1}, InstallTime: 1, Success: true},
				"recipe2": Recipe{Name: "recipe2", Version: recipeVersion{2}, InstallTime: 2, Success: false},
			},
		},
		{
			name:    "invalid JSON, expect syntax error",
			setup:   func(t *testing.T) { setupTestDB(t, []byte(`invalid json`)) },
			wantErr: func() error { var dummy any; return json.Unmarshal([]byte(`invalid json`), &dummy) }(),
			wantDB:  nil,
		},
		{
			name: "one valid recipe and malformed entry, expect syntax error",
			setup: func(t *testing.T) {
				setupTestDB(t, []byte(`[{"Name":"recipe1","Version":[1],"InstallTime":1,"Success":true}, invalid json]`))
			},
			wantErr: func() error {
				var dummy any
				return json.Unmarshal([]byte(`[{"Name":"recipe1","Version":[1],"InstallTime":1,"Success":true}, invalid json]`), &dummy)
			}(),
			wantDB: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			gotDB, gotErr := newRecipeDB()
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotDB, tt.wantDB)
		})
	}
}

func setInstallTime(db RecipeDB, recipeName string, installTime int64) RecipeDB {
	if recipe, ok := db[recipeName]; ok {
		recipe.InstallTime = installTime
		db[recipeName] = recipe
	}
	return db
}

func TestAddRecipe(t *testing.T) {
	existingRecipes := []Recipe{
		{Name: "existing-recipe-1", Version: recipeVersion{1, 0, 0}, InstallTime: 12345, Success: true},
		{Name: "existing-recipe-2", Version: recipeVersion{2, 0, 0}, InstallTime: 67890, Success: false},
	}

	tests := []struct {
		name          string
		setup         func(t *testing.T) RecipeDB
		recipeName    string
		recipeVersion string
		recipeSuccess bool
		wantErr       error
		wantDB        RecipeDB
	}{
		{
			name: "empty DB, valid version, expect new recipe added",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, nil)
				return getTestDB(t)
			},
			recipeName:    "test-recipe",
			recipeVersion: "1.2.3",
			recipeSuccess: true,
			wantErr:       nil,
			wantDB: RecipeDB{
				"test-recipe": {Name: "test-recipe", Version: recipeVersion{1, 2, 3}, Success: true},
			},
		},
		{
			name: "non-empty DB, valid version, expect new recipe added and existing preserved",
			setup: func(t *testing.T) RecipeDB {
				setupTestDBFromRecipes(t, existingRecipes)
				return getTestDB(t)
			},
			recipeName:    "test-recipe",
			recipeVersion: "1.2.3",
			recipeSuccess: true,
			wantErr:       nil,
			wantDB: RecipeDB{
				"existing-recipe-1": existingRecipes[0],
				"existing-recipe-2": existingRecipes[1],
				"test-recipe":       {Name: "test-recipe", Version: recipeVersion{1, 2, 3}, Success: true},
			},
		},
		{
			name: "non-empty DB, overwrite existing recipe, expect recipe updated and others preserved",
			setup: func(t *testing.T) RecipeDB {
				setupTestDBFromRecipes(t, existingRecipes)
				return getTestDB(t)
			},
			recipeName:    existingRecipes[0].Name,
			recipeVersion: "1.2.3",
			recipeSuccess: true,
			wantErr:       nil,
			wantDB: RecipeDB{
				"existing-recipe-1": {Name: "existing-recipe-1", Version: recipeVersion{1, 2, 3}, Success: true},
				"existing-recipe-2": existingRecipes[1],
			},
		},
		{
			name: "empty DB, invalid version, expect version error and no recipe added",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, nil)
				return getTestDB(t)
			},
			recipeName:    "bad-recipe",
			recipeVersion: "invalid.version",
			recipeSuccess: false,
			wantErr:       errors.New("invalid Version string"),
			wantDB:        RecipeDB{},
		},
		{
			name: "non-empty DB, invalid version, expect version error and existing preserved",
			setup: func(t *testing.T) RecipeDB {
				setupTestDBFromRecipes(t, existingRecipes)
				return getTestDB(t)
			},
			recipeName:    "bad-recipe",
			recipeVersion: "invalid.version",
			recipeSuccess: false,
			wantErr:       errors.New("invalid Version string"),
			wantDB: RecipeDB{
				"existing-recipe-1": existingRecipes[0],
				"existing-recipe-2": existingRecipes[1],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := tt.setup(t)

			gotErr := db.addRecipe(tt.recipeName, tt.recipeVersion, tt.recipeSuccess)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)

			// Sync dynamic InstallTime timestamp before comparing DB states.
			tt.wantDB = setInstallTime(tt.wantDB, tt.recipeName, db[tt.recipeName].InstallTime)
			utiltest.AssertEquals(t, db, tt.wantDB)
		})
	}
}
