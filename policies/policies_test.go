//  Copyright 2026 Google Inc. All Rights Reserved.
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

package policies

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// TestChecksum verifies that checksum correctly calculates the SHA256 hash of the input reader.
func TestChecksum(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "basic string",
			data: []byte("test data"),
		},
		{
			name: "empty string",
			data: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			h := checksum(r)

			expected := sha256.Sum256(tt.data)
			got := h.Sum(nil)

			if !bytes.Equal(got, expected[:]) {
				t.Errorf("checksum() = %x, want %x", got, expected)
			}
		})
	}
}

// TestWriteIfChanged verifies that writeIfChanged only writes to the file if the content has changed.
func TestWriteIfChanged(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		initialContent []byte
		newContent     []byte
		wantErr        error
	}{
		{
			name:       "new file creation",
			newContent: []byte("content 1"),
			wantErr:    nil,
		},
		{
			name:           "no change",
			initialContent: []byte("content 1"),
			newContent:     []byte("content 1"),
			wantErr:        nil,
		},
		{
			name:           "content update",
			initialContent: []byte("content 1"),
			newContent:     []byte("content 2"),
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := t.TempDir()
			path := filepath.Join(td, "test_file")

			if tt.initialContent != nil {
				if err := os.WriteFile(path, tt.initialContent, 0644); err != nil {
					t.Fatalf("failed to setup initial file: %v", err)
				}
			}

			err := writeIfChanged(ctx, tt.newContent, path)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			if err == nil {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("failed to read file after test: %v", err)
				}
				if !bytes.Equal(got, tt.newContent) {
					t.Errorf("file content = %q, want %q", string(got), string(tt.newContent))
				}
			}
		})
	}
}

// TestInstallRecipes verifies that installRecipes iterates over all recipes and calls the install function.
func TestInstallRecipes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		recipes  []*agentendpointpb.SoftwareRecipe
		wantInst []string
	}{
		{
			name: "multiple recipes",
			recipes: []*agentendpointpb.SoftwareRecipe{
				{Name: "recipe1"},
				{Name: "recipe2"},
			},
			wantInst: []string{"recipe1", "recipe2"},
		},
		{
			name:     "no recipes",
			recipes:  nil,
			wantInst: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var installed []string
			oldInstallRecipe := installRecipe
			installRecipe = func(ctx context.Context, recipe *agentendpointpb.SoftwareRecipe) error {
				installed = append(installed, recipe.Name)
				return nil
			}
			defer func() { installRecipe = oldInstallRecipe }()

			var sourcedRecipes []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe
			for _, r := range tt.recipes {
				sourcedRecipes = append(sourcedRecipes, &agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					SoftwareRecipe: r,
				})
			}

			egp := &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: sourcedRecipes,
			}

			if err := installRecipes(ctx, egp); err != nil {
				t.Fatalf("installRecipes() error = %v", err)
			}

			if !reflect.DeepEqual(installed, tt.wantInst) {
				t.Errorf("installed recipes = %v, want %v", installed, tt.wantInst)
			}
		})
	}
}

// TestSetConfig verifies that setConfig correctly distributes packages to different managers.
func TestSetConfig(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		aptExists        bool
		yumExists        bool
		packages         []*agentendpointpb.Package
		wantAptInstalled []string
		wantYumInstalled []string
	}{
		{
			name:      "mixed managers and packages",
			aptExists: true,
			yumExists: true,
			packages: []*agentendpointpb.Package{
				{Name: "any-pkg", Manager: agentendpointpb.Package_ANY, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "apt-pkg", Manager: agentendpointpb.Package_APT, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "yum-pkg", Manager: agentendpointpb.Package_YUM, DesiredState: agentendpointpb.DesiredState_INSTALLED},
			},
			wantAptInstalled: []string{"any-pkg", "apt-pkg"},
			wantYumInstalled: []string{"any-pkg", "yum-pkg"},
		},
		{
			name:      "only apt manager exists",
			aptExists: true,
			yumExists: false,
			packages: []*agentendpointpb.Package{
				{Name: "any-pkg", Manager: agentendpointpb.Package_ANY, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "yum-pkg", Manager: agentendpointpb.Package_YUM, DesiredState: agentendpointpb.DesiredState_INSTALLED},
			},
			wantAptInstalled: []string{"any-pkg"},
			wantYumInstalled: nil,
		},
		{
			name:      "unspecified manager defaults to all",
			aptExists: true,
			yumExists: true,
			packages: []*agentendpointpb.Package{
				{Name: "default-pkg", Manager: agentendpointpb.Package_MANAGER_UNSPECIFIED, DesiredState: agentendpointpb.DesiredState_INSTALLED},
			},
			wantAptInstalled: []string{"default-pkg"},
			wantYumInstalled: []string{"default-pkg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock managers existence
			oldAptExists := packages.AptExists
			oldYumExists := packages.YumExists
			packages.AptExists = tt.aptExists
			packages.YumExists = tt.yumExists
			defer func() {
				packages.AptExists = oldAptExists
				packages.YumExists = oldYumExists
			}()

			var aptInstalled, yumInstalled []string
			// Mock Apt functions
			oldAptRepos := aptRepositoriesFunc
			oldAptChanges := aptChangesFunc
			aptRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.AptRepository, path string) error { return nil }
			aptChangesFunc = func(ctx context.Context, install, remove, update []*agentendpointpb.Package) error {
				for _, p := range install {
					aptInstalled = append(aptInstalled, p.Name)
				}
				return nil
			}
			defer func() {
				aptRepositoriesFunc = oldAptRepos
				aptChangesFunc = oldAptChanges
			}()

			// Mock Yum functions
			oldYumRepos := yumRepositoriesFunc
			oldYumChanges := yumChangesFunc
			yumRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.YumRepository, path string) error { return nil }
			yumChangesFunc = func(ctx context.Context, install, remove, update []*agentendpointpb.Package) error {
				for _, p := range install {
					yumInstalled = append(yumInstalled, p.Name)
				}
				return nil
			}
			defer func() {
				yumRepositoriesFunc = oldYumRepos
				yumChangesFunc = oldYumChanges
			}()

			var sourcedPackages []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage
			for _, p := range tt.packages {
				sourcedPackages = append(sourcedPackages, &agentendpointpb.EffectiveGuestPolicy_SourcedPackage{Package: p})
			}

			egp := &agentendpointpb.EffectiveGuestPolicy{Packages: sourcedPackages}

			setConfig(ctx, egp)

			if !reflect.DeepEqual(aptInstalled, tt.wantAptInstalled) {
				t.Errorf("aptInstalled = %v, want %v", aptInstalled, tt.wantAptInstalled)
			}
			if !reflect.DeepEqual(yumInstalled, tt.wantYumInstalled) {
				t.Errorf("yumInstalled = %v, want %v", yumInstalled, tt.wantYumInstalled)
			}
		})
	}
}
