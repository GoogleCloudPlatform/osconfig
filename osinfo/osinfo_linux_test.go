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

package osinfo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/unix"
)

var debianReleaseFileContent = `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

func TestLinuxOsInfoProvider(t *testing.T) {
	mockNameVersionProvidor := func() (string, string, string) {
		return "testShort", "testLong", "testVersion"
	}

	linuxOsinfoProvider := LinuxOsInfoProvider{
		nameAndVersionProvider: mockNameVersionProvidor,
		uts: unix.Utsname{
			Nodename: toUtsField("testhost"),
			Machine:  toUtsField("amd64"),
			Release:  toUtsField("6.1.0-29-cloud-amd64"),
			Version:  toUtsField("#1 SMP PREEMPT_DYNAMIC Debian 6.1.123-1 (2025-01-02)"),
		},
	}

	ctx := context.Background()
	osInfo, err := linuxOsinfoProvider.Get(ctx)
	if err != nil {
		t.Errorf("Unexpected error, err: %v", err)
	}

	expectedOsInfo := OSInfo{
		Hostname:      "testhost",
		LongName:      "testLong",
		ShortName:     "testShort",
		Version:       "testVersion",
		KernelVersion: "#1 SMP PREEMPT_DYNAMIC Debian 6.1.123-1 (2025-01-02)",
		KernelRelease: "6.1.0-29-cloud-amd64",
		Architecture:  "x86_64",
	}

	if diff := cmp.Diff(expectedOsInfo, osInfo); diff != "" {
		t.Errorf("Unexpected OsInfo (-want,+got):\n%s", diff)

	}
}

func Test_parseOsRelease(t *testing.T) {
	tests := []struct {
		name string

		input string

		expectedShortName string
		expectedLongName  string
		expectedVersion   string
	}{
		{
			name: "Empty content",

			input:             ``,
			expectedShortName: "linux",
			expectedLongName:  "",
			expectedVersion:   "",
		},
		{
			name: "Empty key",

			input:             `=`,
			expectedShortName: "linux",
			expectedLongName:  "",
			expectedVersion:   "",
		},
		{
			name:              "Debian 10, normal case",
			input:             debianReleaseFileContent,
			expectedShortName: "debian",
			expectedLongName:  "Debian buster",
			expectedVersion:   "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortName, longName, version := parseOsRelease(tt.input)
			if tt.expectedShortName != shortName {
				t.Errorf("Unexpected shortName, expected %q, got %q", tt.expectedShortName, shortName)
			}

			if tt.expectedLongName != longName {
				t.Errorf("Unexpected longName, expected %q, got %q", tt.expectedLongName, longName)
			}

			if tt.expectedVersion != version {
				t.Errorf("Unexpected version, expected %q, got %q", tt.expectedVersion, version)
			}
		})
	}

}

func Test_parseEnterpriseRelease(t *testing.T) {
	tests := []struct {
		name string

		input string

		expectedShortName string
		expectedLongName  string
		expectedVersion   string
	}{
		{
			name: "Empty content",

			input:             ``,
			expectedShortName: "linux",
			expectedLongName:  "",
			expectedVersion:   "",
		},
		{
			name: "Red Hat Enterprise, normal case",

			input:             `Red Hat Enterprise Linux release 8.0 (Ootpa)`,
			expectedShortName: "rhel",
			expectedLongName:  "Red Hat Enterprise Linux 8.0 (Ootpa)",
			expectedVersion:   "8.0",
		},
		{
			name: "CentOS Linux, normal case",

			input:             `CentOS Linux release 7.6.1810 (Core)`,
			expectedShortName: "centos",
			expectedLongName:  "CentOS Linux 7.6.1810 (Core)",
			expectedVersion:   "7.6.1810",
		},
		{
			name: "",

			input:             `Oracle Linux Server release 9.5`,
			expectedShortName: "ol",
			expectedLongName:  "Oracle Linux Server 9.5",
			expectedVersion:   "9.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortName, longName, version := parseEnterpriseRelease(tt.input)
			if tt.expectedShortName != shortName {
				t.Errorf("Unexpected shortName, expected %q, got %q", tt.expectedShortName, shortName)
			}

			if tt.expectedLongName != longName {
				t.Errorf("Unexpected longName, expected %q, got %q", tt.expectedLongName, longName)
			}

			if tt.expectedVersion != version {
				t.Errorf("Unexpected version, expected %q, got %q", tt.expectedVersion, version)
			}
		})
	}
}

func Test_osNameAndVersionProvider(t *testing.T) {
	tests := []struct {
		name                      string
		enforceTestingEnvironment func() func()

		expectedShortName string
		expectedLongName  string
		expectedVersion   string
	}{
		{
			name: "no file exists",
			enforceTestingEnvironment: func() func() {
				var cleanups []func()

				doesNotExists := filepath.Join(os.TempDir(), "does_not_exists")

				cleanups = append(cleanups, overrideDefaultReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideOracleReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideRedHatReleaseFilepath(doesNotExists))

				return func() {
					for _, cleanup := range cleanups {
						cleanup()
					}
				}
			},
			expectedShortName: "linux",
			expectedLongName:  "",
			expectedVersion:   "",
		},
		{
			name: "default release file exists, but empty",
			enforceTestingEnvironment: func() func() {
				var cleanups []func()

				doesNotExists := filepath.Join(os.TempDir(), "does_not_exists")
				defaultReleaseFile := filepath.Join(os.TempDir(), "default_release_file")
				enforceFileWithContent(t, defaultReleaseFile, []byte(""))

				cleanups = append(cleanups, overrideDefaultReleaseFilepath(defaultReleaseFile))
				cleanups = append(cleanups, overrideOracleReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideRedHatReleaseFilepath(doesNotExists))

				return func() {
					for _, cleanup := range cleanups {
						cleanup()
					}
				}
			},
			expectedShortName: "linux",
			expectedLongName:  "",
			expectedVersion:   "",
		},
		{
			name: "default release path exists, but it not a file",
			enforceTestingEnvironment: func() func() {
				var cleanups []func()

				tmpDir := os.TempDir()
				doesNotExists := filepath.Join(os.TempDir(), "does_not_exists")

				cleanups = append(cleanups, overrideDefaultReleaseFilepath(tmpDir))
				cleanups = append(cleanups, overrideOracleReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideRedHatReleaseFilepath(doesNotExists))

				return func() {
					for _, cleanup := range cleanups {
						cleanup()
					}
				}
			},
			expectedShortName: "linux",
			expectedLongName:  "",
			expectedVersion:   "",
		},
		{
			name: "Debian release file exists",
			enforceTestingEnvironment: func() func() {
				var cleanups []func()

				doesNotExists := filepath.Join(os.TempDir(), "does_not_exists")
				debianReleaseFile := filepath.Join(os.TempDir(), "debian_release_file")

				enforceFileWithContent(t, debianReleaseFile, []byte(debianReleaseFileContent))

				cleanups = append(cleanups, overrideDefaultReleaseFilepath(debianReleaseFile))
				cleanups = append(cleanups, overrideOracleReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideRedHatReleaseFilepath(doesNotExists))

				return func() {
					for _, cleanup := range cleanups {
						cleanup()
					}
				}
			},

			expectedShortName: "debian",
			expectedLongName:  "Debian buster",
			expectedVersion:   "10",
		},
		{
			name: "Oracle Linux release file exists",
			enforceTestingEnvironment: func() func() {
				var cleanups []func()

				doesNotExists := filepath.Join(os.TempDir(), "does_not_exists")
				oracleReleaseFile := filepath.Join(os.TempDir(), "oracle_release_file")

				oracleReleaseFileContent := `Oracle Linux Server release 9.5`
				enforceFileWithContent(t, oracleReleaseFile, []byte(oracleReleaseFileContent))

				cleanups = append(cleanups, overrideDefaultReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideOracleReleaseFilepath(oracleReleaseFile))
				cleanups = append(cleanups, overrideRedHatReleaseFilepath(doesNotExists))

				return func() {
					for _, cleanup := range cleanups {
						cleanup()
					}
				}
			},

			expectedShortName: "ol",
			expectedLongName:  "Oracle Linux Server 9.5",
			expectedVersion:   "9.5",
		},
		{
			name: "Red Hat release file exists",
			enforceTestingEnvironment: func() func() {
				var cleanups []func()

				doesNotExists := filepath.Join(os.TempDir(), "does_not_exists")
				redHatReleaseFile := filepath.Join(os.TempDir(), "redhat_release_file")

				redHatReleaseFileContent := `Red Hat Enterprise Linux release 8.0 (Ootpa)`
				enforceFileWithContent(t, redHatReleaseFile, []byte(redHatReleaseFileContent))

				cleanups = append(cleanups, overrideDefaultReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideOracleReleaseFilepath(doesNotExists))
				cleanups = append(cleanups, overrideRedHatReleaseFilepath(redHatReleaseFile))

				return func() {
					for _, cleanup := range cleanups {
						cleanup()
					}
				}
			},

			expectedShortName: "rhel",
			expectedLongName:  "Red Hat Enterprise Linux 8.0 (Ootpa)",
			expectedVersion:   "8.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.enforceTestingEnvironment()
			defer cleanup()

			osNameAndVersionProvider := getOsNameAndVersionProvider(context.Background())

			shortName, longName, version := osNameAndVersionProvider()
			if tt.expectedShortName != shortName {
				t.Errorf("unexpected value of \"shortName\", expected %q, got %q", tt.expectedShortName, shortName)
			}

			if tt.expectedLongName != longName {
				t.Errorf("unexpected value of \"longName\", expected %q, got %q", tt.expectedLongName, longName)
			}

			if tt.expectedVersion != version {
				t.Errorf("unexpected value of \"version\", expected %q, got %q", tt.expectedVersion, version)
			}
		})
	}
}

func toUtsField(val string) [65]byte {
	var result [65]byte
	for i := 0; i < len(val); i++ {
		result[i] = val[i]
	}
	result[len(val)] = '\x00'

	return result
}

func TestNewLinuxOsInfoProvider(t *testing.T) {
	ctx := context.Background()

	osInfoProvider, err := NewLinuxOsInfoProvider(getOsNameAndVersionProvider(ctx))
	if err != nil {
		t.Errorf("unable to create osInfoProvider, err: %v", err)
		return
	}

	if osInfoProvider == nil {
		t.Errorf("expected fully functional os info provider, but get nil")
	}
}

func overrideDefaultReleaseFilepath(filepath string) (clean func()) {
	prev := defaultReleaseFilepath

	defaultReleaseFilepath = filepath

	return func() {
		defaultReleaseFilepath = prev
	}
}

func overrideOracleReleaseFilepath(filepath string) (clean func()) {
	prev := oracleReleaseFilepath

	oracleReleaseFilepath = filepath

	return func() {
		oracleReleaseFilepath = prev
	}
}

func overrideRedHatReleaseFilepath(filepath string) (clean func()) {
	prev := redHatReleaseFilepath

	redHatReleaseFilepath = filepath

	return func() {
		redHatReleaseFilepath = prev
	}
}

func enforceFileWithContent(t *testing.T, filepath string, content []byte) {
	if err := os.WriteFile(filepath, content, 0644); err != nil {
		t.Errorf("unexpected error while enforcing file content, err: %v", err)
	}
}
