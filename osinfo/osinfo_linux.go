/*
Copyright 2017 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package osinfo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/util"
	"golang.org/x/sys/unix"
)

var (
	entRelVerRgx = regexp.MustCompile(`\d+(\.\d+)?(\.\d+)?`)
)

var _ Provider = &LinuxOsInfoProvider{}

var (
	defaultReleaseFilepath = "/etc/os-release"
	oracleReleaseFilepath  = "/etc/oracle-release"
	redHatReleaseFilepath  = "/etc/redhat-release"
)

// Get reports OSInfo.
func Get() (OSInfo, error) {
	// Eventually we will get rid of this function and will use providers directly
	// Providers should support context to be able to handle cancelation and logging
	// so far we just create empty context to connect the API.
	ctx := context.TODO()

	osInfoProvider, err := NewLinuxOsInfoProvider(getOsNameAndVersionProvider(ctx))
	if err != nil {
		return OSInfo{}, fmt.Errorf("unable to extract osinfo, err:  %w", err)
	}

	osInfo, err := osInfoProvider.GetOSInfo(ctx)
	if err != nil {
		return osInfo, err
	}

	return osInfo, nil
}

// LinuxOsInfoProvider is a provider of OSInfo for the linux based systems.
type LinuxOsInfoProvider struct {
	nameAndVersionProvider osNameAndVersionProvider
	uts                    unix.Utsname
}

// NewLinuxOsInfoProvider is a constructor function for LinuxOsInfoProvider.
func NewLinuxOsInfoProvider(nameAndVersionProvider osNameAndVersionProvider) (*LinuxOsInfoProvider, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return nil, fmt.Errorf("unable to get unix.Uname, err: %w", err)
	}

	return &LinuxOsInfoProvider{
		nameAndVersionProvider: nameAndVersionProvider,
		uts:                    uts,
	}, nil
}

// GetOSInfo gather all required information and returns OSInfo.
func (oip *LinuxOsInfoProvider) GetOSInfo(ctx context.Context) (OSInfo, error) {
	short, long, version := oip.nameAndVersionProvider()

	return OSInfo{
		ShortName: short,
		LongName:  long,
		Version:   version,

		Hostname:      oip.hostName(),
		Architecture:  oip.architecture(),
		KernelRelease: oip.kernelRelease(),
		KernelVersion: oip.kernelVersion(),
	}, nil
}

func (oip *LinuxOsInfoProvider) hostName() string {
	return stringFromUtsField(oip.uts.Nodename)
}

func (oip *LinuxOsInfoProvider) architecture() string {
	return NormalizeArchitecture(stringFromUtsField(oip.uts.Machine))
}
func (oip *LinuxOsInfoProvider) kernelRelease() string {
	return stringFromUtsField(oip.uts.Release)
}

func (oip *LinuxOsInfoProvider) kernelVersion() string {
	return stringFromUtsField(oip.uts.Version)
}

func stringFromUtsField(field [65]byte) string {
	// unix.Utsname Fields are [65]byte so we need to trim any trailing null characters.
	return string(bytes.TrimRight(field[:], "\x00"))
}

func getOsNameAndVersionProvider(_ context.Context) osNameAndVersionProvider {
	return func() (string, string, string) {
		var (
			extractNameAndVersion func(string) (string, string, string)
			releaseFile           string
		)

		defaultShortName, defaultLongName, defaultVersion := DefaultShortNameLinux, "", ""

		switch {
		// Check for /etc/os-release first.
		case util.Exists(defaultReleaseFilepath):
			releaseFile = defaultReleaseFilepath
			extractNameAndVersion = parseOsRelease
		case util.Exists(oracleReleaseFilepath):
			releaseFile = oracleReleaseFilepath
			extractNameAndVersion = parseEnterpriseRelease
		case util.Exists(redHatReleaseFilepath):
			releaseFile = redHatReleaseFilepath
			extractNameAndVersion = parseEnterpriseRelease
		default:
			return defaultShortName, defaultLongName, defaultVersion
		}

		b, err := ioutil.ReadFile(releaseFile)
		if err != nil {
			// TODO: log an error
			return defaultShortName, defaultLongName, defaultVersion
		}

		return extractNameAndVersion(string(b))
	}
}

func parseOsRelease(releaseDetails string) (shortName, longName, version string) {
	scanner := bufio.NewScanner(strings.NewReader(releaseDetails))

	for scanner.Scan() {
		entry := strings.Split(scanner.Text(), "=")
		switch entry[0] {
		case "":
			continue
		case "PRETTY_NAME":
			longName = strings.Trim(entry[1], `"`)
		case "VERSION_ID":
			version = strings.Trim(entry[1], `"`)
		case "ID":
			shortName = strings.Trim(entry[1], `"`)
		}

		// TODO: Replace with binary mask
		if longName != "" && version != "" && shortName != "" {
			break
		}
	}

	if shortName == "" {
		shortName = DefaultShortNameLinux
	}

	return shortName, longName, version
}

func parseEnterpriseRelease(releaseDetails string) (shortName string, longName string, version string) {
	shortName = DefaultShortNameLinux

	switch {
	case strings.Contains(releaseDetails, "CentOS"):
		shortName = "centos"
	case strings.Contains(releaseDetails, "Red Hat"):
		shortName = "rhel"
	case strings.Contains(releaseDetails, "Oracle"):
		shortName = "ol"
	}

	longName = strings.Replace(releaseDetails, " release ", " ", 1)

	version = entRelVerRgx.FindString(releaseDetails)

	return shortName, longName, version
}
