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

// RecipeDB represents local state of installed recipes.
type RecipeDB map[string]Recipe

// newRecipeDB instantiates a recipeDB.
func newRecipeDB() (RecipeDB, error) {
	db := make(RecipeDB)
	f, err := os.Open(filepath.Join(getDbDir(), dbFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil
		}
		return nil, err
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var recipelist []Recipe
	if err := json.Unmarshal(bytes, &recipelist); err != nil {
		return nil, err
	}
	for _, recipe := range recipelist {
		db[recipe.Name] = recipe
	}
	return db, nil
}

// getRecipe returns the Recipe object for the given recipe name.
func (db RecipeDB) getRecipe(name string) (Recipe, bool) {
	r, ok := db[name]
	return r, ok
}

// addRecipe marks a recipe as installed.
func (db RecipeDB) addRecipe(name, version string, success bool) error {
	versionNum, err := convertVersion(version)
	if err != nil {
		return err
	}
	db[name] = Recipe{Name: name, Version: versionNum, InstallTime: time.Now().Unix(), Success: success}

	var recipelist []Recipe
	for _, recipe := range db {
		recipelist = append(recipelist, recipe)
	}
	dbBytes, err := json.Marshal(recipelist)
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

	if _, err := f.Write(dbBytes); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
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
