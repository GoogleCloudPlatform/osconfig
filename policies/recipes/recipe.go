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

type recipeVersion []int

func (v recipeVersion) String() string {
	res := fmt.Sprintf("%d", v[0])
	for _, val := range v[1:] {
		res = fmt.Sprintf("%s.%d", res, val)
	}
	return res
}

// Recipe represents an installed recipe.
type Recipe struct {
	Name        string
	Version     recipeVersion
	InstallTime int64
	Success     bool
}

func (r *Recipe) setVersion(version string) error {
	var err error
	r.Version, err = convertVersion(version)
	return err
}

// compare returns true if the provided Version is greater than the recipe's
// Version, false otherwise.
func (r *Recipe) compare(version string) bool {
	if version == "" {
		return false
	}
	cVersion, err := convertVersion(version)
	if err != nil {
		return false
	}
	if len(r.Version) > len(cVersion) {
		topad := len(r.Version) - len(cVersion)
		for i := 0; i < topad; i++ {
			cVersion = append(cVersion, 0)
		}
	} else {
		topad := len(cVersion) - len(r.Version)
		for i := 0; i < topad; i++ {
			r.Version = append(r.Version, 0)
		}
	}
	for i := 0; i < len(r.Version); i++ {
		if r.Version[i] != cVersion[i] {
			return cVersion[i] > r.Version[i]
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
			return nil, fmt.Errorf("invalid Version string")
		}
		val, err := strconv.ParseUint(element, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid Version string")
		}
		ret = append(ret, int(val))
	}
	return ret, nil
}
