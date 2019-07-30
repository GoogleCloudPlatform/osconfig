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
	"strconv"
	"strings"
	"time"
)

var (
	dbDirWindows = "C:\\ProgramData\\Google"
	dbDirUnix    = "/var/lib/google"
	dbFileName   = "osconfig_recipedb"
)

// RecipeDB represents local state of installed recipes.
type RecipeDB struct {
	recipes map[string]Recipe
}

func newRecipeDB() (*RecipeDB, error) {
	f, err := os.Open(filepath.Join(getDbDir(), dbFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return &RecipeDB{recipes: make(map[string]Recipe)}, nil
		}
		return nil, err
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	db := &RecipeDB{}
	if err := json.Unmarshal(bytes, &db); err != nil {
		return nil, err
	}
	return db, nil
}

// GetRecipe returns the Recipe object for the given recipe name.
func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	r, ok := db.recipes[name]
	return r, ok
}

// AddRecipe marks a recipe as installed.
func (db *RecipeDB) AddRecipe(name, version string) error {
	versionNum, err := convertVersion(version)
	if err != nil {
		return err
	}
	db.recipes[name] = Recipe{name: name, version: versionNum, installTime: time.Now().Unix()}
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
	if runtime.GOOS != "windows" {
		return dbDirWindows
	}
	return dbDirUnix
}

// A Recipe represents one recipe installed on the system.
type Recipe struct {
	name        string
	version     []int
	installTime int64
}

// SetVersion sets the version on a Recipe.
func (r *Recipe) SetVersion(version string) error {
	var err error
	r.version, err = convertVersion(version)
	return err
}

// Greater returns true if the provided version is greater than the recipe's
// version, false otherwise.
func (r *Recipe) Greater(version string) bool {
	if version == "" {
		return false
	}
	cVersion, err := convertVersion(version)
	if err != nil {
		return false
	}
	if len(r.version) > len(cVersion) {
		topad := len(r.version) - len(cVersion)
		for i := 0; i < topad; i++ {
			cVersion = append(cVersion, 0)
		}
	} else {
		topad := len(cVersion) - len(r.version)
		for i := 0; i < topad; i++ {
			r.version = append(r.version, 0)
		}
	}
	for i := 0; i < len(r.version); i++ {
		if r.version[i] != cVersion[i] {
			return cVersion[i] > r.version[i]
		}
	}
	return false
}

func convertVersion(version string) ([]int, error) {
	// ${ROOT}/recipe[_ver]/runId/recipe.yaml  // recipe at time of application
	// ${ROOT}/recipe[_ver]/runId/artifacts/*
	// ${ROOT}/recipe[_ver]/runId/stepN_type/
	if version == "" {
		return []int{0}, nil
	}
	var ret []int
	for idx, element := range strings.Split(version, ".") {
		if idx > 3 {
			return nil, fmt.Errorf("invalid version string")
		}
		val, err := strconv.ParseUint(element, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid version string")
		}
		ret = append(ret, int(val))
	}
	return ret, nil
}
