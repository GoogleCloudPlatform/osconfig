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
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
)

var (
	aptInstalledPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Apt{
			Apt: &agentendpointpb.OSPolicy_Resource_PackageResource_APT{Name: "foo"}}}
	aptRemovedPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Apt{
			Apt: &agentendpointpb.OSPolicy_Resource_PackageResource_APT{Name: "foo"}}}
	googetInstalledPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Googet{
			Googet: &agentendpointpb.OSPolicy_Resource_PackageResource_GooGet{Name: "foo"}}}
	googetRemovedPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Googet{
			Googet: &agentendpointpb.OSPolicy_Resource_PackageResource_GooGet{Name: "foo"}}}
	yumInstalledPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Yum{
			Yum: &agentendpointpb.OSPolicy_Resource_PackageResource_YUM{Name: "foo"}}}
	yumRemovedPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Yum{
			Yum: &agentendpointpb.OSPolicy_Resource_PackageResource_YUM{Name: "foo"}}}
	zypperInstalledPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Zypper_{
			Zypper: &agentendpointpb.OSPolicy_Resource_PackageResource_Zypper{Name: "foo"}}}
	zypperRemovedPR = &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Zypper_{
			Zypper: &agentendpointpb.OSPolicy_Resource_PackageResource_Zypper{Name: "foo"}}}
)

type fakeCommandRunner struct{}

func (m *fakeCommandRunner) Run(_ context.Context, _ *exec.Cmd) ([]byte, []byte, error) {
	return nil, nil, nil
}

func TestPackageResourceValidate(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "foo")
	if err := ioutil.WriteFile(tmpFile, nil, 0644); err != nil {
		t.Fatal(err)
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	var tests = []struct {
		name              string
		wantErr           bool
		prpb              *agentendpointpb.OSPolicy_Resource_PackageResource
		wantMP            ManagedPackage
		expectedCmd       *exec.Cmd
		expectedCmdReturn []byte
	}{
		{
			"Blank",
			true,
			&agentendpointpb.OSPolicy_Resource_PackageResource{},
			ManagedPackage{},
			nil,
			nil,
		},
		{
			"AptInstalled",
			false,
			aptInstalledPR,
			ManagedPackage{Apt: &AptPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_APT{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"AptRemoved",
			false,
			aptRemovedPR,
			ManagedPackage{Apt: &AptPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_APT{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"DebInstalled",
			false,
			&agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb_{
					Deb: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{
						Source: &agentendpointpb.OSPolicy_Resource_File{
							Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: tmpFile}}}}},
			ManagedPackage{Deb: &DebPackage{
				localPath: tmpFile,
				name:      "foo",
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{
					Source: &agentendpointpb.OSPolicy_Resource_File{
						Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: tmpFile}}}}},
			exec.Command("/usr/bin/dpkg-deb", "-I", tmpFile),
			[]byte("Package: foo\nVersion: 1:1dummy-g1\nArchitecture: amd64"),
		},
		{
			"GoGetInstalled",
			false,
			googetInstalledPR,
			ManagedPackage{GooGet: &GooGetPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_GooGet{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"GooGetRemoved",
			false,
			googetRemovedPR,
			ManagedPackage{GooGet: &GooGetPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_GooGet{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"MSIInstalled",
			false,
			&agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Msi{
					Msi: &agentendpointpb.OSPolicy_Resource_PackageResource_MSI{
						Source: &agentendpointpb.OSPolicy_Resource_File{
							Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: tmpFile}}}}},
			ManagedPackage{MSI: &MSIPackage{
				localPath: tmpFile,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_MSI{
					Source: &agentendpointpb.OSPolicy_Resource_File{
						Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: tmpFile}}}}},
			nil,
			nil,
		},
		{
			"YumInstalled",
			false,
			yumInstalledPR,
			ManagedPackage{Yum: &YumPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_YUM{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"YumRemoved",
			false,
			yumRemovedPR,
			ManagedPackage{Yum: &YumPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_YUM{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"ZypperInstalled",
			false,
			zypperInstalledPR,
			ManagedPackage{Zypper: &ZypperPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_Zypper{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"ZypperRemoved",
			false,
			zypperRemovedPR,
			ManagedPackage{Zypper: &ZypperPackage{
				DesiredState:    agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_Zypper{Name: "foo"}}},
			nil,
			nil,
		},
		{
			"RPMInstalled",
			false,
			&agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Rpm{
					Rpm: &agentendpointpb.OSPolicy_Resource_PackageResource_RPM{
						Source: &agentendpointpb.OSPolicy_Resource_File{
							Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: tmpFile}}}}},
			ManagedPackage{RPM: &RPMPackage{
				localPath: tmpFile,
				name:      "gcc",
				PackageResource: &agentendpointpb.OSPolicy_Resource_PackageResource_RPM{
					Source: &agentendpointpb.OSPolicy_Resource_File{
						Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: tmpFile}}}}},
			exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpFile),
			[]byte("{\"architecture\":\"x86_64\",\"package\":\"gcc\",\"source_name\":\"gcc-11.4.1-3.el9.src.rpm\",\"version\":\"11.4.1-3.el9\"}"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Pkg{Pkg: tt.prpb},
				},
			}
			defer pr.Cleanup(ctx)

			if tt.expectedCmd != nil {
				mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(tt.expectedCmd)).Return(tt.expectedCmdReturn, nil, nil)
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

			opts := []cmp.Option{protocmp.Transform(), cmp.AllowUnexported(ManagedPackage{}), cmp.AllowUnexported(DebPackage{}), cmp.AllowUnexported(RPMPackage{}), cmp.AllowUnexported(MSIPackage{})}
			if diff := cmp.Diff(pr.ManagedResources(), wantMR, opts...); diff != "" {
				t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
			}
			if diff := cmp.Diff(pr.resource.(*packageResouce).managedPackage, tt.wantMP, opts...); diff != "" {
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
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(exec.Command("googet.exe", "installed"))).Return([]byte("Installed Packages:\nfoo.x86_64 1.2.3@4\nbar.noarch 1.2.3@4"), nil, nil).Times(1)

	if err := populateInstalledCache(ctx, ManagedPackage{GooGet: &GooGetPackage{}}); err != nil {
		t.Fatalf("Unexpected error from populateInstalledCache: %v", err)
	}

	want := map[string]struct{}{"foo": {}, "bar": {}}
	if diff := cmp.Diff(gooInstalled.cache, want); diff != "" {
		t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
	}
}

func TestPackageResourceCheckState(t *testing.T) {
	ctx := t.Context()

	tmpFile := utiltest.WriteToTempFileMust(t, "foo.deb", []byte{})
	tmpRpmFile := utiltest.WriteToTempFileMust(t, "foo.rpm", []byte{})

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	tests := []struct {
		name               string
		setup              func(t *testing.T)
		cachePointer       *packageCache
		packageResourcePB  *agentendpointpb.OSPolicy_Resource_PackageResource
		wantInDesiredState bool
		wantErr            error
	}{
		{
			name: "valid Apt installed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				aptInstalled.cache = map[string]struct{}{"foo": {}}
				aptInstalled.refreshed = time.Now()
			},
			cachePointer:       aptInstalled,
			packageResourcePB:  aptInstalledPR,
			wantInDesiredState: true,
			wantErr:            nil,
		},
		{
			name: "valid Apt installed package not in desired state, expect CheckState false",
			setup: func(t *testing.T) {
				aptInstalled.cache = map[string]struct{}{"foo": {}}
				aptInstalled.refreshed = time.Now()
			},
			cachePointer:       aptInstalled,
			packageResourcePB:  aptRemovedPR,
			wantInDesiredState: false,
			wantErr:            nil,
		},
		{
			name: "valid Apt removed package not in desired state, expect CheckState false",
			setup: func(t *testing.T) {
				aptInstalled.cache = map[string]struct{}{}
				aptInstalled.refreshed = time.Now()
			},
			cachePointer:       aptInstalled,
			packageResourcePB:  aptInstalledPR,
			wantInDesiredState: false,
			wantErr:            nil,
		},
		{
			name: "valid Apt removed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				aptInstalled.cache = map[string]struct{}{}
				aptInstalled.refreshed = time.Now()
			},
			cachePointer:       aptInstalled,
			packageResourcePB:  aptRemovedPR,
			wantInDesiredState: true,
			wantErr:            nil,
		},
		// For the rest of the package types we only need to test one scenario.
		{
			name: "valid GooGet installed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				gooInstalled.cache = map[string]struct{}{"foo": {}}
				gooInstalled.refreshed = time.Now()
			},
			cachePointer:       gooInstalled,
			packageResourcePB:  googetInstalledPR,
			wantInDesiredState: true,
			wantErr:            nil,
		},
		{
			name: "valid Yum installed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				yumInstalled.cache = map[string]struct{}{"foo": {}}
				yumInstalled.refreshed = time.Now()
			},
			cachePointer:       yumInstalled,
			packageResourcePB:  yumInstalledPR,
			wantInDesiredState: true,
			wantErr:            nil,
		},
		{
			name: "valid Zypper installed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				zypperInstalled.cache = map[string]struct{}{"foo": {}}
				zypperInstalled.refreshed = time.Now()
			},
			cachePointer:       zypperInstalled,
			packageResourcePB:  zypperInstalledPR,
			wantInDesiredState: true,
			wantErr:            nil,
		},
		{
			name: "valid Deb installed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				dpkgCmd := exec.Command("/usr/bin/dpkg-deb", "-I", tmpFile)
				mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(dpkgCmd)).Return([]byte("Package: foo\nVersion: 1:1dummy-g1\nArchitecture: amd64"), nil, nil)
				debInstalled.cache = map[string]struct{}{"foo": {}}
				debInstalled.refreshed = time.Now()
			},
			cachePointer:       debInstalled,
			packageResourcePB:  createTestDebPackageResource(tmpFile, false),
			wantInDesiredState: true,
			wantErr:            nil,
		},
		{
			name: "valid RPM installed package in desired state, expect CheckState true",
			setup: func(t *testing.T) {
				rpmqueryCmd := exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpRpmFile)
				mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(rpmqueryCmd)).Return([]byte("{\"architecture\":\"x86_64\",\"package\":\"foo\",\"source_name\":\"foo.src.rpm\",\"version\":\"1.0\"}"), nil, nil)
				rpmInstalled.cache = map[string]struct{}{"foo": {}}
				rpmInstalled.refreshed = time.Now()
			},
			cachePointer:       rpmInstalled,
			packageResourcePB:  createRPMPackageResource(tmpRpmFile, false),
			wantInDesiredState: true,
			wantErr:            nil,
		},
		{
			name: "unspecified desired state, expect error",
			setup: func(t *testing.T) {
				aptInstalled.cache = map[string]struct{}{"foo": {}}
				aptInstalled.refreshed = time.Now()
			},
			packageResourcePB: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_DESIRED_STATE_UNSPECIFIED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Apt{
					Apt: &agentendpointpb.OSPolicy_Resource_PackageResource_APT{Name: "foo"},
				},
			},
			cachePointer:       aptInstalled,
			wantInDesiredState: false,
			wantErr:            errors.New("DesiredState field not set or references state: \"DESIRED_STATE_UNSPECIFIED\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Pkg{Pkg: tt.packageResourcePB},
				},
			}
			defer pr.Cleanup(ctx)
			// Run validate first to make sure everything gets setup correctly.
			// This adds complexity to this 'unit' test and turns it into more
			// of a integration test but reduces overall test functions and gives
			// us good coverage.
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			gotErr := pr.CheckState(ctx)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, pr.InDesiredState(), tt.wantInDesiredState)
		})
	}
}

func createTestDebPackageResource(localPath string, pullDeps bool) *agentendpointpb.OSPolicy_Resource_PackageResource {
	debPR := &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{
		PullDeps: pullDeps,
		Source: &agentendpointpb.OSPolicy_Resource_File{
			Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{
				LocalPath: localPath,
			},
		},
	}
	return &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState:  agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb_{Deb: debPR},
	}
}

func createRPMPackageResource(localPath string, pullDeps bool) *agentendpointpb.OSPolicy_Resource_PackageResource {
	rpmPR := &agentendpointpb.OSPolicy_Resource_PackageResource_RPM{
		PullDeps: pullDeps,
		Source:   &agentendpointpb.OSPolicy_Resource_File{Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: localPath}},
	}
	return &agentendpointpb.OSPolicy_Resource_PackageResource{
		DesiredState:  agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
		SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Rpm{Rpm: rpmPR},
	}
}

func TestPackageResourceEnforceState(t *testing.T) {
	ctx := t.Context()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	tmpFile := utiltest.WriteToTempFileMust(t, "foo.deb", []byte{})
	tmpRpmFile := utiltest.WriteToTempFileMust(t, "foo.rpm", []byte{})

	var tests = []struct {
		name              string
		packageResourcePB *agentendpointpb.OSPolicy_Resource_PackageResource
		cachePointer      *packageCache
		expectedCommands  []utiltest.ExpectedCommand
		setup             func(t *testing.T)
		wantErr           error
	}{
		{
			name:              "valid Apt installed package, expect successful enforcement",
			packageResourcePB: aptInstalledPR,
			cachePointer:      aptInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/apt-get", "update"), Envs: []string{"DEBIAN_FRONTEND=noninteractive"}},
				{Cmd: exec.Command("/usr/bin/apt-get", "install", "-y", "foo"), Envs: []string{"DEBIAN_FRONTEND=noninteractive"}},
			},
			setup:   func(t *testing.T) { aptInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Apt removed package, expect successful enforcement",
			packageResourcePB: aptRemovedPR,
			cachePointer:      aptInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/apt-get", "remove", "-y", "foo"), Envs: []string{"DEBIAN_FRONTEND=noninteractive"}},
			},
			setup:   func(t *testing.T) { aptInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid GooGet installed package, expect successful enforcement",
			packageResourcePB: googetInstalledPR,
			cachePointer:      gooInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("googet.exe", "-noconfirm", "install", "foo")},
			},
			setup:   func(t *testing.T) { gooInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid GooGet removed package, expect successful enforcement",
			packageResourcePB: googetRemovedPR,
			cachePointer:      gooInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("googet.exe", "-noconfirm", "remove", "foo")},
			},
			setup:   func(t *testing.T) { gooInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Yum installed package, expect successful enforcement",
			packageResourcePB: yumInstalledPR,
			cachePointer:      yumInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo")},
			},
			setup:   func(t *testing.T) { yumInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Yum removed package, expect successful enforcement",
			packageResourcePB: yumRemovedPR,
			cachePointer:      yumInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/yum", "remove", "--assumeyes", "foo")},
			},
			setup:   func(t *testing.T) { yumInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Zypper installed package, expect successful enforcement",
			packageResourcePB: zypperInstalledPR,
			cachePointer:      zypperInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "foo")},
			},
			setup:   func(t *testing.T) { zypperInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Zypper removed package, expect successful enforcement",
			packageResourcePB: zypperRemovedPR,
			cachePointer:      zypperInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "foo")},
			},
			setup:   func(t *testing.T) { zypperInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Deb package with PullDeps false, expect dpkg command",
			packageResourcePB: createTestDebPackageResource(tmpFile, false),
			cachePointer:      debInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/dpkg-deb", "-I", tmpFile), Stdout: []byte("Package: foo\nVersion: 1:1dummy-g1\nArchitecture: amd64")},
				{Cmd: exec.Command("/usr/bin/dpkg", "--install", tmpFile)},
			},
			setup:   func(t *testing.T) { debInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid Deb package with PullDeps true, expect apt-get command",
			packageResourcePB: createTestDebPackageResource(tmpFile, true),
			cachePointer:      debInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/dpkg-deb", "-I", tmpFile), Stdout: []byte("Package: foo\nVersion: 1:1dummy-g1\nArchitecture: amd64")},
				{Cmd: exec.Command("/usr/bin/apt-get", "install", "-y", tmpFile), Envs: []string{"DEBIAN_FRONTEND=noninteractive"}},
			},
			setup:   func(t *testing.T) { debInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid RPM package with PullDeps false, expect rpm command",
			packageResourcePB: createRPMPackageResource(tmpRpmFile, false),
			cachePointer:      rpmInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpRpmFile), Stdout: []byte("{\"architecture\":\"x86_64\",\"package\":\"foo\",\"source_name\":\"foo.src.rpm\",\"version\":\"1.0\"}")},
				{Cmd: exec.Command("/bin/rpm", "--upgrade", "--replacepkgs", "-v", tmpRpmFile)},
			},
			setup:   func(t *testing.T) { rpmInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: nil,
		},
		{
			name:              "valid RPM package with PullDeps true on Yum system, expect yum command",
			packageResourcePB: createRPMPackageResource(tmpRpmFile, true),
			cachePointer:      rpmInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpRpmFile), Stdout: []byte("{\"architecture\":\"x86_64\",\"package\":\"foo\",\"source_name\":\"foo.src.rpm\",\"version\":\"1.0\"}")},
				{Cmd: exec.Command("/usr/bin/yum", "install", "--assumeyes", tmpRpmFile)},
			},
			setup: func(t *testing.T) {
				utiltest.OverrideVariable(t, &packages.YumExists, true)
				rpmInstalled.cache = map[string]struct{}{"foo": {}}
			},
			wantErr: nil,
		},
		{
			name:              "valid RPM package with PullDeps true on Zypper system, expect zypper command",
			packageResourcePB: createRPMPackageResource(tmpRpmFile, true),
			cachePointer:      rpmInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpRpmFile), Stdout: []byte("{\"architecture\":\"x86_64\",\"package\":\"foo\",\"source_name\":\"foo.src.rpm\",\"version\":\"1.0\"}")},
				{Cmd: exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", tmpRpmFile)},
			},
			setup: func(t *testing.T) {
				utiltest.OverrideVariable(t, &packages.YumExists, false)
				utiltest.OverrideVariable(t, &packages.ZypperExists, true)
				rpmInstalled.cache = map[string]struct{}{"foo": {}}
			},
			wantErr: nil,
		},
		{
			name:              "RPM package with PullDeps true without package managers, expect error",
			packageResourcePB: createRPMPackageResource(tmpRpmFile, true),
			cachePointer:      rpmInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpRpmFile), Stdout: []byte("{\"architecture\":\"x86_64\",\"package\":\"foo\",\"source_name\":\"foo.src.rpm\",\"version\":\"1.0\"}")},
			},
			setup: func(t *testing.T) {
				utiltest.OverrideVariable(t, &packages.YumExists, false)
				utiltest.OverrideVariable(t, &packages.ZypperExists, false)
				rpmInstalled.cache = map[string]struct{}{"foo": {}}
			},
			wantErr: errors.New("cannot install rpm \"foo\" with 'PullDeps' option as neither yum or zypper exist on system"),
		},
		{
			name:              "Apt install command failure, expect error",
			packageResourcePB: aptInstalledPR,
			cachePointer:      aptInstalled,
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/apt-get", "update"), Envs: []string{"DEBIAN_FRONTEND=noninteractive"}, Err: errors.New("apt-get update failed")},
			},
			setup:   func(t *testing.T) { aptInstalled.cache = map[string]struct{}{"foo": {}} },
			wantErr: errors.New("error installing apt package \"foo\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Pkg{Pkg: tt.packageResourcePB},
				},
			}
			defer pr.Cleanup(ctx)

			// Run Validate first to make sure everything gets setup correctly.
			// This adds complexity to this 'unit' test and turns it into more
			// of a integration test but reduces overall test functions and gives
			// us good coverage.
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			gotErr := pr.EnforceState(ctx)
			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, tt.cachePointer.cache, map[string]struct{}(nil))
		})
	}
}

func TestPackageInfoCache(t *testing.T) {
	ctx := context.Background()
	pkgInfo := &packages.PkgInfo{Name: "name", Arch: "arch", Version: "version"}
	pkgFile := &agentendpointpb.OSPolicy_Resource_File{AllowInsecure: true, Type: &agentendpointpb.OSPolicy_Resource_File_Gcs_{Gcs: &agentendpointpb.OSPolicy_Resource_File_Gcs{Bucket: "bucket", Object: "object", Generation: 123456789}}}
	wantKey := "IAESFQoGYnVja2V0EgZvYmplY3QYlZrvOg"

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	packageInfoCacheFile = filepath.Join(tmpDir, "file.cache")
	packageInfoCacheStore = nil

	updatePackageInfoCache(ctx, pkgInfo, pkgFile)
	if err := savePackageInfoCache(ctx); err != nil {
		t.Fatal(err)
	}
	got := getPackageInfoFromCache(ctx, pkgFile)
	if !reflect.DeepEqual(got, pkgInfo) {
		t.Errorf("Did not get expected cache data, got: %+v, want: %+v", got, pkgInfo)
	}
	if _, ok := packageInfoCacheStore[wantKey]; !ok {
		t.Errorf("Cache did not contain expected key, cache: %+v, want: %q", packageInfoCacheStore, wantKey)
	}

	// Now test save and reload.
	savePackageInfoCache(ctx)
	if packageInfoCacheStore != nil {
		t.Fatal("expected packageInfoCacheStore to be nil")
	}
	loadPackageInfoCache(ctx)
	if _, ok := packageInfoCacheStore[wantKey]; !ok {
		t.Errorf("Cache did not contain expected key, cache: %+v, want: %q", packageInfoCacheStore, wantKey)
	}
}

func TestUpdatePackageInfoCacheTimeout(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	packageInfoCacheFile = filepath.Join(tmpDir, "file.cache")
	packageInfoCacheStore = nil

	cache := packageInfoCache{}
	key := "IAESFQoGYnVja2V0EgZvYmplY3QYlZrvOg"
	info := &packages.PkgInfo{Name: "name", Arch: "arch", Version: "version"}
	cache[key] = packageInfo{PkgInfo: info, LastLookup: time.Now().Add(packageInfoCacheTimeout).Add(-1 * time.Hour)}

	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(packageInfoCacheFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	updatePackageInfoCache(ctx, nil, nil)
	if _, ok := packageInfoCacheStore[key]; ok {
		t.Errorf("Cache should not contain expired data, cache: %+v", packageInfoCacheStore)
	}
}

func TestPackageResourceCleanup(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	tmpCacheFile := filepath.Join(t.TempDir(), "test.cache")
	utiltest.OverrideVariable(t, &packageInfoCacheFile, tmpCacheFile)
	packageInfoCacheStore = packageInfoCache{"test-key": packageInfo{}}

	pr := &OSPolicyResource{
		OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
			ResourceType: &agentendpointpb.OSPolicy_Resource_Pkg{Pkg: aptInstalledPR},
		},
	}
	if err := pr.Validate(ctx); err != nil {
		t.Fatalf("Unexpected Validate error: %v", err)
	}

	pr.resource.(*packageResouce).managedPackage.tempDir = tmpDir

	gotErr := pr.Cleanup(ctx)

	utiltest.AssertErrorMatch(t, gotErr, nil)
	utiltest.AssertEquals(t, util.Exists(tmpDir), false)
	utiltest.AssertEquals(t, packageInfoCacheStore, packageInfoCache(nil))
	utiltest.AssertEquals(t, util.Exists(tmpCacheFile), true)
}
