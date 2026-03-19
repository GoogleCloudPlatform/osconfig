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
)

func TestExcludeConversion(t *testing.T) {
	regex, _ := regexp.Compile("PackageName")
	emptyRegex, _ := regexp.Compile("")

	tests := []struct {
		name    string
		input   []string
		want    []*ospatch.Exclude
		wantErr bool
	}{
		{name: "StrictStringConversion", input: []string{"PackageName"}, want: CreateStringExcludes("PackageName")},
		{name: "MultipleStringConversion", input: []string{"PackageName1", "PackageName2"}, want: CreateStringExcludes("PackageName1", "PackageName2")},
		{name: "RegexConversion", input: []string{"/PackageName/"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(regex)}},
		{name: "CornerCaseRegex", input: []string{"//"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(emptyRegex)}},
		{name: "CornerCaseStrictString", input: []string{"/"}, want: CreateStringExcludes("/")},
		{name: "CornerCaseEmptyString", input: []string{""}, want: CreateStringExcludes("")},
		{name: "ErrorInvalidRegex", input: []string{"/[a-z/"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludes, err := convertInputToExcludes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertInputToExcludes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(excludes, tt.want) {
				t.Errorf("convertInputToExcludes() = %s, want = %s", toString(excludes), toString(tt.want))
			}
		})
	}
}

func TestRunUpdates(t *testing.T) {
	stubGlobalState(t)
	retryPeriod = 1 * time.Millisecond

	mockAptSuccess := func(ctx context.Context, opts ...ospatch.AptGetUpgradeOption) error { return nil }
	mockYumSuccess := func(ctx context.Context, opts ...ospatch.YumUpdateOption) error { return nil }
	mockZypperSuccess := func(ctx context.Context, opts ...ospatch.ZypperPatchOption) error { return nil }
	mockAptErr := func(ctx context.Context, opts ...ospatch.AptGetUpgradeOption) error { return errors.New("apt err") }
	mockYumErr := func(ctx context.Context, opts ...ospatch.YumUpdateOption) error { return errors.New("yum err") }
	mockZypperErr := func(ctx context.Context, opts ...ospatch.ZypperPatchOption) error { return errors.New("zypper err") }

	tests := []struct {
		name           string
		setupMocks     func()
		taskConfig     *agentendpointpb.PatchConfig
		wantErrContain string
	}{
		{
			name: "No package managers found",
			setupMocks: func() {
				packages.AptExists = false
				packages.YumExists = false
				packages.ZypperExists = false
			},
			taskConfig:     &agentendpointpb.PatchConfig{},
			wantErrContain: "",
		},
		{
			name: "Apt upgrade success with DIST type",
			setupMocks: func() {
				packages.AptExists = true
				packages.DpkgQueryExists = true
				packages.YumExists = false
				packages.ZypperExists = false
				runAptGetUpgrade = mockAptSuccess
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Apt: &agentendpointpb.AptSettings{Type: agentendpointpb.AptSettings_DIST},
			},
			wantErrContain: "",
		},
		{
			name: "Apt conversion error bubbles up",
			setupMocks: func() {
				packages.AptExists = true
				packages.DpkgQueryExists = true
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Apt: &agentendpointpb.AptSettings{Excludes: []string{"/[a-z/"}}, // Invalid regex
			},
			wantErrContain: "error parsing regexp",
		},
		{
			name: "Yum update success",
			setupMocks: func() {
				packages.AptExists = false
				packages.YumExists = true
				packages.RPMQueryExists = true
				packages.ZypperExists = false
				runYumUpdate = mockYumSuccess
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Yum: &agentendpointpb.YumSettings{
					Security: true,
					Minimal:  true,
				},
			},
			wantErrContain: "",
		},
		{
			name: "Zypper patch success",
			setupMocks: func() {
				packages.AptExists = false
				packages.YumExists = false
				packages.ZypperExists = true
				packages.RPMQueryExists = true
				runZypperPatch = mockZypperSuccess
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Zypper: &agentendpointpb.ZypperSettings{
					WithUpdate:   true,
					WithOptional: true,
				},
			},
			wantErrContain: "",
		},
		{
			name: "Yum conversion error bubbles up",
			setupMocks: func() {
				packages.AptExists = false
				packages.YumExists = true
				packages.RPMQueryExists = true
				packages.ZypperExists = false
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Yum: &agentendpointpb.YumSettings{Excludes: []string{"/[a-z/"}}, // Invalid regex
			},
			wantErrContain: "error parsing regexp",
		},
		{
			name: "Zypper conversion error bubbles up",
			setupMocks: func() {
				packages.AptExists = false
				packages.YumExists = false
				packages.ZypperExists = true
				packages.RPMQueryExists = true
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Zypper: &agentendpointpb.ZypperSettings{Excludes: []string{"/[a-z/"}}, // Invalid regex
			},
			wantErrContain: "error parsing regexp",
		},
		{
			name: "Yum update fails",
			setupMocks: func() {
				packages.AptExists = false
				packages.YumExists = true
				packages.RPMQueryExists = true
				packages.ZypperExists = false
				runYumUpdate = mockYumErr
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Yum: &agentendpointpb.YumSettings{},
			},
			wantErrContain: "yum err",
		},
		{
			name: "Multiple package manager error aggregation",
			setupMocks: func() {
				packages.AptExists = true
				packages.DpkgQueryExists = true
				packages.YumExists = true
				packages.RPMQueryExists = true
				packages.ZypperExists = true // RPMQuery is already true

				runAptGetUpgrade = mockAptErr
				runYumUpdate = mockYumErr
				runZypperPatch = mockZypperErr
			},
			taskConfig: &agentendpointpb.PatchConfig{
				Apt:    &agentendpointpb.AptSettings{},
				Yum:    &agentendpointpb.YumSettings{},
				Zypper: &agentendpointpb.ZypperSettings{},
			},
			wantErrContain: "apt err,\nyum err,\nzypper err",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			task := &patchTask{
				Task: &applyPatchesTask{
					ApplyPatchesTask: &agentendpointpb.ApplyPatchesTask{
						PatchConfig: tt.taskConfig,
					},
				},
			}

			err := task.runUpdates(context.Background())

			if tt.wantErrContain == "" {
				if err != nil {
					t.Errorf("runUpdates() expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("runUpdates() expected error containing %q, got nil", tt.wantErrContain)
				}
				if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("runUpdates() error = %v, want error containing %q", err, tt.wantErrContain)
				}
			}
		})
	}
}

func stubGlobalState(t *testing.T) {
	t.Helper()
	origAptExists := packages.AptExists
	origDpkgExists := packages.DpkgQueryExists
	origYumExists := packages.YumExists
	origRPMExists := packages.RPMQueryExists
	origZypperExists := packages.ZypperExists

	origRunApt := runAptGetUpgrade
	origRunYum := runYumUpdate
	origRunZypper := runZypperPatch
	origRetryPeriod := retryPeriod

	t.Cleanup(func() {
		packages.AptExists = origAptExists
		packages.DpkgQueryExists = origDpkgExists
		packages.YumExists = origYumExists
		packages.RPMQueryExists = origRPMExists
		packages.ZypperExists = origZypperExists

		runAptGetUpgrade = origRunApt
		runYumUpdate = origRunYum
		runZypperPatch = origRunZypper
		retryPeriod = origRetryPeriod
	})
}

func toString(excludes []*ospatch.Exclude) string {
	results := make([]string, len(excludes))
	for i, exc := range excludes {
		results[i] = exc.String()
	}

	return strings.Join(results, ",")
}

func CreateStringExcludes(pkgs ...string) []*ospatch.Exclude {
	excludes := make([]*ospatch.Exclude, len(pkgs))
	for i := 0; i < len(pkgs); i++ {
		pkg := pkgs[i]
		excludes[i] = ospatch.CreateStringExclude(&pkg)
	}

	return excludes
}
