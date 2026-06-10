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
func setupTestDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	utiltest.OverrideVariable(t, &dbDirWindows, tmpDir)
	utiltest.OverrideVariable(t, &dbDirUnix, tmpDir)

	return tmpDir
}

func TestNewRecipeDB(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T)
		wantErr  error
		wantSize int
	}{
		{
			name:     "no file exists, expect no error and empty db",
			setup:    func(t *testing.T) { setupTestDB(t) },
			wantErr:  nil,
			wantSize: 0,
		},
		{
			name: "valid JSON, expect no error and db size 2",
			setup: func(t *testing.T) {
				tmpDir := setupTestDB(t)
				if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), []byte(`[{"Name":"recipe1","Version":[1],"InstallTime":1,"Success":true},{"Name":"recipe2","Version":[2],"InstallTime":2,"Success":false}]`), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			},
			wantErr:  nil,
			wantSize: 2,
		},
		{
			name: "invalid JSON, expect syntax error",
			setup: func(t *testing.T) {
				tmpDir := setupTestDB(t)
				if err := os.WriteFile(filepath.Join(tmpDir, dbFileName), []byte(`invalid json`), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			},
			wantErr:  func() error { var dummy interface{}; return json.Unmarshal([]byte(`invalid json`), &dummy) }(),
			wantSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			db, gotErr := newRecipeDB()
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, len(db), tt.wantSize)
		})
	}
}

func TestAddRecipe_InMemory(t *testing.T) {
	tests := []struct {
		name       string
		recipeName string
		version    string
		success    bool
		wantErr    error
	}{
		{
			name:       "valid version, expect success",
			recipeName: "test-recipe",
			version:    "1.2.3",
			success:    true,
			wantErr:    nil,
		},
		{
			name:       "invalid version, expect invalid Version string error",
			recipeName: "bad-recipe",
			version:    "invalid.version",
			success:    false,
			wantErr:    errors.New("invalid Version string"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestDB(t)
			db := make(RecipeDB)
			gotErr := db.addRecipe(tt.recipeName, tt.version, tt.success)
			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)

			memRecipe, ok := db.getRecipe(tt.recipeName)
			utiltest.AssertEquals(t, ok, true)
			utiltest.AssertEquals(t, memRecipe.Name, tt.recipeName)
			utiltest.AssertEquals(t, memRecipe.Version.String(), tt.version)
			utiltest.AssertEquals(t, memRecipe.Success, tt.success)
		})
	}
}

func TestAddRecipe_FromFile(t *testing.T) {
	setupTestDB(t)
	db := make(RecipeDB)
	err := db.addRecipe("test-recipe", "1.2.3", true)
	utiltest.AssertErrorMatch(t, err, nil)
	memRecipe, ok := db.getRecipe("test-recipe")

	readDB, err := newRecipeDB()
	utiltest.AssertErrorMatch(t, err, nil)
	fileRecipe, ok := readDB.getRecipe("test-recipe")
	utiltest.AssertEquals(t, ok, true)
	utiltest.AssertEquals(t, memRecipe, fileRecipe)
}
