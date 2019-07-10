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

import osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"

// FetchArtifacts is a stub.
func FetchArtifacts(recipeArtifacts []*osconfigpb.SoftwareRecipe_Artifact) (map[string]string, error) {
	artifacts := make(map[string]string)
	for _, artifact := range recipeArtifacts {
		artifacts[artifact.Id] = artifact.Uri
	}
	return artifacts, nil
}
