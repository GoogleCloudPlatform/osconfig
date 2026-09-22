//  Copyright 2026 Google LLC
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

package testenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	osconfigv1beta "google.golang.org/api/osconfig/v1beta"
)

// BuildMetadataPolicyJSON builds the JSON value for the gce-software-declaration metadata key.
func BuildMetadataPolicyJSON(recipeName string) (string, error) {
	if strings.TrimSpace(recipeName) == "" {
		return "", errors.New("recipe name is required")
	}
	payload := struct {
		SoftwareRecipes []*osconfigv1beta.SoftwareRecipe `json:"softwareRecipes"`
	}{
		SoftwareRecipes: []*osconfigv1beta.SoftwareRecipe{
			{
				Name: recipeName,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal software declaration JSON: %w", err)
	}
	return string(data), nil
}
