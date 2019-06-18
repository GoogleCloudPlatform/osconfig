package recipes

import (
	"fmt"
	"strconv"
	"strings"
)

type RecipeDB struct{}

func newRecipeDB() RecipeDB {
	return RecipeDB{}
}

func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	return Recipe{}, false
}

func (db *RecipeDB) AddRecipe(name, version string) error {
	return nil
}

type Recipe struct {
	version []int
}

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
			return nil, fmt.Errorf("Invalid version string")
		}
		val, err := strconv.ParseUint(element, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("Invalid version string")
		}
		ret = append(ret, int(val))
	}
	return ret, nil
}
