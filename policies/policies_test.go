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
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

type managerCalls struct {
	install []string
	remove  []string
	update  []string
	repos   int
}

func setupMocks(googetExists, aptExists, yumExists, zypperExists bool, gotCalls map[string]*managerCalls) func() {
	// preserve original functions
	oldGooE, oldAptE, oldYumE, oldZypE := packages.GooGetExists, packages.AptExists, packages.YumExists, packages.ZypperExists
	oldGooR, oldGooC := googetRepositoriesFunc, googetChangesFunc
	oldAptR, oldAptC := aptRepositoriesFunc, aptChangesFunc
	oldYumR, oldYumC := yumRepositoriesFunc, yumChangesFunc
	oldZypR, oldZypC := zypperRepositoriesFunc, zypperChangesFunc
	oldRetry := retryRPC

	// mock Exists functions
	packages.GooGetExists, packages.AptExists = googetExists, aptExists
	packages.YumExists, packages.ZypperExists = yumExists, zypperExists

	// replace retryRPC function to avoid long awaits
	retryRPC = func(ctx context.Context, timeout time.Duration, desc string, f func() error) error { return f() }

	getCalls := func(m string) *managerCalls {
		if _, ok := gotCalls[m]; !ok {
			gotCalls[m] = &managerCalls{}
		}
		return gotCalls[m]
	}

	capture := func(m string) func(ctx context.Context, inst, rem, upd []*agentendpointpb.Package) error {
		return func(ctx context.Context, inst, rem, upd []*agentendpointpb.Package) error {
			c := getCalls(m)
			for _, p := range inst {
				c.install = append(c.install, p.Name)
			}
			for _, p := range rem {
				c.remove = append(c.remove, p.Name)
			}
			for _, p := range upd {
				c.update = append(c.update, p.Name)
			}
			return nil
		}
	}

	googetRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.GooRepository, path string) error {
		getCalls("googet").repos += len(repos)
		return nil
	}
	googetChangesFunc = capture("googet")

	aptRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.AptRepository, path string) error {
		getCalls("apt").repos += len(repos)
		return nil
	}
	aptChangesFunc = capture("apt")

	yumRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.YumRepository, path string) error {
		getCalls("yum").repos += len(repos)
		return nil
	}
	yumChangesFunc = capture("yum")

	zypperRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.ZypperRepository, path string) error {
		getCalls("zypper").repos += len(repos)
		return nil
	}
	zypperChangesFunc = capture("zypper")

	return func() {
		packages.GooGetExists, packages.AptExists = oldGooE, oldAptE
		packages.YumExists, packages.ZypperExists = oldYumE, oldZypE
		googetRepositoriesFunc, googetChangesFunc = oldGooR, oldGooC
		aptRepositoriesFunc, aptChangesFunc = oldAptR, oldAptC
		yumRepositoriesFunc, yumChangesFunc = oldYumR, oldYumC
		zypperRepositoriesFunc, zypperChangesFunc = oldZypR, oldZypC
		retryRPC = oldRetry
	}
}

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
			t.Parallel()
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
			t.Parallel()
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
		name       string
		recipes    []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe
		installErr error
		wantInst   []string
	}{
		{
			name: "multiple recipes",
			recipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
				{SoftwareRecipe: &agentendpointpb.SoftwareRecipe{Name: "recipe1"}},
				{SoftwareRecipe: &agentendpointpb.SoftwareRecipe{Name: "recipe2"}},
			},
			wantInst: []string{"recipe1", "recipe2"},
		},
		{
			name: "nil SoftwareRecipe",
			recipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
				{SoftwareRecipe: &agentendpointpb.SoftwareRecipe{Name: "recipe1"}},
				{SoftwareRecipe: nil},
			},
			wantInst: []string{"recipe1"},
		},
		{
			name: "installation error",
			recipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
				{SoftwareRecipe: &agentendpointpb.SoftwareRecipe{Name: "recipe1"}},
			},
			installErr: errors.New("install error"),
			wantInst:   []string{"recipe1"},
		},
		{
			name:     "no recipes",
			recipes:  nil,
			wantInst: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var installed []string
			oldInstallRecipe := installRecipe
			installRecipe = func(ctx context.Context, recipe *agentendpointpb.SoftwareRecipe) error {
				installed = append(installed, recipe.Name)
				return tt.installErr
			}
			defer func() { installRecipe = oldInstallRecipe }()

			egp := &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: tt.recipes,
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

// TestSetConfig verifies that setConfig correctly distributes packages and repositories to different managers.
func TestSetConfig(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		googetExists bool
		aptExists    bool
		yumExists    bool
		zypperExists bool
		packages     []*agentendpointpb.Package
		repos        []*agentendpointpb.PackageRepository
		wantCalls    map[string]managerCalls
	}{
		{
			name:         "all managers and all states",
			googetExists: true, aptExists: true, yumExists: true, zypperExists: true,
			packages: []*agentendpointpb.Package{
				{Name: "p-any-rem", Manager: agentendpointpb.Package_ANY, DesiredState: agentendpointpb.DesiredState_REMOVED},
				{Name: "p-apt-upd", Manager: agentendpointpb.Package_APT, DesiredState: agentendpointpb.DesiredState_UPDATED},
				{Name: "p-goo-inst", Manager: agentendpointpb.Package_GOO, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "p-yum-inst", Manager: agentendpointpb.Package_YUM, DesiredState: agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED},
				{Name: "p-zyp-upd", Manager: agentendpointpb.Package_ZYPPER, DesiredState: agentendpointpb.DesiredState_UPDATED},
			},
			repos: []*agentendpointpb.PackageRepository{
				{Repository: &agentendpointpb.PackageRepository_Apt{Apt: &agentendpointpb.AptRepository{}}},
				{Repository: &agentendpointpb.PackageRepository_Yum{Yum: &agentendpointpb.YumRepository{}}},
				{Repository: &agentendpointpb.PackageRepository_Zypper{Zypper: &agentendpointpb.ZypperRepository{}}},
				{Repository: &agentendpointpb.PackageRepository_Goo{Goo: &agentendpointpb.GooRepository{}}},
			},
			wantCalls: map[string]managerCalls{
				"googet": {install: []string{"p-goo-inst"}, remove: []string{"p-any-rem"}, repos: 1},
				"apt":    {remove: []string{"p-any-rem"}, update: []string{"p-apt-upd"}, repos: 1},
				"yum":    {install: []string{"p-yum-inst"}, remove: []string{"p-any-rem"}, repos: 1},
				"zypper": {remove: []string{"p-any-rem"}, update: []string{"p-zyp-upd"}, repos: 1},
			},
		},
		{
			name:      "manager doesnt exist",
			aptExists: true,
			packages: []*agentendpointpb.Package{
				{Name: "apt-pkg", Manager: agentendpointpb.Package_APT, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "yum-pkg", Manager: agentendpointpb.Package_YUM, DesiredState: agentendpointpb.DesiredState_INSTALLED},
			},
			wantCalls: map[string]managerCalls{
				"apt": {install: []string{"apt-pkg"}},
			},
		},
		{
			name:         "exhaustive combinations",
			googetExists: true, aptExists: true, yumExists: true, zypperExists: true,
			packages: []*agentendpointpb.Package{
				{Name: "any-inst", Manager: agentendpointpb.Package_ANY, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "any-upd", Manager: agentendpointpb.Package_ANY, DesiredState: agentendpointpb.DesiredState_UPDATED},
				{Name: "unspec-inst", Manager: agentendpointpb.Package_MANAGER_UNSPECIFIED, DesiredState: agentendpointpb.DesiredState_INSTALLED},
				{Name: "goo-rem", Manager: agentendpointpb.Package_GOO, DesiredState: agentendpointpb.DesiredState_REMOVED},
				{Name: "goo-upd", Manager: agentendpointpb.Package_GOO, DesiredState: agentendpointpb.DesiredState_UPDATED},
				{Name: "apt-rem", Manager: agentendpointpb.Package_APT, DesiredState: agentendpointpb.DesiredState_REMOVED},
				{Name: "yum-rem", Manager: agentendpointpb.Package_YUM, DesiredState: agentendpointpb.DesiredState_REMOVED},
				{Name: "yum-upd", Manager: agentendpointpb.Package_YUM, DesiredState: agentendpointpb.DesiredState_UPDATED},
				{Name: "zyp-rem", Manager: agentendpointpb.Package_ZYPPER, DesiredState: agentendpointpb.DesiredState_REMOVED},
			},
			wantCalls: map[string]managerCalls{
				"googet": {install: []string{"any-inst", "unspec-inst"}, remove: []string{"goo-rem"}, update: []string{"any-upd", "goo-upd"}},
				"apt":    {install: []string{"any-inst", "unspec-inst"}, remove: []string{"apt-rem"}, update: []string{"any-upd"}},
				"yum":    {install: []string{"any-inst", "unspec-inst"}, remove: []string{"yum-rem"}, update: []string{"any-upd", "yum-upd"}},
				"zypper": {install: []string{"any-inst", "unspec-inst"}, remove: []string{"zyp-rem"}, update: []string{"any-upd"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotCalls := make(map[string]*managerCalls)
			defer setupMocks(tt.googetExists, tt.aptExists, tt.yumExists, tt.zypperExists, gotCalls)()

			var sourcedPackages []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage
			for _, p := range tt.packages {
				sourcedPackages = append(sourcedPackages, &agentendpointpb.EffectiveGuestPolicy_SourcedPackage{Package: p})
			}
			var sourcedRepos []*agentendpointpb.EffectiveGuestPolicy_SourcedPackageRepository
			for _, r := range tt.repos {
				sourcedRepos = append(sourcedRepos, &agentendpointpb.EffectiveGuestPolicy_SourcedPackageRepository{PackageRepository: r})
			}

			egp := &agentendpointpb.EffectiveGuestPolicy{Packages: sourcedPackages, PackageRepositories: sourcedRepos}

			setConfig(ctx, egp)

			for m, want := range tt.wantCalls {
				got := gotCalls[m]
				if got == nil {
					got = &managerCalls{}
				}
				if !reflect.DeepEqual(got.install, want.install) {
					t.Errorf("%s: install = %v, want %v", m, got.install, want.install)
				}
				if !reflect.DeepEqual(got.remove, want.remove) {
					t.Errorf("%s: remove = %v, want %v", m, got.remove, want.remove)
				}
				if !reflect.DeepEqual(got.update, want.update) {
					t.Errorf("%s: update = %v, want %v", m, got.update, want.update)
				}
				if got.repos != want.repos {
					t.Errorf("%s: repos = %d, want %d", m, got.repos, want.repos)
				}
			}
		})
	}
}

// TestSetConfigErrors verifies that setConfig handles errors from repository and package operations without panicking.
func TestSetConfigErrors(t *testing.T) {
	ctx := context.Background()

	gotCalls := make(map[string]*managerCalls)
	defer setupMocks(false, true, false, false, gotCalls)()

	// Overwrite some mocks to return errors
	oldAptRepos := aptRepositoriesFunc
	oldAptChanges := aptChangesFunc
	aptRepositoriesFunc = func(ctx context.Context, repos []*agentendpointpb.AptRepository, path string) error {
		return errors.New("repo error")
	}
	aptChangesFunc = func(ctx context.Context, install, remove, update []*agentendpointpb.Package) error {
		return errors.New("changes error")
	}
	defer func() {
		aptRepositoriesFunc = oldAptRepos
		aptChangesFunc = oldAptChanges
	}()

	egp := &agentendpointpb.EffectiveGuestPolicy{
		PackageRepositories: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackageRepository{
			{PackageRepository: &agentendpointpb.PackageRepository{Repository: &agentendpointpb.PackageRepository_Apt{Apt: &agentendpointpb.AptRepository{}}}},
		},
		Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
			{Package: &agentendpointpb.Package{Name: "pkg", Manager: agentendpointpb.Package_APT, DesiredState: agentendpointpb.DesiredState_INSTALLED}},
		},
	}

	// This should run without panic even if errors occur (errors are logged)
	setConfig(ctx, egp)
}
