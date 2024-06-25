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
	tests := []struct {
		name string

		expectedCommandsChain []expectedCommand
		expectedResults       []*PkgInfo
		expectedError         error
	}{
		{
			name: "success path",
			expectedCommandsChain: []expectedCommand{
				{
					cmd: exec.Command(rpmquery, rpmqueryInstalledArgs...),
					stdout: []byte("" +
						`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}` + "\n" +
						`{"architecture":"noarch","package":"golang-src","source_name":"golang-1.22.3-1.el9.src.rpm","version":"1.22.3-1.el9"}`),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{
				{Name: "gcc", Arch: "x86_64", Version: "11.4.1-3.el9", Source: Source{Name: "gcc-11.4.1-3.el9.src.rpm"}},
				{Name: "golang-src", Arch: "all", Version: "1.22.3-1.el9", Source: Source{Name: "golang-1.22.3-1.el9.src.rpm"}},
			},
			expectedError: nil,
		},
		{
			name: "rpmquery command failed",
			expectedCommandsChain: []expectedCommand{{
				cmd:    exec.Command(rpmquery, rpmqueryInstalledArgs...),
				stdout: []byte("stdout"),
				stderr: []byte("stderr"),
				err:    errors.New("unexpected error"),
			},
			},
			expectedResults: nil,
			expectedError:   errors.New("error running /usr/bin/rpmquery with args [\"--queryformat\" \"\\\\{\\\"architecture\\\":\\\"%{ARCH}\\\",\\\"package\\\":\\\"%{NAME}\\\",\\\"source_name\\\":\\\"%{SOURCERPM}\\\",\\\"version\\\":\\\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\\\"\\\\}\\n\" \"-a\"]: unexpected error, stdout: \"stdout\", stderr: \"stderr\""),
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			results, err := InstalledRPMPackages(testCtx)
			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("InstalledRPMPackages: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}

			if !reflect.DeepEqual(results, tt.expectedResults) {
				t.Errorf("InstalledRPMPackages: unexpected result, expect %v, got %v", pkgs, tt.expectedResults)
			}
		})
	}
}

func TestRPMPkgInfo(t *testing.T) {
	tests := []struct {
		name string

		path string

		expectedCommandsChain []expectedCommand

		expectedResult *PkgInfo
		expectedError  error
	}{
		{
			name: "single package",
			path: "/tmp/gcc.rpm",

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(rpmquery, append(rpmqueryRPMArgs, "/tmp/gcc.rpm")...),
					stdout: []byte(`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}`),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedResult: &PkgInfo{
				Name:    "gcc",
				Arch:    "x86_64",
				Version: "11.4.1-3.el9",
				Source:  Source{Name: "gcc-11.4.1-3.el9.src.rpm"},
			},
			expectedError: nil,
		},
		{
			name: "multiple packages",
			path: "/tmp/gcc.rpm",

			expectedCommandsChain: []expectedCommand{
				{
					cmd: exec.Command(rpmquery, append(rpmqueryRPMArgs, "/tmp/gcc.rpm")...),
					stdout: []byte("" +
						`{"architecture":"x86_64","package":"gcc","source_name":"gcc-11.4.1-3.el9.src.rpm","version":"11.4.1-3.el9"}` + "\n" +
						`{"architecture":"noarch","package":"golang-src","source_name":"golang-1.22.3-1.el9.src.rpm","version":"1.22.3-1.el9"}`),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},

			expectedResult: nil,
			expectedError:  errors.New("unexpected number of parsed rpm packages 2: \"{\\\"architecture\\\":\\\"x86_64\\\",\\\"package\\\":\\\"gcc\\\",\\\"source_name\\\":\\\"gcc-11.4.1-3.el9.src.rpm\\\",\\\"version\\\":\\\"11.4.1-3.el9\\\"}\\n{\\\"architecture\\\":\\\"noarch\\\",\\\"package\\\":\\\"golang-src\\\",\\\"source_name\\\":\\\"golang-1.22.3-1.el9.src.rpm\\\",\\\"version\\\":\\\"1.22.3-1.el9\\\"}\""),
		},
		{
			name: "rpmquery returns no package",
			path: "/tmp/gcc.rpm",

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(rpmquery, append(rpmqueryRPMArgs, "/tmp/gcc.rpm")...),
					stdout: []byte("no packages"),
					stderr: []byte("stderr"),
					err:    nil,
				},
			},
			expectedResult: nil,
			expectedError:  errors.New("unexpected number of parsed rpm packages 0: \"no packages\""),
		},
		{
			name: "rpmquery failed with error",
			path: "/tmp/gcc.rpm",

			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command(rpmquery, append(rpmqueryRPMArgs, "/tmp/gcc.rpm")...),
					stdout: []byte("stdout"),
					stderr: []byte("stderr"),
					err:    errors.New("unexpected error"),
				},
			},
			expectedResult: nil,
			expectedError:  errors.New("error running /usr/bin/rpmquery with args [\"--queryformat\" \"\\\\{\\\"architecture\\\":\\\"%{ARCH}\\\",\\\"package\\\":\\\"%{NAME}\\\",\\\"source_name\\\":\\\"%{SOURCERPM}\\\",\\\"version\\\":\\\"%|EPOCH?{%{EPOCH}:}:{}|%{VERSION}-%{RELEASE}\\\"\\\\}\\n\" \"-p\" \"/tmp/gcc.rpm\"]: unexpected error, stdout: \"stdout\", stderr: \"stderr\""),
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			result, err := RPMPkgInfo(testCtx, tt.path)
			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("RPMPkgInfo: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}

			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("RPMPkgInfo: unexpected result, expect %v, got %v", result, tt.expectedResult)
			}
		})
	}

}

func TestRPMInstall(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner

	path := "/tmp/test.dpkg"
	rpmInstallCmd := exec.CommandContext(testCtx, rpm, append(rpmInstallArgs, path)...)

	//rpm install fail
	wantErr := errors.New("unexpected error")
	mockCommandRunner.EXPECT().Run(testCtx, utilmocks.EqCmd(rpmInstallCmd)).Return([]byte("stdout"), []byte("stderr"), wantErr).Times(1)
	if err := RPMInstall(testCtx, path); reflect.DeepEqual(err, wantErr) {
		t.Errorf("RPMInstall: expected error %q, but got %q", formatError(wantErr), formatError(err))
	}

	//rpm install succeeded
	mockCommandRunner.EXPECT().Run(testCtx, utilmocks.EqCmd(rpmInstallCmd)).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
	if err := RPMInstall(testCtx, path); err != nil {
		t.Errorf("RPMInstall: got unexpected error %q", err)
	}
}
