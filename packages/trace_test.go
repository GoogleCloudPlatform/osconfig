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
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
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

func runGetInstalledPackages(ctx context.Context, tp *mockInstalledPackagesProvider, op *utilmocks.MockOSInfoProvider, testInfo osinfo.OSInfo, wantPkgs Packages, tracedErr, osInfoErr error) (Packages, error) {
	call1 := tp.EXPECT().GetInstalledPackages(gomock.Any()).DoAndReturn(func(ctx context.Context) (Packages, error) {
		time.Sleep(110 * time.Millisecond)
		return wantPkgs, tracedErr
	}).Times(1)

	op.EXPECT().GetOSInfo(gomock.Any()).After(call1).Return(testInfo, osInfoErr).Times(1)

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
	op := utilmocks.NewMockOSInfoProvider(mockCtrl)

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
