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
	"context"
	"encoding/json"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

// localConfig represents the structure of the config to the JSON parser.
//
// The types of members of the struct are wrappers for protobufs and delegate
// the parsing to protojson lib via their UnmarshalJSON implementations.
type localConfig struct {
	Packages            []*pkg
	PackageRepositories []*packageRepository
	SoftwareRecipes     []*softwareRecipe
}

type pkg struct {
	agentendpointpb.Package
}

func (r *pkg) UnmarshalJSON(b []byte) error {
	un := &protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}
	return un.Unmarshal(b, &r.Package)
}

type packageRepository struct {
	agentendpointpb.PackageRepository
}

func (r *packageRepository) UnmarshalJSON(b []byte) error {
	un := &protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}
	return un.Unmarshal(b, &r.PackageRepository)
}

type softwareRecipe struct {
	agentendpointpb.SoftwareRecipe
}

func (r *softwareRecipe) UnmarshalJSON(b []byte) error {
	un := &protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}
	return un.Unmarshal(b, &r.SoftwareRecipe)
}

func readLocalConfig(ctx context.Context) (*localConfig, error) {
	s, err := metadata.Get("/instance/attributes/gce-software-declaration")
	if err != nil {
		clog.Debugf(ctx, "No local config: %v", err)
		return nil, nil
	}

	var lc localConfig
	return &lc, json.Unmarshal([]byte(s), &lc)
}

// GetId returns a repository Id that is used to group repositories for
// override by higher priotiry policy(-ies).
// For repositories that have no such Id, GetId returns "", in which
// case the repository is never overridden.
func getID(repo *agentendpointpb.PackageRepository) string {
	switch repo.Repository.(type) {
	case *agentendpointpb.PackageRepository_Yum:
		return "yum-" + repo.GetYum().GetId()
	case *agentendpointpb.PackageRepository_Zypper:
		return "zypper-" + repo.GetZypper().GetId()
	default:
		return ""
	}
}

// mergeConfigs merges the local config with the lookup response, giving priority to the lookup
// result. If both arguments are nil, returns an empty policy.
func mergeConfigs(local *localConfig, egp *agentendpointpb.EffectiveGuestPolicy) *agentendpointpb.EffectiveGuestPolicy {
	if egp == nil {
		egp = &agentendpointpb.EffectiveGuestPolicy{}
	}
	if local == nil {
		return egp
	}

	// Ids that are in the maps below
	repos := make(map[string]bool)
	pkgs := make(map[string]bool)
	recipes := make(map[string]bool)

	for _, v := range egp.GetPackages() {
		pkgs[v.Package.Name] = true
	}
	for _, v := range egp.GetPackageRepositories() {
		if id := getID(v.GetPackageRepository()); id != "" {
			repos[id] = true
		}
	}
	for _, v := range egp.GetSoftwareRecipes() {
		recipes[v.SoftwareRecipe.Name] = true
	}
	for _, v := range local.Packages {
		if _, ok := pkgs[v.Name]; !ok {
			sp := new(agentendpointpb.EffectiveGuestPolicy_SourcedPackage)
			sp.Package = &v.Package
			egp.Packages = append(egp.Packages, sp)
		}
	}
	for _, v := range local.PackageRepositories {
		id := getID(&v.PackageRepository)
		if id != "" {
			if _, ok := repos[id]; ok {
				continue
			}
		}
		sr := new(agentendpointpb.EffectiveGuestPolicy_SourcedPackageRepository)
		sr.PackageRepository = &v.PackageRepository
		egp.PackageRepositories = append(egp.PackageRepositories, sr)

	}
	for _, v := range local.SoftwareRecipes {
		if _, ok := recipes[v.Name]; !ok {
			sp := new(agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe)
			sp.SoftwareRecipe = proto.Clone(&v.SoftwareRecipe).(*agentendpointpb.SoftwareRecipe)
			egp.SoftwareRecipes = append(egp.SoftwareRecipes, sp)
		}

	}
	return egp
}
