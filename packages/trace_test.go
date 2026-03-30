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

// TestTracingInstalledPackagesProvider verifies that the tracing decorator
// correctly handles results and errors from the underlying providers.
func TestTracingInstalledPackagesProvider(t *testing.T) {
	ctx := context.Background()
	testPkgs := Packages{Yum: []*PkgInfo{{Name: "pkg1"}}}
	testInfo := osinfo.OSInfo{Hostname: "test-host"}

	tests := []struct {
		name      string
		tracedErr error
		osInfoErr error
		wantPkgs  Packages
		wantErr   error
	}{
		{
			name:      "success case",
			tracedErr: nil,
			osInfoErr: nil,
			wantPkgs:  testPkgs,
			wantErr:   nil,
		},
		{
			name:      "traced provider returns error",
			tracedErr: errors.New("traced error"),
			osInfoErr: nil,
			wantPkgs:  Packages{},
			wantErr:   errors.New("traced error"),
		},
		{
			name:      "osinfo provider returns error",
			tracedErr: nil,
			osInfoErr: errors.New("osinfo error"),
			wantPkgs:  testPkgs,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tp := newMockInstalledPackagesProvider(mockCtrl)
			op := newMockOSInfoProvider(mockCtrl)

			tp.EXPECT().GetInstalledPackages(gomock.Any()).Return(tt.wantPkgs, tt.tracedErr)
			op.EXPECT().GetOSInfo(gomock.Any()).Return(testInfo, tt.osInfoErr)

			provider := TracingInstalledPackagesProvider(tp, op)

			gotPkgs, err := provider.GetInstalledPackages(ctx)

			if !reflect.DeepEqual(gotPkgs, tt.wantPkgs) {
				t.Errorf("GetInstalledPackages() gotPkgs = %v, want %v", gotPkgs, tt.wantPkgs)
			}
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
