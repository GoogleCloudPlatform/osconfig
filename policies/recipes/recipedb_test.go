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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// setupTestDB creates a temporary directory for the test database.
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

func TestNewRecipeDB(t *testing.T) {
	tmpDir, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name        string
		fileContent string
		wantErr     error
		wantSize    int
	}{
		{
			name:        "no file exists, expect no error and empty db",
			fileContent: "",
			wantErr:     nil,
			wantSize:    0,
		},
		{
			name:        "valid JSON, expect no error and db size 2",
			fileContent: `[{"Name":"recipe1","Version":[1],"InstallTime":1,"Success":true},{"Name":"recipe2","Version":[2],"InstallTime":2,"Success":false}]`,
			wantErr:     nil,
			wantSize:    2,
		},
		{
			name:        "invalid JSON, expect syntax error",
			fileContent: `invalid json`,
			wantErr:     func() error { var dummy interface{}; return json.Unmarshal([]byte(`invalid json`), &dummy) }(),
			wantSize:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbFilePath := filepath.Join(tmpDir, dbFileName)
			os.Remove(dbFilePath)

			if tt.fileContent != "" {
				if err := ioutil.WriteFile(dbFilePath, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			}

			db, err := newRecipeDB()
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			utiltest.AssertEquals(t, len(db), tt.wantSize)
		})
	}
}

func TestAddRecipe(t *testing.T) {
	_, cleanup := setupTestDB(t)
	defer cleanup()

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
			name:       "invalid version, expect error",
			recipeName: "bad-recipe",
			version:    "invalid.version",
			success:    false,
			wantErr:    errors.New("invalid Version string"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := make(RecipeDB)
			err := db.addRecipe(tt.recipeName, tt.version, tt.success)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			if tt.wantErr != nil {
				return
			}
			r, ok := db.getRecipe(tt.recipeName)
			utiltest.AssertEquals(t, ok, true)
			utiltest.AssertEquals(t, r.Name, tt.recipeName)
			utiltest.AssertEquals(t, r.Version.String(), tt.version)
			utiltest.AssertEquals(t, r.Success, tt.success)

			readDB, err := newRecipeDB()
			if err != nil {
				t.Fatalf("failed to read db file: %v", err)
			}

			readRecipe, ok := readDB.getRecipe(tt.recipeName)
			utiltest.AssertEquals(t, ok, true)
			utiltest.AssertEquals(t, readRecipe, r)
		})
	}
}
