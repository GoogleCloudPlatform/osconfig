//  Copyright 2019 Google Inc. All Rights Reserved.
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
	"errors"
	"os/exec"
	"reflect"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

func TestParseInstalledRPMPackages(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []*PkgInfo
	}{
		{
			name: "Two packages in input",
			data: []byte("" +
				`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}` + "\n" +
				`{"architecture":"noarch","package":"golang-src","source_name":"golang-1.22.3-1.el9.src.rpm","version":"1.22.3-1.el9"}`),
			want: []*PkgInfo{
				{Name: "gcc", Arch: "x86_64", Version: "11.4.1-3.el9", Source: Source{Name: "gcc-11.4.1-3.el9.src.rpm"}},
				{Name: "golang-src", Arch: "all", Version: "1.22.3-1.el9", Source: Source{Name: "golang-1.22.3-1.el9.src.rpm"}},
			},
		},
		{
			name: "No valid pacakges",
			data: []byte("nothing here"),
			want: nil,
		},
		{
			name: "Function doesn't panic on nil input",
			data: nil,
			want: nil,
		},
		{
			name: "Skip invalid packages",
			data: []byte("" +
				`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}` + "\n" +
				"something we dont understand\n bar noarch 1.2.3-4 "),
			want: []*PkgInfo{{Name: "gcc", Arch: "x86_64", Version: "11.4.1-3.el9", Source: Source{Name: "gcc-11.4.1-3.el9.src.rpm"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInstalledRPMPackages(testCtx, tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("installedRPMPackages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstalledRPMPackages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	expectedCmd := utilmocks.EqCmd(exec.Command(rpmquery, rpmqueryInstalledArgs...))

	stdout := []byte(`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}`)
	stderr := []byte("stderr")
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(stdout, stderr, nil).Times(1)
	ret, err := InstalledRPMPackages(testCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := []*PkgInfo{{Name: "gcc", Arch: "x86_64", Version: "11.4.1-3.el9", Source: Source{Name: "gcc-11.4.1-3.el9.src.rpm"}}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("InstalledRPMPackages() = %v, want %v", ret, want)
	}

	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("bad error")).Times(1)
	if _, err := InstalledRPMPackages(testCtx); err == nil {
		t.Errorf("did not get expected error")
	}
}

func TestRPMPkgInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	testPkg := "test.rpm"
	expectedCmd := utilmocks.EqCmd(exec.Command(rpmquery, append(rpmqueryRPMArgs, testPkg)...))

	stdout := []byte(`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}`)
	stderr := []byte("stderr")
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(stdout, stderr, nil).Times(1)
	ret, err := RPMPkgInfo(testCtx, testPkg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	want := &PkgInfo{Name: "gcc", Arch: "x86_64", Version: "11.4.1-3.el9", Source: Source{Name: "gcc-11.4.1-3.el9.src.rpm"}}
	if !reflect.DeepEqual(ret, want) {
		t.Errorf("RPMPkgInfo() = %v, want %v", ret, want)
	}

	// Error output.
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte("stdout"), []byte("stderr"), errors.New("bad error")).Times(1)
	if _, err := RPMPkgInfo(testCtx, testPkg); err == nil {
		t.Errorf("did not get expected error")
	}
	// More than 1 package
	stdout = []byte("" +
		`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}` + "\n" +
		`{"architecture":"noarch","package":"golang-src","source_name":"golang-1.22.3-1.el9.src.rpm","version":"1.22.3-1.el9"}`)
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return(stdout, stderr, nil).Times(1)
	if _, err := RPMPkgInfo(testCtx, testPkg); err == nil {
		t.Errorf("did not get expected error")
	}
	// No package
	mockCommandRunner.EXPECT().Run(testCtx, expectedCmd).Return([]byte(""), []byte("stderr"), nil).Times(1)
	if _, err := RPMPkgInfo(testCtx, testPkg); err == nil {
		t.Errorf("did not get expected error")
	}
}
