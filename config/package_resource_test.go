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
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

func init() {
	packages.YumExists = true
	packages.AptExists = true
	packages.GooGetExists = true
	packages.DpkgExists = true
	packages.RPMExists = true
	packages.ZypperExists = true
	packages.MSIExecExists = true
}

var (
	aptInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Apt{
			Apt: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}}
	aptRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Apt{
			Apt: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}}
	debInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb_{
			Deb: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}}
	debRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb_{
			Deb: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}}
	googetInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Googet{
			Googet: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}}
	googetRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Googet{
			Googet: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}}
	msiInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Msi{
			Msi: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}}
	msiRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Msi{
			Msi: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}}
	yumInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Yum{
			Yum: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}}
	yumRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Yum{
			Yum: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}}
	zypperInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper_{
			Zypper: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}}
	zypperRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper_{
			Zypper: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}}
	rpmInstalledPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Rpm{
			Rpm: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}}
	rpmRemovedPR = &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
		DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Rpm{
			Rpm: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}}
)

func TestPackageResourceValidate(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name    string
		wantErr bool
		prpb    *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource
		wantMP  ManagedPackage
	}{
		{
			"Blank",
			true,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{},
			ManagedPackage{},
		},
		{
			"AptInstalled",
			false,
			aptInstalledPR,
			ManagedPackage{Apt: &AptPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}},
		},
		{
			"AptRemoved",
			false,
			aptRemovedPR,
			ManagedPackage{Apt: &AptPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}},
		},
		{
			"DebInstalled",
			false,
			debInstalledPR,
			ManagedPackage{Deb: &DebPackage{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
					Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"DebRemoved",
			false,
			debRemovedPR,
			ManagedPackage{Deb: &DebPackage{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
					Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"GoGetInstalled",
			false,
			googetInstalledPR,
			ManagedPackage{GooGet: &GooGetPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}},
		},
		{
			"GooGetRemoved",
			false,
			googetRemovedPR,
			ManagedPackage{GooGet: &GooGetPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}},
		},
		{
			"MSIInstalled",
			false,
			msiInstalledPR,
			ManagedPackage{MSI: &MSIPackage{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
					Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"MSIRemoved",
			false,
			msiRemovedPR,
			ManagedPackage{MSI: &MSIPackage{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
					Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"YumInstalled",
			false,
			yumInstalledPR,
			ManagedPackage{Yum: &YumPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}},
		},
		{
			"YumRemoved",
			false,
			yumRemovedPR,
			ManagedPackage{Yum: &YumPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}},
		},
		{
			"ZypperInstalled",
			false,
			zypperInstalledPR,
			ManagedPackage{Zypper: &ZypperPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}},
		},
		{
			"ZypperRemoved",
			false,
			zypperRemovedPR,
			ManagedPackage{Zypper: &ZypperPackage{
				DesiredState:    agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}},
		},
		{
			"RPMInstalled",
			false,
			rpmInstalledPR,
			ManagedPackage{RPM: &RPMPackage{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
					Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"RPMRemoved",
			false,
			rpmRemovedPR,
			ManagedPackage{RPM: &RPMPackage{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
					Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				ApplyConfigTask_Config_Resource: &agentendpointpb.ApplyConfigTask_Config_Resource{
					ResourceType: &agentendpointpb.ApplyConfigTask_Config_Resource_Pkg{Pkg: tt.prpb},
				},
			}
			err := pr.Validate(ctx)
			if err != nil && !tt.wantErr {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err == nil && tt.wantErr {
				t.Fatal("Expected error and did not get one.")
			}

			wantMR := &ManagedResources{Packages: []ManagedPackage{tt.wantMP}}
			if err != nil {
				wantMR = nil
			}

			if diff := cmp.Diff(wantMR, pr.ManagedResources(), protocmp.Transform()); diff != "" {
				t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantMP, pr.resource.(*packageResouce).managedPackage, protocmp.Transform()); diff != "" {
				t.Errorf("packageResouce does not match expectation: (-got +want)\n%s", diff)
			}
		})
	}
}

func TestPopulateInstalledCache(t *testing.T) {
	ctx := context.Background()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)
	mockCommandRunner.EXPECT().Run(ctx, exec.Command("googet.exe", "installed")).Return([]byte("Installed Packages:\nfoo.x86_64 1.2.3@4\nbar.noarch 1.2.3@4"), nil, nil).Times(1)

	if err := populateInstalledCache(ctx, ManagedPackage{GooGet: &GooGetPackage{}}); err != nil {
		t.Fatalf("Unexpected error from populateInstalledCache: %v", err)
	}

	want := map[string]struct{}{"foo": {}, "bar": {}}
	if diff := cmp.Diff(gooInstalled.cache, want); diff != "" {
		t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
	}
}

func TestPackageResourceCheckState(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name               string
		installedCache     map[string]struct{}
		cachePointer       *packageCache
		prpb               *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource
		wantInDesiredState bool
	}{
		// We only need to test the full set once as all the logic is shared.
		{
			"AptInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			&aptInstalled,
			aptInstalledPR,
			true,
		},
		{
			"AptInstalledNeedsRemoved",
			map[string]struct{}{"foo": {}},
			&aptInstalled,
			aptRemovedPR,
			false,
		},
		{
			"AptRemovedNeedsInstalled",
			map[string]struct{}{},
			&aptInstalled,
			aptInstalledPR,
			false,
		},
		{
			"AptRemovedNeedsRemoved",
			map[string]struct{}{},
			&aptInstalled,
			aptRemovedPR,
			true,
		},

		// For the rest of the package types we only need to test one scenario.
		{
			"GooGetInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			&gooInstalled,
			googetInstalledPR,
			true,
		},
		{
			"YUMInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			&yumInstalled,
			yumInstalledPR,
			true,
		},
		{
			"ZypperInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			&zypperInstalled,
			zypperInstalledPR,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				ApplyConfigTask_Config_Resource: &agentendpointpb.ApplyConfigTask_Config_Resource{
					ResourceType: &agentendpointpb.ApplyConfigTask_Config_Resource_Pkg{Pkg: tt.prpb},
				},
			}
			// Run validate first to make sure everything gets setup correctly.
			// This adds complexity to this 'unit' test and turns it into more
			// of a integration test but reduces overall test functions and gives
			// us good coverage.
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			tt.cachePointer.cache = tt.installedCache
			tt.cachePointer.refreshed = time.Now()
			if err := pr.CheckState(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.wantInDesiredState != pr.InDesiredState() {
				t.Fatalf("Unexpected InDesiredState, want: %t, got: %t", tt.wantInDesiredState, pr.InDesiredState())
			}
		})
	}
}

func TestPackageResourceEnforceState(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		name        string
		prpb        *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource
		expectedCmd *exec.Cmd
	}{
		{
			"AptInstalled",
			aptInstalledPR,
			func() *exec.Cmd {
				cmd := exec.Command("/usr/bin/apt-get", "install", "-y", "foo")
				cmd.Env = append(os.Environ(),
					"DEBIAN_FRONTEND=noninteractive",
				)
				return cmd
			}(),
		},
		{
			"AptRemoved",
			aptRemovedPR,
			func() *exec.Cmd {
				cmd := exec.Command("/usr/bin/apt-get", "remove", "-y", "foo")
				cmd.Env = append(os.Environ(),
					"DEBIAN_FRONTEND=noninteractive",
				)
				return cmd
			}(),
		},
		{
			"GooGetInstalled",
			googetInstalledPR,
			exec.Command("googet.exe", "-noconfirm", "install", "foo"),
		},
		{
			"GooGetRemoved",
			googetRemovedPR,
			exec.Command("googet.exe", "-noconfirm", "remove", "foo"),
		},
		{
			"YumInstalled",
			yumInstalledPR,
			exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo"),
		},
		{
			"YumRemoved",
			yumRemovedPR,
			exec.Command("/usr/bin/yum", "remove", "--assumeyes", "foo"),
		},
		{
			"ZypperInstalled",
			zypperInstalledPR,
			exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "foo"),
		},
		{
			"ZypperRemoved",
			zypperRemovedPR,
			exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "foo"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				ApplyConfigTask_Config_Resource: &agentendpointpb.ApplyConfigTask_Config_Resource{
					ResourceType: &agentendpointpb.ApplyConfigTask_Config_Resource_Pkg{Pkg: tt.prpb},
				},
			}
			// Run Validate first to make sure everything gets setup correctly.
			// This adds complexity to this 'unit' test and turns it into more
			// of a integration test but reduces overall test functions and gives
			// us good coverage.
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			packages.SetCommandRunner(mockCommandRunner)
			mockCommandRunner.EXPECT().Run(ctx, tt.expectedCmd)

			if err := pr.EnforceState(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}
