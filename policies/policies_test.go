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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

// TestChecksum verifies that checksum correctly calculates the SHA256 hash of the input reader.
func TestChecksum(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "basic string data, want correct sha256 hash",
			data: []byte("test data"),
		},
		{
			name: "empty data, want hash of empty data",
			data: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.data)
			hasher := checksum(reader)

			want := sha256.Sum256(tt.data)
			got := hasher.Sum(nil)

			utiltest.AssertEquals(t, got, want[:])
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
		pathFunc       func(t *testing.T, initialContent []byte) string
		wantErr        error
	}{
		{
			name:       "new content for non-existent file, want nil error",
			newContent: []byte("content 1"),
			pathFunc: func(t *testing.T, initialContent []byte) string {
				return utiltest.WriteToTempFileMust(t, "test_file", initialContent)
			},
			wantErr: nil,
		},
		{
			name:           "same content as existing file, want nil error",
			initialContent: []byte("content 1"),
			newContent:     []byte("content 1"),
			pathFunc: func(t *testing.T, initialContent []byte) string {
				return utiltest.WriteToTempFileMust(t, "test_file", initialContent)
			},
			wantErr: nil,
		},
		{
			name:           "different content for existing file, want nil error",
			initialContent: []byte("content 1"),
			newContent:     []byte("content 2"),
			pathFunc: func(t *testing.T, initialContent []byte) string {
				return utiltest.WriteToTempFileMust(t, "test_file", initialContent)
			},
			wantErr: nil,
		},
		{
			name:       "path is a directory, want error",
			newContent: []byte("content 1"),
			pathFunc: func(t *testing.T, initialContent []byte) string {
				return "/tmp"
			},
			wantErr: &os.PathError{Op: "open", Path: "/tmp", Err: syscall.EISDIR},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.pathFunc(t, tt.initialContent)

			err := writeIfChanged(ctx, tt.newContent, path)
			utiltest.AssertErrorMatchAndSkip(t, err, tt.wantErr)
			utiltest.AssertFileContents(t, path, string(tt.newContent))
		})
	}
}

// TestInstallRecipesHandlesInputs verifies that installRecipes correctly handles different EffectiveGuestPolicy inputs.
func TestInstallRecipesHandlesInputs(t *testing.T) {
	ctx := context.Background()
	uniqueSuffix := fmt.Sprintf("-%d", time.Now().UnixNano())

	tests := []struct {
		name    string
		egp     *agentendpointpb.EffectiveGuestPolicy
		wantErr error
	}{
		{
			name: "policy without recipes, want nil error",
			egp:  &agentendpointpb.EffectiveGuestPolicy{},
		},
		{
			name: "policy with nil software recipe, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: nil,
					},
				},
			},
		},
		{
			name: "policy with invalid recipe, want installing error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name: "failing-recipe" + uniqueSuffix,
							InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
								{
									Step: &agentendpointpb.SoftwareRecipe_Step_FileCopy{
										FileCopy: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
											ArtifactId:  "non-existent",
											Destination: "/tmp/dest",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: installRecipesError{
				errors: []error{
					fmt.Errorf("Error installing recipe: error running step 0 (CopyFile): could not find location for artifact \"non-existent\""),
				},
			},
		},
		{
			name: "valid policy with one recipe, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name:    "success-recipe" + uniqueSuffix,
							Version: "1.0.0",
						},
					},
				},
			},
		},
		{
			name: "valid policy with multiple recipes, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name:    "recipe-1" + uniqueSuffix,
							Version: "1.0.0",
						},
					},
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name:    "recipe-2" + uniqueSuffix,
							Version: "2.1.0",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := installRecipes(ctx, tt.egp)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestSetConfig verifies that setConfig handles different package managers and their configurations.
func TestSetConfig(t *testing.T) {
	ctx := context.Background()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)

	dpkgQueryArgs := []string{"-W", "-f", "\\{\"architecture\":\"${Architecture}\",\"package\":\"${Package}\",\"source_name\":\"${source:Package}\",\"source_version\":\"${source:Version}\",\"status\":\"${db:Status-Status}\",\"version\":\"${Version}\"\\}\n"}
	rpmQueryArgs := []string{"--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-a"}
	aptUpgradableArgs := []string{"--just-print", "-qq", "dist-upgrade"}
	yumCheckUpdateArgs := []string{"check-update", "--assumeyes"}
	yumListUpdatesArgs := []string{"update", "--assumeno", "--color=never"}
	zypperListUpdatesArgs := []string{"--gpg-auto-import-keys", "-q", "list-updates"}
	aptEnv := []string{"DEBIAN_FRONTEND=noninteractive"}

	yumCheckUpdateErr := exec.Command("/bin/bash", "-c", "exit 100").Run()

	tests := []struct {
		name             string
		egp              *agentendpointpb.EffectiveGuestPolicy
		aptExists        bool
		yumExists        bool
		zypperExists     bool
		googetExists     bool
		expectedCommands []utiltest.ExpectedCommand
		wantErr          error
	}{
		{
			name:    "empty policy, want nil error",
			egp:     &agentendpointpb.EffectiveGuestPolicy{},
			wantErr: nil,
		},
		{
			name: "apt install package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_APT}},
				},
			},
			aptExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "update"),
					Envs: aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p1"),
					Envs: aptEnv,
				},
			},
			wantErr: nil,
		},
		{
			name: "apt install manager not found, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_APT}},
				},
			},
			aptExists: false,
			wantErr:   nil,
		},
		{
			name: "apt update package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_UPDATED, Manager: agentendpointpb.Package_APT}},
				},
			},
			aptExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "update"),
					Envs: aptEnv,
				},
				{
					Cmd:    exec.Command("/usr/bin/apt-get", aptUpgradableArgs...),
					Stdout: []byte("Inst p1 [1.0] (2.0 repo [amd64])\n"),
					Envs:   aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p1"),
					Envs: aptEnv,
				},
			},
			wantErr: nil,
		},
		{
			name: "apt remove package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_REMOVED, Manager: agentendpointpb.Package_APT}},
				},
			},
			aptExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "remove", "-y", "p1"),
					Envs: aptEnv,
				},
			},
			wantErr: nil,
		},
		{
			name: "yum install package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_YUM}},
				},
			},
			yumExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "yum update package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_UPDATED, Manager: agentendpointpb.Package_YUM}},
				},
			},
			yumExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", yumCheckUpdateArgs...),
					Err: yumCheckUpdateErr,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", yumListUpdatesArgs...),
					Stdout: []byte("Updating:\n p1 x86_64 2.0 updates 100 k\n"),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "yum remove package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_REMOVED, Manager: agentendpointpb.Package_YUM}},
				},
			},
			yumExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "remove", "--assumeyes", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "zypper install package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_ZYPPER}},
				},
			},
			zypperExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "zypper update package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_UPDATED, Manager: agentendpointpb.Package_ZYPPER}},
				},
			},
			zypperExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd:    exec.Command("/usr/bin/zypper", zypperListUpdatesArgs...),
					Stdout: []byte("v | Repo | p1 | 1.0 | 2.0 | x86_64\n"),
				},
				{
					Cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "zypper remove package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_REMOVED, Manager: agentendpointpb.Package_ZYPPER}},
				},
			},
			zypperExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd: exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "googet install package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_GOO}},
				},
			},
			googetExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("googet.exe", "installed"),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("googet.exe", "-noconfirm", "install", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "googet update package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_UPDATED, Manager: agentendpointpb.Package_GOO}},
				},
			},
			googetExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("googet.exe", "installed"),
					Stdout: []byte("p1.x86_64 1.0\n"),
				},
				{
					Cmd:    exec.Command("googet.exe", "update"),
					Stdout: []byte("p1.noarch, 1.0 --> 2.0 from repo\n"),
				},
				{
					Cmd: exec.Command("googet.exe", "-noconfirm", "install", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "googet remove package p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_REMOVED, Manager: agentendpointpb.Package_GOO}},
				},
			},
			googetExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("googet.exe", "installed"),
					Stdout: []byte("p1.x86_64 1.0\n"),
				},
				{
					Cmd: exec.Command("googet.exe", "-noconfirm", "remove", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "package ANY install p1, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_ANY}},
				},
			},
			aptExists: true, yumExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "update"),
					Envs: aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p1"),
					Envs: aptEnv,
				},
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1"),
				},
			},
			wantErr: nil,
		},
		{
			name: "all repositories configured, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				PackageRepositories: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackageRepository{
					{PackageRepository: &agentendpointpb.PackageRepository{Repository: &agentendpointpb.PackageRepository_Apt{Apt: &agentendpointpb.AptRepository{Uri: "http://repo"}}}},
					{PackageRepository: &agentendpointpb.PackageRepository{Repository: &agentendpointpb.PackageRepository_Yum{Yum: &agentendpointpb.YumRepository{Id: "repo", BaseUrl: "http://repo"}}}},
					{PackageRepository: &agentendpointpb.PackageRepository{Repository: &agentendpointpb.PackageRepository_Zypper{Zypper: &agentendpointpb.ZypperRepository{Id: "repo", BaseUrl: "http://repo"}}}},
					{PackageRepository: &agentendpointpb.PackageRepository{Repository: &agentendpointpb.PackageRepository_Goo{Goo: &agentendpointpb.GooRepository{Name: "repo", Url: "http://repo"}}}},
				},
			},
			aptExists: true, yumExists: true, zypperExists: true, googetExists: true,
			wantErr: nil,
		},
		{
			name: "apt install p1 with failure, want installing error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_APT}},
				},
			},
			aptExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "update"),
					Envs: aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p1"),
					Envs: aptEnv,
					Err:  fmt.Errorf("individual install error"),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p1"),
					Envs: aptEnv,
					Err:  fmt.Errorf("individual install error"),
				},
			},
			wantErr: setConfigError{
				errors: []error{
					fmt.Errorf("Error performing apt changes: error installing apt packages: Error installing apt package: p1. Error details: error running /usr/bin/apt-get with args [\"install\" \"-y\" \"p1\"]: individual install error, stdout: \"\", stderr: \"\""),
				},
			},
		},
		{
			name: "yum install p1 with failure, want installing error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_YUM}},
				},
			},
			yumExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p1"),
					Err: fmt.Errorf("yum error"),
				},
			},
			wantErr: setConfigError{
				errors: []error{
					fmt.Errorf("Error performing yum changes: error installing yum packages: error running /usr/bin/yum with args [\"install\" \"--assumeyes\" \"p1\"]: yum error, stdout: \"\", stderr: \"\""),
				},
			},
		},
		{
			name: "zypper install p1 with failure, want installing error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_ZYPPER}},
				},
			},
			zypperExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "p1"),
					Err: fmt.Errorf("zypper error"),
				},
			},
			wantErr: setConfigError{
				errors: []error{
					fmt.Errorf("Error performing zypper changes: error installing zypper packages: error running /usr/bin/zypper with args [\"--gpg-auto-import-keys\" \"--non-interactive\" \"install\" \"--auto-agree-with-licenses\" \"p1\"]: zypper error, stdout: \"\", stderr: \"\""),
				},
			},
		},
		{
			name: "googet install p1 with failure, want installing  error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{Package: &agentendpointpb.Package{Name: "p1", DesiredState: agentendpointpb.DesiredState_INSTALLED, Manager: agentendpointpb.Package_GOO}},
				},
			},
			googetExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("googet.exe", "installed"),
					Stdout: []byte(""),
				},
				{
					Cmd: exec.Command("googet.exe", "-noconfirm", "install", "p1"),
					Err: fmt.Errorf("googet error"),
				},
			},
			wantErr: setConfigError{
				errors: []error{
					fmt.Errorf("Error performing googet changes: error installing googet packages: error running googet.exe with args [\"-noconfirm\" \"install\" \"p1\"]: googet error, stdout: \"\", stderr: \"\""),
				},
			},
		},
		{
			name: "package ANY removed and unspecified manager/state, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				Packages: []*agentendpointpb.EffectiveGuestPolicy_SourcedPackage{
					{
						Package: &agentendpointpb.Package{
							Name:         "p1",
							DesiredState: agentendpointpb.DesiredState_REMOVED,
							Manager:      agentendpointpb.Package_ANY,
						},
					},
					{
						Package: &agentendpointpb.Package{
							Name:         "p2",
							DesiredState: agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED,
							Manager:      agentendpointpb.Package_MANAGER_UNSPECIFIED,
						},
					},
					{
						Package: &agentendpointpb.Package{
							Name:         "p3",
							DesiredState: agentendpointpb.DesiredState_UPDATED,
							Manager:      agentendpointpb.Package_ANY,
						},
					},
				},
			},
			aptExists: true, yumExists: true,
			expectedCommands: []utiltest.ExpectedCommand{
				{
					Cmd:    exec.Command("/usr/bin/dpkg-query", dpkgQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "update"),
					Envs: aptEnv,
				},
				{
					Cmd:    exec.Command("/usr/bin/apt-get", aptUpgradableArgs...),
					Stdout: []byte("Inst p3 [1.0] (2.0 repo [amd64])\n"),
					Envs:   aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "update"),
					Envs: aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p2"),
					Envs: aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "install", "-y", "p3"),
					Envs: aptEnv,
				},
				{
					Cmd:  exec.Command("/usr/bin/apt-get", "remove", "-y", "p1"),
					Envs: aptEnv,
				},
				{
					Cmd:    exec.Command("/usr/bin/rpmquery", rpmQueryArgs...),
					Stdout: []byte(`{"package":"p1","status":"installed"}`),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", yumCheckUpdateArgs...),
					Err: yumCheckUpdateErr,
				},
				{
					Cmd:    exec.Command("/usr/bin/yum", yumListUpdatesArgs...),
					Stdout: []byte("Updating:\n p3 x86_64 2.0 updates 100 k\n"),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p2"),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "p3"),
				},
				{
					Cmd: exec.Command("/usr/bin/yum", "remove", "--assumeyes", "p1"),
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSetConfigTest(t, tt.aptExists, tt.yumExists, tt.zypperExists, tt.googetExists, mockCommandRunner)

			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)
			err := setConfig(context.Background(), tt.egp)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

type policiesStubOsInfoProvider struct {
	osinfo osinfo.OSInfo
}

func (s policiesStubOsInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return s.osinfo, nil
}

// setupSetConfigTest sets up the environment for SetConfig tests by mocking the command runner.
func setupSetConfigTest(t *testing.T, apt, yum, zypper, googet bool, runner *utilmocks.MockCommandRunner) {
	utiltest.OverrideVariable(t, &packages.AptExists, apt)
	utiltest.OverrideVariable(t, &packages.YumExists, yum)
	utiltest.OverrideVariable(t, &packages.ZypperExists, zypper)
	utiltest.OverrideVariable(t, &packages.GooGetExists, googet)
	utiltest.OverrideVariable(t, &osInfoProvider, (osinfo.Provider)(policiesStubOsInfoProvider{
		osinfo: osinfo.OSInfo{ShortName: "debian", Version: "11"},
	}))

	tmpDir := t.TempDir()
	utiltest.OverrideVariable(t, &googetRepoFilePath, func() string { return filepath.Join(tmpDir, "googet.repo") })
	utiltest.OverrideVariable(t, &aptRepoFilePath, func() string { return filepath.Join(tmpDir, "apt.list") })
	utiltest.OverrideVariable(t, &yumRepoFilePath, func() string { return filepath.Join(tmpDir, "yum.repo") })
	utiltest.OverrideVariable(t, &zypperRepoFilePath, func() string { return filepath.Join(tmpDir, "zypper.repo") })

	utiltest.OverrideVariable(t, &retry, func(ctx context.Context, timeout time.Duration, desc string, f func() error) error {
		return f()
	})

	packages.SetCommandRunner(runner)
	packages.SetPtyCommandRunner(runner)
}
