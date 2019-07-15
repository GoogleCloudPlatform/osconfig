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
	"fmt"
	"strconv"
	"strings"
)

// RecipeDB represents local state of installed recipes.
type RecipeDB struct{}

func newRecipeDB() RecipeDB {
	return RecipeDB{}
}

// GetRecipe returns the Recipe object for the given recipe name.
func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	return Recipe{}, false
}

// AddRecipe marks a recipe as installed.
func (db *RecipeDB) AddRecipe(name, version string) error {
	return nil
}

// A Recipe represents one recipe installed on the system.
type Recipe struct {
	name    string
	version []int
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
