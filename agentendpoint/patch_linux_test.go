//  Copyright 2022 Google Inc. All Rights Reserved.
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

package agentendpoint

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func TestExcludeConversion(t *testing.T) {
	regex, _ := regexp.Compile("PackageName")
	emptyRegex, _ := regexp.Compile("")
	_, regexpErr := regexp.Compile("[a-z")

	tests := []struct {
		name    string
		input   []string
		want    []*ospatch.Exclude
		wantErr error
	}{
		{name: "Single package name, want one string exclude", input: []string{"PackageName"}, want: createStringExcludes("PackageName")},
		{name: "Multiple package names, want multiple string excludes", input: []string{"PackageName1", "PackageName2"}, want: createStringExcludes("PackageName1", "PackageName2")},
		{name: "Slash-wrapped value, want regex exclude", input: []string{"/PackageName/"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(regex)}},
		{name: "Empty regex //, want empty regex exclude", input: []string{"//"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(emptyRegex)}},
		{name: "Single slash, want string exclude", input: []string{"/"}, want: createStringExcludes("/")},
		{name: "Empty string, want string exclude", input: []string{""}, want: createStringExcludes("")},
		{name: "Invalid regex, want regex compile error", input: []string{"/[a-z/"}, wantErr: regexpErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludes, err := convertInputToExcludes(tt.input)

			if !reflect.DeepEqual(excludes, tt.want) {
				t.Errorf("convertInputToExcludes() = %s, want = %s", toString(excludes), toString(tt.want))
			}
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestRunUpdates(t *testing.T) {
	utiltest.OverrideVariable(t, &retryPeriod, 1*time.Millisecond)

	mockAptSuccess := func(ctx context.Context, opts ...ospatch.AptGetUpgradeOption) error { return nil }
	mockYumSuccess := func(ctx context.Context, opts ...ospatch.YumUpdateOption) error { return nil }
	mockZypperSuccess := func(ctx context.Context, opts ...ospatch.ZypperPatchOption) error { return nil }
	mockAptErr := func(ctx context.Context, opts ...ospatch.AptGetUpgradeOption) error { return errors.New("apt err") }
	mockYumErr := func(ctx context.Context, opts ...ospatch.YumUpdateOption) error { return errors.New("yum err") }
	mockZypperErr := func(ctx context.Context, opts ...ospatch.ZypperPatchOption) error { return errors.New("zypper err") }
	_, regexpErr := regexp.Compile("[a-z")

	tests := []struct {
		name       string
		setup      func(t *testing.T)
		taskConfig *agentendpointpb.PatchConfig
		wantErr    error
	}{
		{
			name: "No package managers detected, want no patch operations attempted and no error",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
			},
			taskConfig: &agentendpointpb.PatchConfig{},
			wantErr:    nil,
		},
		{
			name: "Apt available, want apt-get dist-upgrade attempted and no error",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableApt(t, mockAptSuccess)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Apt: &agentendpointpb.AptSettings{Type: agentendpointpb.AptSettings_DIST},
			},
			wantErr: nil,
		},
		{
			name: "Apt available with invalid regex in excludes, want regex compile error and no upgrade attempted",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableApt(t, nil)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Apt: &agentendpointpb.AptSettings{Excludes: []string{"/[a-z/"}},
			},
			wantErr: regexpErr,
		},
		{
			name: "Yum available, want yum update attempted and no error",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableYum(t, mockYumSuccess)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Yum: &agentendpointpb.YumSettings{
					Security: true,
					Minimal:  true,
				},
			},
			wantErr: nil,
		},
		{
			name: "Zypper available, want zypper patch attempted and no error",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableZypper(t, mockZypperSuccess)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Zypper: &agentendpointpb.ZypperSettings{
					WithUpdate:   true,
					WithOptional: true,
				},
			},
			wantErr: nil,
		},
		{
			name: "Yum available with invalid regex in excludes, want regex compile error and no update attempted",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableYum(t, nil)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Yum: &agentendpointpb.YumSettings{Excludes: []string{"/[a-z/"}},
			},
			wantErr: regexpErr,
		},
		{
			name: "Zypper available with invalid regex in excludes, want regex compile error and no patch attempted",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableZypper(t, nil)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Zypper: &agentendpointpb.ZypperSettings{Excludes: []string{"/[a-z/"}},
			},
			wantErr: regexpErr,
		},
		{
			name: "Yum available and yum update returns error, want that error returned",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableYum(t, mockYumErr)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Yum: &agentendpointpb.YumSettings{},
			},
			wantErr: errors.New("yum err"),
		},
		{
			name: "All package managers available and each fails, want all three errors aggregated",
			setup: func(t *testing.T) {
				disableAllPackageManagers(t)
				enableApt(t, mockAptErr)
				enableYum(t, mockYumErr)
				enableZypper(t, mockZypperErr)
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Apt:    &agentendpointpb.AptSettings{},
				Yum:    &agentendpointpb.YumSettings{},
				Zypper: &agentendpointpb.ZypperSettings{},
			},
			wantErr: errors.New("apt err,\nyum err,\nzypper err"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			task := initializePatchTask(tt.taskConfig)

			err := task.runUpdates(context.Background())
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func initializePatchTask(config *agentendpointpb.PatchConfig) *patchTask {
	return &patchTask{
		Task: &applyPatchesTask{
			ApplyPatchesTask: &agentendpointpb.ApplyPatchesTask{
				PatchConfig: config,
			},
		},
	}
}

func disableAllPackageManagers(t *testing.T) {
	t.Helper()
	utiltest.OverrideVariable(t, &packages.DpkgQueryExists, false)
	utiltest.OverrideVariable(t, &packages.YumExists, false)
	utiltest.OverrideVariable(t, &packages.RPMQueryExists, false)
	utiltest.OverrideVariable(t, &packages.ZypperExists, false)
}

func enableApt(t *testing.T, run func(ctx context.Context, opts ...ospatch.AptGetUpgradeOption) error) {
	t.Helper()
	utiltest.OverrideVariable(t, &packages.AptExists, true)
	utiltest.OverrideVariable(t, &packages.DpkgQueryExists, true)
	if run != nil {
		utiltest.OverrideVariable(t, &runAptGetUpgrade, run)
	}
}

func enableYum(t *testing.T, run func(ctx context.Context, opts ...ospatch.YumUpdateOption) error) {
	t.Helper()
	utiltest.OverrideVariable(t, &packages.YumExists, true)
	utiltest.OverrideVariable(t, &packages.RPMQueryExists, true)
	if run != nil {
		utiltest.OverrideVariable(t, &runYumUpdate, run)
	}
}

func enableZypper(t *testing.T, run func(ctx context.Context, opts ...ospatch.ZypperPatchOption) error) {
	t.Helper()
	utiltest.OverrideVariable(t, &packages.ZypperExists, true)
	utiltest.OverrideVariable(t, &packages.RPMQueryExists, true)
	if run != nil {
		utiltest.OverrideVariable(t, &runZypperPatch, run)
	}
}

func toString(excludes []*ospatch.Exclude) string {
	results := make([]string, len(excludes))
	for i, exc := range excludes {
		results[i] = exc.String()
	}

	return strings.Join(results, ",")
}

func createStringExcludes(pkgs ...string) []*ospatch.Exclude {
	excludes := make([]*ospatch.Exclude, len(pkgs))
	for i := 0; i < len(pkgs); i++ {
		pkg := pkgs[i]
		excludes[i] = ospatch.CreateStringExclude(&pkg)
	}

	return excludes
}
