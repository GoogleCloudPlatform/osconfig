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

package packages

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

// mockInstalledPackagesProvider is a gomock implementation of InstalledPackagesProvider.
type mockInstalledPackagesProvider struct {
	ctrl     *gomock.Controller
	recorder *mockInstalledPackagesProviderMockRecorder
}

type mockInstalledPackagesProviderMockRecorder struct {
	mock *mockInstalledPackagesProvider
}

func newMockInstalledPackagesProvider(ctrl *gomock.Controller) *mockInstalledPackagesProvider {
	mock := &mockInstalledPackagesProvider{ctrl: ctrl}
	mock.recorder = &mockInstalledPackagesProviderMockRecorder{mock}
	return mock
}

func (m *mockInstalledPackagesProvider) EXPECT() *mockInstalledPackagesProviderMockRecorder {
	return m.recorder
}

func (m *mockInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetInstalledPackages", ctx)
	ret0, _ := ret[0].(Packages)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *mockInstalledPackagesProviderMockRecorder) GetInstalledPackages(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetInstalledPackages", reflect.TypeOf((*mockInstalledPackagesProvider)(nil).GetInstalledPackages), ctx)
}

// mockOSInfoProvider is a gomock implementation of osinfo.Provider.
type mockOSInfoProvider struct {
	ctrl     *gomock.Controller
	recorder *mockOSInfoProviderMockRecorder
}

type mockOSInfoProviderMockRecorder struct {
	mock *mockOSInfoProvider
}

func newMockOSInfoProvider(ctrl *gomock.Controller) *mockOSInfoProvider {
	mock := &mockOSInfoProvider{ctrl: ctrl}
	mock.recorder = &mockOSInfoProviderMockRecorder{mock}
	return mock
}

func (m *mockOSInfoProvider) EXPECT() *mockOSInfoProviderMockRecorder {
	return m.recorder
}

func (m *mockOSInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOSInfo", ctx)
	ret0, _ := ret[0].(osinfo.OSInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *mockOSInfoProviderMockRecorder) GetOSInfo(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOSInfo", reflect.TypeOf((*mockOSInfoProvider)(nil).GetOSInfo), ctx)
}

func runGetInstalledPackages(ctx context.Context, tp *mockInstalledPackagesProvider, op *mockOSInfoProvider, testInfo osinfo.OSInfo, wantPkgs Packages, tracedErr, osInfoErr error) (Packages, error) {
	call := op.EXPECT().GetOSInfo(gomock.Any()).Return(testInfo, osInfoErr).Times(1)
	tp.EXPECT().GetInstalledPackages(gomock.Any()).After(call).DoAndReturn(func(ctx context.Context) (Packages, error) {
		// Wait at least 110ms to ensure TraceMemory (100ms interval) samples at least once.
		time.Sleep(110 * time.Millisecond)
		return wantPkgs, tracedErr
	}).Times(1)

	return TracingInstalledPackagesProvider(tp, op).GetInstalledPackages(ctx)
}

// TestTracingInstalledPackagesProvider verifies that the tracing decorator
// correctly handles results and errors from the underlying providers.
func TestTracingInstalledPackagesProvider(t *testing.T) {
	testPkgs := Packages{Yum: []*PkgInfo{{Name: "pkg1"}}}
	testInfo := osinfo.OSInfo{Hostname: "test-host"}
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	tp := newMockInstalledPackagesProvider(mockCtrl)
	op := newMockOSInfoProvider(mockCtrl)

	tests := []struct {
		name      string
		tracedErr error
		osInfoErr error
		wantPkgs  Packages
		wantErr   error
	}{
		{
			name:      "no errors from providers, want nil error",
			tracedErr: nil,
			osInfoErr: nil,
			wantPkgs:  testPkgs,
			wantErr:   nil,
		},
		{
			name:      "traced provider error, want traced provider error",
			tracedErr: errors.New("traced provider error"),
			osInfoErr: nil,
			wantPkgs:  Packages{},
			wantErr:   errors.New("traced provider error"),
		},
		{
			name:      "osinfo provider error, want nil error",
			tracedErr: nil,
			osInfoErr: errors.New("osinfo error"),
			wantPkgs:  testPkgs,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPkgs, gotErr := runGetInstalledPackages(context.Background(), tp, op, testInfo, tt.wantPkgs, tt.tracedErr, tt.osInfoErr)

			utiltest.AssertEquals(t, gotPkgs, tt.wantPkgs)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func runGetInstalledPackagesWithScalibr(t *testing.T, extractors []string, scanRoots, dirsToSkip []string, op osinfo.Provider) (Packages, error) {
	t.Helper()
	provider := &scalibrInstalledPackagesProvider{
		extractors:     extractors,
		osinfoProvider: op,
		scanRootPaths:  scanRoots,
		dirsToSkip:     dirsToSkip,
	}
	traced := TracingInstalledPackagesProvider(provider, op)
	return traced.GetInstalledPackages(context.Background())
}

// TestTracingInstalledPackagesProvider_Scalibr verifies that TracingInstalledPackagesProvider
// works correctly when decorating a scalibrInstalledPackagesProvider.
func TestTracingInstalledPackagesProvider_Scalibr(t *testing.T) {
	utiltest.OverrideVariable(t, &ZypperExists, false)
	virtualRoot := arrangeVirtualRoot(t, "./testdata/debian.dpkg-status", "/var/lib/dpkg/status")

	wantPkgs := Packages{Deb: []*PkgInfo{
		{Name: "7zip", Version: "24.09+dfsg-4", Arch: "x86_64", Source: Source{Name: "7zip", Version: "24.09+dfsg-4"}, Type: "deb", Purl: "pkg:deb/linux/7zip@24.09%2Bdfsg-4?arch=amd64"},
		{Name: "llvm-16", Version: "1:16.0.6-27+build3", Arch: "x86_64", Source: Source{Name: "llvm-toolchain-16", Version: "1:16.0.6-27+build3"}, Type: "deb", Purl: "pkg:deb/linux/llvm-16@1%3A16.0.6-27%2Bbuild3?arch=amd64&source=llvm-toolchain-16"},
	}}

	tests := []struct {
		name       string
		extractors []string
		scanRoots  []string
		dirsToSkip []string
		op         osinfo.Provider
		wantPkgs   Packages
		wantErr    error
	}{
		{
			name:       "valid dpkg scan with scalibr, want packages and nil error",
			extractors: []string{"os/dpkg"},
			scanRoots:  []string{virtualRoot},
			dirsToSkip: []string{},
			op:         stubProvider{},
			wantPkgs:   wantPkgs,
			wantErr:    nil,
		},
		{
			name:       "invalid extractor with scalibr, want unknown plugin error",
			extractors: []string{"invalid/extractor"},
			scanRoots:  []string{virtualRoot},
			dirsToSkip: []string{},
			op:         stubProvider{},
			wantPkgs:   Packages{},
			wantErr:    errors.New("unknown plugin \"invalid/extractor\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPkgs, gotErr := runGetInstalledPackagesWithScalibr(t, tt.extractors, tt.scanRoots, tt.dirsToSkip, tt.op)

			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotPkgs, tt.wantPkgs)
		})
	}
}
