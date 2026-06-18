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
	"net/http"
	"net/http/httptest"
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

func TestPackageResourceEnforceState(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	var tests = []struct {
		name         string
		prpb         *agentendpointpb.OSPolicy_Resource_PackageResource
		cachePointer *packageCache
		expectedCmds []*exec.Cmd
	}{
		{
			"AptInstalled",
			aptInstalledPR,
			aptInstalled,
			func() []*exec.Cmd {
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
		},
		{
			"AptRemoved",
			aptRemovedPR,
			aptInstalled,
			func() []*exec.Cmd {
				cmd1 := exec.Command("/usr/bin/apt-get", "remove", "-y", "foo")
				cmd1.Env = append(os.Environ(),
					"DEBIAN_FRONTEND=noninteractive",
				)
				return []*exec.Cmd{cmd1}
			}(),
		},
		{
			"GooGetInstalled",
			googetInstalledPR,
			gooInstalled,
			[]*exec.Cmd{exec.Command("googet.exe", "-noconfirm", "install", "foo")},
		},
		{
			"GooGetRemoved",
			googetRemovedPR,
			gooInstalled,
			[]*exec.Cmd{exec.Command("googet.exe", "-noconfirm", "remove", "foo")},
		},
		{
			"YumInstalled",
			yumInstalledPR,
			yumInstalled,
			[]*exec.Cmd{exec.Command("/usr/bin/yum", "install", "--assumeyes", "foo")},
		},
		{
			"YumRemoved",
			yumRemovedPR,
			yumInstalled,
			[]*exec.Cmd{exec.Command("/usr/bin/yum", "remove", "--assumeyes", "foo")},
		},
		{
			"ZypperInstalled",
			zypperInstalledPR,
			zypperInstalled,
			[]*exec.Cmd{exec.Command("/usr/bin/zypper", "--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses", "foo")},
		},
		{
			"ZypperRemoved",
			zypperRemovedPR,
			zypperInstalled,
			[]*exec.Cmd{exec.Command("/usr/bin/zypper", "--non-interactive", "remove", "foo")},
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

func TestPackageResourceValidateErrors(t *testing.T) {
	ctx := t.Context()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	tests := []struct {
		name    string
		setup   func(t *testing.T)
		prpb    *agentendpointpb.OSPolicy_Resource_PackageResource
		wantErr error
	}{
		{
			name:    "Apt does not exist, expect apt-get missing error",
			setup:   func(t *testing.T) { utiltest.OverrideVariable(t, &packages.AptExists, false) },
			prpb:    aptInstalledPR,
			wantErr: errors.New("cannot manage Apt package \"foo\" because apt-get does not exist on the system"),
		},
		{
			name:  "Deb does not exist, expect dpkg missing error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.DpkgExists, false) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb_{
					Deb: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{},
				},
			},
			wantErr: errors.New("cannot manage Deb package because dpkg does not exist on the system"),
		},
		{
			name:  "Deb not installed state, expect state not applicable error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.DpkgExists, true) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb_{
					Deb: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{},
				},
			},
			wantErr: errors.New("desired state of \"REMOVED\" not applicable for deb package"),
		},
		{
			name:    "GooGet does not exist, expect googet missing error",
			setup:   func(t *testing.T) { utiltest.OverrideVariable(t, &packages.GooGetExists, false) },
			prpb:    googetInstalledPR,
			wantErr: errors.New("cannot manage GooGet package \"foo\" because googet does not exist on the system"),
		},
		{
			name:  "MSI does not exist, expect msiexec missing error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.MSIExists, false) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Msi{
					Msi: &agentendpointpb.OSPolicy_Resource_PackageResource_MSI{},
				},
			},
			wantErr: errors.New("cannot manage MSI package because msiexec does not exist on the system"),
		},
		{
			name:  "MSI not installed state, expect state not applicable error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.MSIExists, true) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Msi{
					Msi: &agentendpointpb.OSPolicy_Resource_PackageResource_MSI{},
				},
			},
			wantErr: errors.New("desired state of \"REMOVED\" not applicable for MSI package"),
		},
		{
			name:    "Yum does not exist, expect yum missing error",
			setup:   func(t *testing.T) { utiltest.OverrideVariable(t, &packages.YumExists, false) },
			prpb:    yumInstalledPR,
			wantErr: errors.New("cannot manage Yum package \"foo\" because yum does not exist on the system"),
		},
		{
			name:    "Zypper does not exist, expect zypper missing error",
			setup:   func(t *testing.T) { utiltest.OverrideVariable(t, &packages.ZypperExists, false) },
			prpb:    zypperInstalledPR,
			wantErr: errors.New("cannot manage Zypper package \"foo\" because zypper does not exist on the system"),
		},
		{
			name:  "RPM does not exist, expect rpm missing error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.RPMExists, false) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Rpm{
					Rpm: &agentendpointpb.OSPolicy_Resource_PackageResource_RPM{},
				},
			},
			wantErr: errors.New("cannot manage RPM package because rpm does not exist on the system"),
		},
		{
			name:  "RPM not installed state, expect state not applicable error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.RPMExists, true) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Rpm{
					Rpm: &agentendpointpb.OSPolicy_Resource_PackageResource_RPM{},
				},
			},
			wantErr: errors.New("desired state of \"REMOVED\" not applicable for rpm package"),
		},
		{
			name:    "Unknown package manager, expect unknown package manager error",
			setup:   func(t *testing.T) {},
			prpb:    &agentendpointpb.OSPolicy_Resource_PackageResource{},
			wantErr: errors.New("SystemPackage field not set or references unknown package manager: <nil>"),
		},
		{
			name:  "Local path does not exist, expect file not found error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.RPMExists, true) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Rpm{
					Rpm: &agentendpointpb.OSPolicy_Resource_PackageResource_RPM{
						Source: &agentendpointpb.OSPolicy_Resource_File{
							Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: "doesnotexist.rpm"},
						},
					},
				},
			},
			wantErr: errors.New("\"doesnotexist.rpm\" does not exist"),
		},
		{
			name:  "Deb remote file download fails, expect 404 error",
			setup: func(t *testing.T) { utiltest.OverrideVariable(t, &packages.DpkgExists, true) },
			prpb: &agentendpointpb.OSPolicy_Resource_PackageResource{
				DesiredState: agentendpointpb.OSPolicy_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb_{
					Deb: &agentendpointpb.OSPolicy_Resource_PackageResource_Deb{
						Source: &agentendpointpb.OSPolicy_Resource_File{
							Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
								Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{Uri: server.URL},
							},
						},
					},
				},
			},
			wantErr: errors.New("got http status 404 when attempting to download artifact"),
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
			gotErr := pr.Validate(ctx)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func setupPackageResourceCleanupTest(t *testing.T, ctx context.Context) (*OSPolicyResource, string, string) {
	t.Helper()
	dir := t.TempDir()

	tmpCacheFile := filepath.Join(t.TempDir(), "test.cache")
	utiltest.OverrideVariable(t, &packageInfoCacheFile, tmpCacheFile)
	packageInfoCacheStore = packageInfoCache{"test-key": packageInfo{}}

	pr := &OSPolicyResource{
		OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
			ResourceType: &agentendpointpb.OSPolicy_Resource_Pkg{Pkg: aptInstalledPR},
		},
	}
	if err := pr.Validate(ctx); err != nil {
		t.Fatal(err)
	}

	pr.resource.(*packageResouce).managedPackage.tempDir = dir
	return pr, dir, tmpCacheFile
}

func TestPackageResourceCleanup(t *testing.T) {
	ctx := t.Context()
	pr, dir, tmpCacheFile := setupPackageResourceCleanupTest(t, ctx)

	gotErr := pr.Cleanup(ctx)

	utiltest.AssertErrorMatch(t, gotErr, nil)
	utiltest.AssertEquals(t, util.Exists(dir), false)
	utiltest.AssertEquals(t, packageInfoCacheStore == nil, true)
	utiltest.AssertEquals(t, util.Exists(tmpCacheFile), true)
}
