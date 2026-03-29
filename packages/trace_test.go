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
)

// mockInstalledPackagesProvider is a mock implementation of InstalledPackagesProvider.
type mockInstalledPackagesProvider struct {
	pkgs Packages
	err  error
}

// GetInstalledPackages returns the predefined packages and error.
func (m mockInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	return m.pkgs, m.err
}

// mockOSInfoProvider is a mock implementation of osinfo.Provider.
type mockOSInfoProvider struct {
	info osinfo.OSInfo
	err  error
}

// GetOSInfo returns the predefined OSInfo and error.
func (m mockOSInfoProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return m.info, m.err
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
			name:      "Success",
			tracedErr: nil,
			osInfoErr: nil,
			wantPkgs:  testPkgs,
			wantErr:   nil,
		},
		{
			name:      "TracedProviderError",
			tracedErr: errors.New("traced error"),
			osInfoErr: nil,
			wantPkgs:  Packages{},
			wantErr:   errors.New("traced error"),
		},
		{
			name:      "OSInfoError",
			tracedErr: nil,
			osInfoErr: errors.New("osinfo error"),
			wantPkgs:  testPkgs,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := mockInstalledPackagesProvider{pkgs: tt.wantPkgs, err: tt.tracedErr}
			op := mockOSInfoProvider{info: testInfo, err: tt.osInfoErr}
			provider := TracingInstalledPackagesProvider(tp, op)

			gotPkgs, err := provider.GetInstalledPackages(ctx)

			if !reflect.DeepEqual(gotPkgs, tt.wantPkgs) {
				t.Errorf("GetInstalledPackages() gotPkgs = %v, want %v", gotPkgs, tt.wantPkgs)
			}
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
