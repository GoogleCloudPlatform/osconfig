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
//

package policies

import (
	"bytes"
	"encoding/json"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// localConfig represents the structure of the config to the JSON parser.
//
// The types of members of the struct are wrappers for protobufs and delegate
// the parsing to jsonpb lib via their UnmarshalJSON implementations.
type localConfig struct {
	Packages            []pkg
	PackageRepositories []packageRepository
	SoftwareRecipes     []softwareRecipe
}

type pkg struct {
	i agentendpointpb.Package
}

func (r *pkg) UnmarshalJSON(b []byte) error {
	rd := bytes.NewReader(b)
	return jsonpb.Unmarshal(rd, &r.i)
}

type packageRepository struct {
	i agentendpointpb.PackageRepository
}

func (r *packageRepository) UnmarshalJSON(b []byte) error {
	rd := bytes.NewReader(b)
	return jsonpb.Unmarshal(rd, &r.i)
}

type softwareRecipe struct {
	r agentendpointpb.SoftwareRecipe
}

func (r *softwareRecipe) UnmarshalJSON(b []byte) error {
	rd := bytes.NewReader(b)
	return jsonpb.Unmarshal(rd, &r.r)
}

func parseLocalConfig(a []byte) (*localConfig, error) {
	var lc localConfig
	err := json.Unmarshal(a, &lc)
	if err != nil {
		return nil, err
	}
	return &lc, nil
}

// GetId returns a repository Id that is used to group repositories for
// override by higher priotiry policy(-ies).
// For repositories that have no such Id, GetId returns "", in which
// case the repository is never overridden.
func getID(repo agentendpointpb.PackageRepository) string {
	switch repo.Repository.(type) {
	case *agentendpointpb.PackageRepository_Yum:
		return "yum-" + repo.GetYum().GetId()
	case *agentendpointpb.PackageRepository_Zypper:
		return "zypper-" + repo.GetZypper().GetId()
	default:
		return ""
	}
}

// MergeConfigs merges the local config with the lookup response, giving priority to the lookup
// result.
func mergeConfigs(local *localConfig, resp agentendpointpb.LookupEffectiveGuestPoliciesResponse) agentendpointpb.LookupEffectiveGuestPoliciesResponse {
	if local == nil {
		return resp
	}
	// Ids that are in the maps below
	repos := make(map[string]bool)
	pkgs := make(map[string]bool)
	recipes := make(map[string]bool)

	for _, v := range resp.GetPackages() {
		pkgs[v.Package.Name] = true
	}
	for _, v := range resp.GetPackageRepositories() {
		if id := getID(*v.PackageRepository); id != "" {
			repos[id] = true
		}
	}
	for _, v := range resp.GetSoftwareRecipes() {
		recipes[v.SoftwareRecipe.Name] = true
	}
	for _, v := range local.Packages {
		if _, ok := pkgs[v.i.Name]; !ok {
			sp := new(agentendpointpb.LookupEffectiveGuestPoliciesResponse_SourcedPackage)
			sp.Package = &v.i
			resp.Packages = append(resp.Packages, sp)
		}
	}
	for _, v := range local.PackageRepositories {
		id := getID(v.i)
		if id != "" {
			if _, ok := repos[id]; ok {
				continue
			}
		}
		sr := new(agentendpointpb.LookupEffectiveGuestPoliciesResponse_SourcedPackageRepository)
		sr.PackageRepository = &v.i
		resp.PackageRepositories = append(resp.PackageRepositories, sr)

	}
	for _, v := range local.SoftwareRecipes {
		if _, ok := recipes[v.r.Name]; !ok {
			sp := new(agentendpointpb.LookupEffectiveGuestPoliciesResponse_SourcedSoftwareRecipe)
			sp.SoftwareRecipe = proto.Clone(&v.r).(*agentendpointpb.SoftwareRecipe)
			resp.SoftwareRecipes = append(resp.SoftwareRecipes, sp)
		}

	}
	return resp
}
