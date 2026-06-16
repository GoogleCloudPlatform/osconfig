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

package utilmocks

import (
	"context"
	"reflect"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/golang/mock/gomock"
)

// MockOSInfoProvider is a gomock implementation of osinfo.Provider.
type MockOSInfoProvider struct {
	ctrl     *gomock.Controller
	recorder *MockOSInfoProviderMockRecorder
}

// MockOSInfoProviderMockRecorder is the mock recorder for MockOSInfoProvider.
type MockOSInfoProviderMockRecorder struct {
	mock *MockOSInfoProvider
}

// NewMockOSInfoProvider creates a new mock instance.
func NewMockOSInfoProvider(ctrl *gomock.Controller) *MockOSInfoProvider {
	mock := &MockOSInfoProvider{ctrl: ctrl}
	mock.recorder = &MockOSInfoProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOSInfoProvider) EXPECT() *MockOSInfoProviderMockRecorder {
	return m.recorder
}

// GetOSInfo responds to a GetOSInfo call.
func (m *MockOSInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOSInfo", ctx)
	ret0, _ := ret[0].(osinfo.OSInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOSInfo indicates an expected call of GetOSInfo.
func (mr *MockOSInfoProviderMockRecorder) GetOSInfo(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOSInfo", reflect.TypeOf((*MockOSInfoProvider)(nil).GetOSInfo), ctx)
}
