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

// recipeDB represents local state of installed recipes.
type recipeDB struct {
	recipes map[string]recipe
}

func newRecipeDB() (*recipeDB, error) {
	f, err := os.Open(filepath.Join(getDbDir(), dbFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return &recipeDB{recipes: make(map[string]recipe)}, nil
		}
		return nil, err
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	db := &recipeDB{}
	if err := json.Unmarshal(bytes, &db); err != nil {
		return nil, fmt.Errorf("Json Unmarshalling error: %s", err.Error())
	}
	return db, nil
}

// getRecipe returns the Recipe object for the given recipe name.
func (db *recipeDB) getRecipe(name string) (recipe, bool) {
	r, ok := db.recipes[name]
	return r, ok
}

// addRecipe marks a recipe as installed.
func (db *recipeDB) addRecipe(name, version string, success bool) error {
	versionNum, err := convertVersion(version)
	if err != nil {
		return err
	}
	db.recipes[name] = recipe{name: name, version: versionNum, installTime: time.Now().Unix(), success: success}
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
