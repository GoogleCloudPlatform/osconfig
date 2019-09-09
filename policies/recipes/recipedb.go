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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	dbDirWindows = "C:\\ProgramData\\Google"
	dbDirUnix    = "/var/lib/google"
	dbFileName   = "osconfig_recipedb"
)

// RecipeDB represents local state of installed Recipes.
type RecipeDB struct {
	Recipes map[string]Recipe `json:"recipes,omitempty"`
}

func newRecipeDB() (*RecipeDB, error) {
	f, err := os.Open(filepath.Join(getDbDir(), dbFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return &RecipeDB{Recipes: make(map[string]Recipe)}, nil
		}
		return nil, err
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	db := &RecipeDB{Recipes: make(map[string]Recipe)}
	if err := json.Unmarshal(bytes, &db); err != nil {
		return nil, errors.New(fmt.Sprintf("Json Unmarshalling error: %s", err.Error()))
	}
	return db, nil
}

// GetRecipe returns the Recipe object for the given recipe Name.
func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	r, ok := db.Recipes[name]
	return r, ok
}

// AddRecipe marks a recipe as installed.
func (db *RecipeDB) AddRecipe(name, version string) error {
	versionNum, err := convertVersion(version)
	if err != nil {
		return err
	}
	rcp := Recipe{Name: name, Version: versionNum, InstallTime: time.Now().Unix()}
	db.Recipes[name] = rcp
	dbBytes, err := json.Marshal(db)
	if err != nil {
		return err
	}

	dbDir := getDbDir()
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	f, err := ioutil.TempFile(dbDir, dbFileName+"_*")
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(dbBytes); err != nil {
		return err
	}

	return os.Rename(f.Name(), filepath.Join(dbDir, dbFileName))
}

func getDbDir() string {
	if runtime.GOOS == "windows" {
		return dbDirWindows
	}
	return dbDirUnix
}
