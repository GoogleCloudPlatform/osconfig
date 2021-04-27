//  Copyright 2020 Google Inc. All Rights Reserved.
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

package config

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

var (
	aptRepositoryResource = &agentendpointpb.OSPolicy_Resource_RepositoryResource_AptRepository{
		ArchiveType:  agentendpointpb.OSPolicy_Resource_RepositoryResource_AptRepository_DEB,
		Uri:          "uri",
		Distribution: "distribution",
		Components:   []string{"c1", "c2"},
	}
	gooRepositoryResource = &agentendpointpb.OSPolicy_Resource_RepositoryResource_GooRepository{
		Name: "name",
		Url:  "url",
	}
	yumRepositoryResource = &agentendpointpb.OSPolicy_Resource_RepositoryResource_YumRepository{
		Id:          "id",
		DisplayName: "displayname",
		BaseUrl:     "baseurl",
		GpgKeys:     []string{"key1", "key2"},
	}
	zypperRepositoryResource = &agentendpointpb.OSPolicy_Resource_RepositoryResource_ZypperRepository{
		Id:          "id",
		DisplayName: "displayname",
		BaseUrl:     "baseurl",
		GpgKeys:     []string{"key1", "key2"},
	}
)

func TestRepositoryResourceValidate(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name   string
		rrpb   *agentendpointpb.OSPolicy_Resource_RepositoryResource
		wantMR ManagedRepository
	}{
		{
			"Apt",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Apt{
					Apt: aptRepositoryResource,
				},
			},
			ManagedRepository{
				Apt: &AptRepository{
					RepositoryResource: aptRepositoryResource,
				},
				RepoChecksum:     "8faacd43b230b08e7a1da7b670bf6f90fcc59ade1a5e7179a0ccffc9aa3d7cdf",
				RepoFileContents: []byte("# Repo file managed by Google OSConfig agent\ndeb uri distribution c1 c2\n"),
				RepoFilePath:     "/etc/apt/sources.list.d/osconfig_managed_8faacd43b2.list",
			},
		},
		{
			"GooGet",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Goo{
					Goo: gooRepositoryResource,
				},
			},
			ManagedRepository{
				GooGet: &GooGetRepository{
					RepositoryResource: gooRepositoryResource,
				},
				RepoChecksum:     "76ae1ea015cd184a18434ae45e233c22f36e35faa80f495cf21f95495af3b599",
				RepoFileContents: []byte("# Repo file managed by Google OSConfig agent\n- name: name\n  url: url\n"), RepoFilePath: "C:/ProgramData/GooGet/repos/osconfig_managed_76ae1ea015.repo",
			},
		},
		{
			"Yum",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Yum{
					Yum: yumRepositoryResource,
				},
			},
			ManagedRepository{
				Yum: &YumRepository{
					RepositoryResource: yumRepositoryResource,
				},
				RepoChecksum:     "c588551e69834e5f2f4d825324b2add5df3064af7d5d68021e83a308c6f62048",
				RepoFileContents: []byte("# Repo file managed by Google OSConfig agent\n[id]\nname=displayname\nbaseurl=baseurl\nenabled=1\ngpgcheck=1\ngpgkey=key1\n       key2\n"),
				RepoFilePath:     "/etc/yum.repos.d/osconfig_managed_c588551e69.repo",
			},
		},
		{
			"Zypper",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Zypper{
					Zypper: zypperRepositoryResource,
				},
			},
			ManagedRepository{
				Zypper: &ZypperRepository{
					RepositoryResource: zypperRepositoryResource,
				},
				RepoChecksum:     "415a52ad70e5118cd797882d5f421b0a8d84bfe1e35cd53e8dcac486bc93186d",
				RepoFileContents: []byte("# Repo file managed by Google OSConfig agent\n[id]\nname=displayname\nbaseurl=baseurl\nenabled=1\ngpgkey=key1\n       key2\n"),
				RepoFilePath:     "/etc/zypp/repos.d/osconfig_managed_415a52ad70.repo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Repository{Repository: tt.rrpb},
				},
			}
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := cmp.Diff(pr.ManagedResources(), &ManagedResources{Repositories: []ManagedRepository{tt.wantMR}}, protocmp.Transform()); diff != "" {
				t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
			}
			if diff := cmp.Diff(pr.resource.(*repositoryResource).managedRepository, tt.wantMR, protocmp.Transform()); diff != "" {
				t.Errorf("packageResouce does not match expectation: (-got +want)\n%s", diff)
			}
		})
	}
}

func TestRepositoryResourceCheckState(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name               string
		rrpb               *agentendpointpb.OSPolicy_Resource_RepositoryResource
		contents           []byte
		wantInDesiredState bool
	}{
		{
			"Matches",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Apt{
					Apt: aptRepositoryResource,
				},
			},
			[]byte("# Repo file managed by Google OSConfig agent\ndeb uri distribution c1 c2\n"),
			true,
		},
		{
			"DoesNotMatch",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Apt{
					Apt: aptRepositoryResource,
				},
			},
			[]byte("# Repo file managed by Google OSConfig agent\nsome other repo\n"),
			false,
		},
		{
			"NoRepoFile",
			&agentendpointpb.OSPolicy_Resource_RepositoryResource{
				Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Apt{
					Apt: aptRepositoryResource,
				},
			},
			nil,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Repository{Repository: tt.rrpb},
				},
			}
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(dir)

			path := filepath.Join(dir, "repo")
			if tt.contents != nil {
				if err := ioutil.WriteFile(path, tt.contents, 0755); err != nil {
					t.Fatal(err)
				}
			}
			pr.resource.(*repositoryResource).managedRepository.RepoFilePath = path

			if err := pr.CheckState(ctx); err != nil {
				t.Fatalf("Unexpected CheckState error: %v", err)
			}

			if tt.wantInDesiredState != pr.InDesiredState() {
				t.Fatalf("Unexpected InDesiredState, want: %t, got: %t", tt.wantInDesiredState, pr.InDesiredState())
			}
		})
	}
}

func TestRepositoryResourceEnforceState(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)

	path := filepath.Join(dir, "repo")
	if err := ioutil.WriteFile(path, []byte("foo"), 0755); err != nil {
		t.Fatal(err)
	}

	rrpb := &agentendpointpb.OSPolicy_Resource_RepositoryResource{
		Repository: &agentendpointpb.OSPolicy_Resource_RepositoryResource_Apt{
			Apt: aptRepositoryResource,
		},
	}

	for _, tt := range []struct {
		name string
		path string
	}{
		{
			"FileExists",
			path,
		},
		{
			"FileDNE",
			filepath.Join(dir, "some/other/dir/repo"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Repository{Repository: rrpb},
				},
			}
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			pr.resource.(*repositoryResource).managedRepository.RepoFilePath = tt.path

			if err := pr.EnforceState(ctx); err != nil {
				t.Fatalf("Unexpected EnforceState error: %v", err)
			}

			match, err := contentsMatch(pr.resource.(*repositoryResource).managedRepository.RepoFilePath, pr.resource.(*repositoryResource).managedRepository.RepoChecksum)
			if err != nil {
				t.Fatal(err)
			}
			if !match {
				t.Fatal("Repo file contents do not match after enforcement")
			}
		})
	}
}
