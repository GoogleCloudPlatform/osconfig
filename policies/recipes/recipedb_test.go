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

// setupTestDB creates a temporary directory for the test database.
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
			name: "empty JSON array, expect no error and empty db",
			setup: func(t *testing.T) {
				tmpDir := setupTestDB(t, nil)
				if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), []byte(`[]`), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			},
			wantErr: nil,
			wantDB:  RecipeDB{},
		},
		{
			name: "valid JSON, expect no error and db content",
			setup: func(t *testing.T) {
				tmpDir := setupTestDB(t, nil)
				if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), []byte(`[{"Name":"recipe1","Version":[1],"InstallTime":1,"Success":true},{"Name":"recipe2","Version":[2],"InstallTime":2,"Success":false}]`), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			},
			wantErr: nil,
			wantDB: RecipeDB{
				"recipe1": Recipe{Name: "recipe1", Version: recipeVersion{1}, InstallTime: 1, Success: true},
				"recipe2": Recipe{Name: "recipe2", Version: recipeVersion{2}, InstallTime: 2, Success: false},
			},
		},
		{
			name: "invalid JSON, expect syntax error",
			setup: func(t *testing.T) {
				tmpDir := setupTestDB(t, nil)
				if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), []byte(`invalid json`), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			},
			wantErr: func() error { var dummy interface{}; return json.Unmarshal([]byte(`invalid json`), &dummy) }(),
			wantDB:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			db, gotErr := newRecipeDB()
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, db, tt.wantDB)
		})
	}
}

func TestAddRecipe(t *testing.T) {
	existingRecipes := []Recipe{
		{Name: "existing-recipe-1", Version: recipeVersion{1, 0, 0}, InstallTime: 12345, Success: true},
		{Name: "existing-recipe-2", Version: recipeVersion{2, 0, 0}, InstallTime: 67890, Success: false},
	}

	tests := []struct {
		name           string
		setup          func(t *testing.T) RecipeDB
		initialRecipes []Recipe
		recipeName     string
		version        string
		success        bool
		wantErr        error
	}{
		{
			name: "empty DB, valid version, expect success",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, nil)
				db, err := newRecipeDB()
				if err != nil {
					t.Fatalf("failed to load recipe DB: %v", err)
				}
				return db
			},
			initialRecipes: nil,
			recipeName:     "test-recipe",
			version:        "1.2.3",
			success:        true,
			wantErr:        nil,
		},
		{
			name: "empty DB, invalid version, expect error",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, nil)
				db, err := newRecipeDB()
				if err != nil {
					t.Fatalf("failed to load recipe DB: %v", err)
				}
				return db
			},
			initialRecipes: nil,
			recipeName:     "bad-recipe",
			version:        "invalid.version",
			success:        false,
			wantErr:        errors.New("invalid Version string"),
		},
		{
			name: "non-empty DB, valid version, expect success and existing preserved",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, existingRecipes)
				db, err := newRecipeDB()
				if err != nil {
					t.Fatalf("failed to load recipe DB: %v", err)
				}
				return db
			},
			initialRecipes: existingRecipes,
			recipeName:     "test-recipe",
			version:        "1.2.3",
			success:        true,
			wantErr:        nil,
		},
		{
			name: "non-empty DB, invalid version, expect error and existing preserved",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, existingRecipes)
				db, err := newRecipeDB()
				if err != nil {
					t.Fatalf("failed to load recipe DB: %v", err)
				}
				return db
			},
			initialRecipes: existingRecipes,
			recipeName:     "bad-recipe",
			version:        "invalid.version",
			success:        false,
			wantErr:        errors.New("invalid Version string"),
		},
		{
			name: "non-empty DB, overwrite existing recipe, expect success",
			setup: func(t *testing.T) RecipeDB {
				setupTestDB(t, existingRecipes)
				db, err := newRecipeDB()
				if err != nil {
					t.Fatalf("failed to load recipe DB: %v", err)
				}
				return db
			},
			initialRecipes: existingRecipes,
			recipeName:     existingRecipes[0].Name,
			version:        "1.2.3",
			success:        true,
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := tt.setup(t)

			gotErr := db.addRecipe(tt.recipeName, tt.version, tt.success)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)

			// Verify initial entries are still there and unchanged (except the one being updated).
			for _, wantRecipe := range tt.initialRecipes {
				if wantRecipe.Name == tt.recipeName {
					continue
				}
				gotRecipe, ok := db.getRecipe(wantRecipe.Name)
				utiltest.AssertEquals(t, ok, true)
				utiltest.AssertEquals(t, gotRecipe, wantRecipe)
			}

			memRecipe, ok := db.getRecipe(tt.recipeName)
			if tt.wantErr != nil {
				utiltest.AssertEquals(t, ok, false)
				return
			}
			utiltest.AssertEquals(t, ok, true)
			utiltest.AssertEquals(t, memRecipe.Name, tt.recipeName)
			utiltest.AssertEquals(t, memRecipe.Version.String(), tt.version)
			utiltest.AssertEquals(t, memRecipe.Success, tt.success)

			// Verify file persistence.
			readDB, err := newRecipeDB()
			if err != nil {
				t.Fatalf("failed to load recipe DB from file: %v", err)
			}
			utiltest.AssertEquals(t, db, readDB)
		})
	}
}
