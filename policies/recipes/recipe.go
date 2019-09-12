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

// A Recipe represents one recipe installed on the system.
type Recipe struct {
	Name        string `json:"Name,omitempty"`
	Version     []int  `json:"Version,omitempty"`
	InstallTime int64  `json:"install_time,omitempty"`
}

// SetVersion sets the Version on a Recipe.
func (r *Recipe) SetVersion(version string) error {
	var err error
	r.Version, err = convertVersion(version)
	return err
}

// Greater returns true if the provided Version is greater than the recipe's
// Version, false otherwise.
func (r *Recipe) Greater(version string) bool {
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

// GetVersionString return the version string of recipe
func (r *Recipe) GetVersionString() string {
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(r.Version[0]))
	for i := 1; i < len(r.Version); i++ {
		sb.Write([]byte(fmt.Sprintf(".%s", strconv.Itoa(r.Version[i]))))
	}
	return sb.String()
}
