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
	"testing"
)

// debian system with all details in os-release file
// happy case, taken from google desktop
func TestGetDistributionInfoOSRelease(t *testing.T) {
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`
	di := parseOsRelease(fcontent)
	tests := []struct {
		expect string
		actual string
		errMsg string
	}{
		{"Debian buster", di.LongName, "unexpected long name"},
		{"debian", di.ShortName, "unexpected short name"},
		{"10", di.Version, "unexpected version id"},
	}

	for _, v := range tests {
		if v.actual != v.expect {
			t.Errorf("%s! expected(%s); got(%s)", v.errMsg, v.expect, v.actual)
		}
	}
}

// debian system with empty os-release file
// with empty file, the short name should default to Linux
func TestGetDistributionInfoEmptyOSRelease(t *testing.T) {
	fcontent := `
`
	di := parseOsRelease(fcontent)
	tests := []struct {
		expectation string
		actual      string
		errMsg      string
	}{
		{"", di.LongName, "unexpected long name"},
		{"linux", di.ShortName, "unexpected short name"},
	}

	for _, v := range tests {
		if v.actual != v.expectation {
			t.Errorf("%s! expected(%s); got(%s)", v.errMsg, v.expectation, v.actual)
		}
	}
}

// os-release
// normal details of centos system
func TestGetDistributionInfoOracleReleaseCentos(t *testing.T) {

	fcontent := `CentOS Linux release 7.6.1810 (Core)`
	di := parseEnterpriseRelease(fcontent)
	tests := []struct {
		expect string
		actual string
		errMsg string
	}{
		{"CentOS Linux 7.6.1810 (Core)", di.LongName, "unexpected long name"},
		{"centos", di.ShortName, "unexpected short name"},
		{"7.6.1810", di.Version, "unexpected version id"},
	}

	for _, v := range tests {
		if v.actual != v.expect {
			t.Errorf("%s! expected(%s); got(%s)", v.errMsg, v.expect, v.actual)
		}
	}
}

//// redhat-release
//// normal details of redhat system
func TestGetDistributionInfoRedHatRelease(t *testing.T) {
	fcontent := `Red Hat Enterprise Linux release 8.0 (Ootpa)`
	di := parseEnterpriseRelease(fcontent)
	tests := []struct {
		expectation string
		actual      string
		errMsg      string
	}{
		{"Red Hat Enterprise Linux 8.0 (Ootpa)", di.LongName, "unexpected long name"},
		{"rhel", di.ShortName, "unexpected short name"},
		{"8.0", di.Version, "unexpected version id"},
	}

	for _, v := range tests {
		if v.actual != v.expectation {
			t.Errorf("%s! expected(%s); got(%s)", v.errMsg, v.expectation, v.actual)
		}
	}
}

//TODO: add test case for oracle release system
