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

// +build !windows

package osinfo

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

// debian system with all details in os-release file
// happy case, taken from google desktop
func TestGetDistributionInfoOSRelease(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
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
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	stubs := gostub.Stub(&GetUname, func() ([]byte, error) {
		return []byte("Linux"), nil
	})

	readFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	defer stubs.Reset()

	di, _ := GetDistributionInfo()

	assertion.Equal("Debian buster", di.LongName, "unexpected long name")
	assertion.Equal("debian", di.ShortName, "unexpected short name")
	assertion.Equal("Linux", di.Kernel, "unexpected kernel name")
}

// Error while reading the release file
// should throw error
func TestGetDistributionInfoOSReleaseReadError(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
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
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	stubs := gostub.Stub(&GetUname, func() ([]byte, error) {
		return []byte("Linux"), nil
	})

	readFile = func(file string) ([]byte, error) {
		return []byte(fcontent), errors.New("file read error")
	}

	exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	defer stubs.Reset()

	di, err := GetDistributionInfo()

	assertion.Nil(di)
	assertion.EqualError(err, "unable to obtain release info: file read error")
}

// debian system with empty os-release file
// with empty file, the short name should default to Linux
func TestGetDistributionInfoEmptyOSRelease(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `
`
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	stubs := gostub.Stub(&GetUname, func() ([]byte, error) {
		return []byte("Linux"), nil
	})

	readFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	defer stubs.Reset()

	di, _ := GetDistributionInfo()
	fmt.Printf("+%v", di)
	assertion.Equal("", di.LongName, "unexpected long name")
	assertion.Equal("linux", di.ShortName, "unexpected short name")
	assertion.Equal("Linux", di.Kernel, "unexpected kernel name")
}

// os-release
// normal details of centos system
func TestGetDistributionInfoOracleReleaseCentos(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"

CENTOS_MANTISBT_PROJECT="CentOS-7"
CENTOS_MANTISBT_PROJECT_VERSION="7"
REDHAT_SUPPORT_PRODUCT="centos"
REDHAT_SUPPORT_PRODUCT_VERSION="7"
`
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	stubs := gostub.Stub(&GetUname, func() ([]byte, error) {
		return []byte("Linux"), nil
	})

	readFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	defer stubs.Reset()

	di, _ := GetDistributionInfo()
	assertion.Equal("centos", di.ShortName, "unexpected short name")
	assertion.Equal("Linux", di.Kernel, "unexpected kernel name")
	assertion.Equal("7", di.Version, "unexpected version")
}

// redhat-release
// normal details of redhat system
func TestGetDistributionInfoRedHatRelease(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/redhat-release"
	AppFs.Create(releaseFile)
	fcontent := `Red Hat Enterprise Linux release 8.0 (Ootpa)
`
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	stubs := gostub.Stub(&GetUname, func() ([]byte, error) {
		return []byte("Linux"), nil
	})

	readFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	defer stubs.Reset()

	di, _ := GetDistributionInfo()
	fmt.Printf("sdafASDF: +%v\n ", di)
	assertion.Equal("rhel", di.ShortName, "unexpected short name")
	assertion.Equal("Linux", di.Kernel, "unexpected kernel name")
	assertion.Equal("8.0", di.Version, "unexpected version")
}

// redhat-release
// runtime error during running uname to fetch kernel name
func TestGetDistributionInfoRedHatReleaseUnameError(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/redhat-release"
	AppFs.Create(releaseFile)
	fcontent := `Red Hat Enterprise Linux release 8.0 (Ootpa)
`
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	stubs := gostub.Stub(&GetUname, func() ([]byte, error) {
		return nil, errors.New("error running uname")
	})

	readFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	defer stubs.Reset()

	di, err := GetDistributionInfo()
	assertion.Nil(di)
	assertion.Error(err, "error running uname")
}

// No release file found on the system
func TestGetDistributionInfoNoReleaseFile(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)

	exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	_, err := GetDistributionInfo()

	assertion.EqualError(err, "unable to obtain release info, no known /etc/*-release exists")
}

//TODO: add test case for oracle release system
