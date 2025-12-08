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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

var (
	dbDirWindows = "C:\\ProgramData\\Google"
	dbDirUnix    = "/var/lib/google"
	dbFileName   = "osconfig_recipedb"
)

type timeFunc func() time.Time

// RecipeDB represents local state of installed recipes.
type recipeDB struct {
	file     string
	timeFunc timeFunc

	recipes map[string]Recipe
}

func newRecipeDB(path string) (*recipeDB, error) {
	db := &recipeDB{
		file:     path,
		timeFunc: time.Now,
		recipes:  make(map[string]Recipe, 0),
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil
		}

		return nil, err
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var recipes []Recipe
	if err := json.Unmarshal(raw, &recipes); err != nil {
		return nil, err
	}

	for _, recipe := range recipes {
		db.recipes[recipe.Name] = recipe
	}
	return db, nil
}

// newRecipeDB instantiates a recipeDB.
func newRecipeDBWithDefaults() (*recipeDB, error) {
	dir, fileName := getDbDir(), dbFileName
	return newRecipeDB(filepath.Join(dir, fileName))
}

// getRecipe returns the Recipe object for the given recipe name.
func (db *recipeDB) getRecipe(name string) (Recipe, bool) {
	r, ok := db.recipes[name]
	return r, ok
}

// addRecipe marks a recipe as installed.
func (db *recipeDB) addRecipe(name, version string, success bool) error {
	versionNum, err := convertVersion(version)
	if err != nil {
		return err
	}
	db.recipes[name] = Recipe{Name: name, Version: versionNum, InstallTime: db.timeFunc().Unix(), Success: success}

	return db.saveToFS()
}

func (db *recipeDB) saveToFS() error {
	var recipes []Recipe
	for _, recipe := range db.recipes {
		recipes = append(recipes, recipe)
	}

	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].Name < recipes[j].Name
	})

	raw, err := json.Marshal(recipes)
	if err != nil {
		return err
	}

	dir := filepath.Dir(db.file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fileName := filepath.Base(db.file)
	f, err := ioutil.TempFile(dir, fileName+"_*")
	if err != nil {
		return err
	}

	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), db.file)
}

func getDbDir() string {
	if runtime.GOOS == "windows" {
		return dbDirWindows
	}
	return dbDirUnix
}
