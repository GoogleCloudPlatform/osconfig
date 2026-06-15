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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
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
	ctx := context.Background()
	var tests = []struct {
		name               string
		installedCache     map[string]struct{}
		cachePointer       *packageCache
		prpb               *agentendpointpb.OSPolicy_Resource_PackageResource
		wantInDesiredState bool
	}{
		// We only need to test the full set once as all the logic is shared.
		{
			"AptInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			aptInstalled,
			aptInstalledPR,
			true,
		},
		{
			"AptInstalledNeedsRemoved",
			map[string]struct{}{"foo": {}},
			aptInstalled,
			aptRemovedPR,
			false,
		},
		{
			"AptRemovedNeedsInstalled",
			map[string]struct{}{},
			aptInstalled,
			aptInstalledPR,
			false,
		},
		{
			"AptRemovedNeedsRemoved",
			map[string]struct{}{},
			aptInstalled,
			aptRemovedPR,
			true,
		},

		// For the rest of the package types we only need to test one scenario.
		{
			"GooGetInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			gooInstalled,
			googetInstalledPR,
			true,
		},
		{
			"YUMInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			yumInstalled,
			yumInstalledPR,
			true,
		},
		{
			"ZypperInstalledNeedsInstalled",
			map[string]struct{}{"foo": {}},
			zypperInstalled,
			zypperInstalledPR,
			true,
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

func createTestDebPackageResource(localPath string, pullDeps bool) *agentendpointpb.OSPolicy_Resource_PackageResource {
	debPR := &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{
		PullDeps: pullDeps,
		Source:   &agentendpointpb.OSPolicy_Resource_File{
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
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "foo.deb")
	if err := ioutil.WriteFile(tmpFile, nil, 0644); err != nil {
		t.Fatal(err)
	}

	tmpRpmFile := filepath.Join(tmpDir, "foo.rpm")
	if err := ioutil.WriteFile(tmpRpmFile, nil, 0644); err != nil {
		t.Fatal(err)
	}

	dpkgCmd := exec.Command("/usr/bin/dpkg-deb", "-I", tmpFile)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(dpkgCmd)).Return([]byte("Package: foo\nVersion: 1:1dummy-g1\nArchitecture: amd64"), nil, nil).AnyTimes()

	rpmqueryCmd := exec.Command("/usr/bin/rpmquery", "--queryformat", "\\{\"architecture\":\"%{ARCH}\",\"package\":\"%{NAME}\",\"source_name\":\"%{SOURCERPM}\",\"version\":\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\"\\}\n", "-p", tmpRpmFile)
	mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(rpmqueryCmd)).Return([]byte("{\"architecture\":\"x86_64\",\"package\":\"foo\",\"source_name\":\"foo.src.rpm\",\"version\":\"1.0\"}"), nil, nil).AnyTimes()

	var tests = []struct {
		name         string
		prpb         *agentendpointpb.OSPolicy_Resource_PackageResource
		cachePointer *packageCache
		expectedCmds []*exec.Cmd
		setup        func(t *testing.T)
	}{
		{
			name:         "AptInstalled",
			prpb:         aptInstalledPR,
			cachePointer: aptInstalled,
			expectedCmds: func() []*exec.Cmd {
				cmd1 := exec.Command("/usr/bin/apt-get", "update")
				cmd1.Env = append(os.Environ(),
					"DEBIAN_FRONTEND=noninteractive",
				)
				cmd2 := exec.Command("/usr/bin/apt-get", "install", "-y", "foo")
				cmd2.Env = append(os.Environ(),
					"DEBIAN_FRONTEND=noninteractive",
				)
				return []*exec.Cmd{cmd1, cmd2}
			}(),
			setup: func(t *testing.T) {},
		},
		{
			name:         "AptRemoved",
			prpb:         aptRemovedPR,
			cachePointer: aptInstalled,
			expectedCmds: func() []*exec.Cmd {
				cmd1 := exec.Command("/usr/bin/apt-get", "remove", "-y", "foo")
				cmd1.Env = append(os.Environ(),
					"DEBIAN_FRONTEND=noninteractive",
				)
				return []*exec.Cmd{cmd1}
			}(),
			setup: func(t *testing.T) {},
		},
		{
			name:         "GooGetInstalled",
			prpb:         googetInstalledPR,
			cachePointer: gooInstalled,
			expectedCmds: []*exec.Cmd{exec.Command("googet.exe", "-noconfirm", "install", "foo")},
			setup:        func(t *testing.T) {},
		},
		{
			name:         "GooGetRemoved",
			prpb:         googetRemovedPR,
			cachePointer: gooInstalled,
			expectedCmds: []*exec.Cmd{exec.Command("googet.exe", "-noconfirm", "remove", "foo")},
			setup:        func(t *testing.T) {},
		},
		{
			name:         "YumInstalled",
			prpb:         yumInstalledPR,
			cachePointer: yumInstalled,
			expectedCmds: []*exec.Cmd{exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo")},
			setup:        func(t *testing.T) {},
		},
		{
			name:         "YumRemoved",
			prpb:         yumRemovedPR,
			cachePointer: yumInstalled,
			expectedCmds: []*exec.Cmd{exec.Command("/usr/bin/yum", "remove", "--assumeyes", "foo")},
			setup:        func(t *testing.T) {},
		},
		{
			name:         "ZypperInstalled",
			prpb:         zypperInstalledPR,
			cachePointer: zypperInstalled,
			expectedCmds: []*exec.Cmd{exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "foo")},
			setup:        func(t *testing.T) {},
		},
		{
			name:         "ZypperRemoved",
			prpb:         zypperRemovedPR,
			cachePointer: zypperInstalled,
			expectedCmds: []*exec.Cmd{exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "foo")},
			setup:        func(t *testing.T) {},
		},
		{
			name:         "DebInstalled_PullDepsFalse",
			prpb:         createTestDebPackageResource(tmpFile, false),
			cachePointer: debInstalled,
			expectedCmds: []*exec.Cmd{
				exec.Command("/usr/bin/dpkg", "--install", tmpFile),
			},
			setup: func(t *testing.T) {},
		},
		{
			name:         "DebInstalled_PullDepsTrue",
			prpb:         createTestDebPackageResource(tmpFile, true),
			cachePointer: debInstalled,
			expectedCmds: func() []*exec.Cmd {
				cmd2 := exec.Command("/usr/bin/apt-get", "install", "-y", tmpFile)
				cmd2.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
				return []*exec.Cmd{cmd2}
			}(),
			setup: func(t *testing.T) {},
		},
		{
			name:         "RPMInstalled_PullDepsFalse",
			prpb:         createRPMPackageResource(tmpRpmFile, false),
			cachePointer: rpmInstalled,
			expectedCmds: []*exec.Cmd{
				exec.Command("/bin/rpm", "--upgrade", "--replacepkgs", "-v", tmpRpmFile),
			},
			setup: func(t *testing.T) {},
		},
		{
			name:         "RPMInstalled_PullDepsTrue_Yum",
			prpb:         createRPMPackageResource(tmpRpmFile, true),
			cachePointer: rpmInstalled,
			expectedCmds: []*exec.Cmd{
				exec.Command("/usr/bin/yum", "install", "--assumeyes", tmpRpmFile),
			},
			setup: func(t *testing.T) {
				utiltest.OverrideVariable(t, &packages.YumExists, true)
			},
		},
		{
			name:         "RPMInstalled_PullDepsTrue_Zypper",
			prpb:         createRPMPackageResource(tmpRpmFile, true),
			cachePointer: rpmInstalled,
			expectedCmds: []*exec.Cmd{
				exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", tmpRpmFile),
			},
			setup: func(t *testing.T) {
				utiltest.OverrideVariable(t, &packages.YumExists, false)
				utiltest.OverrideVariable(t, &packages.ZypperExists, true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Pkg{Pkg: tt.prpb},
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

			tt.cachePointer.cache = map[string]struct{}{"foo": {}}

			for _, expectedCmd := range tt.expectedCmds {
				mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(expectedCmd))
			}

			if err := pr.EnforceState(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.cachePointer.cache != nil {
				t.Errorf("Enforce function did not set package cache to nil")
			}
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
