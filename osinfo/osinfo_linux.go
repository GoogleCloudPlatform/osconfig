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

const (
	osRelease = "/etc/os-release"
	oRelease  = "/etc/oracle-release"
	rhRelease = "/etc/redhat-release"
)

func parseOsRelease(releaseDetails string) *OSInfo {
	oi := &OSInfo{}

	scanner := bufio.NewScanner(bytes.NewReader([]byte(releaseDetails)))
	for scanner.Scan() {
		entry := strings.Split(scanner.Text(), "=")
		switch entry[0] {
		case "":
			continue
		case "PRETTY_NAME":
			oi.LongName = strings.Trim(entry[1], `"`)
		case "VERSION_ID":
			oi.Version = strings.Trim(entry[1], `"`)
		case "ID":
			oi.ShortName = strings.Trim(entry[1], `"`)
		}
		if oi.LongName != "" && oi.Version != "" && oi.ShortName != "" {
			break
		}
	}

	if oi.ShortName == "" {
		oi.ShortName = Linux
	}

	return oi
}

func parseEnterpriseRelease(releaseDetails string) *OSInfo {
	rel := releaseDetails

	var sn string
	switch {
	case strings.Contains(rel, "CentOS"):
		sn = "centos"
	case strings.Contains(rel, "Red Hat"):
		sn = "rhel"
	case strings.Contains(rel, "Oracle"):
		sn = "ol"
	}

	return &OSInfo{
		ShortName: sn,
		LongName:  strings.Replace(rel, " release ", " ", 1),
		Version:   entRelVerRgx.FindString(rel),
	}
}

// Get reports OSInfo.
func Get() (*OSInfo, error) {
	var oi *OSInfo
	var parseReleaseFunc func(string) *OSInfo
	var releaseFile string
	switch {
	// Check for /etc/os-release first.
	case util.Exists(osRelease):
		releaseFile = osRelease
		parseReleaseFunc = parseOsRelease
	case util.Exists(oRelease):
		releaseFile = oRelease
		parseReleaseFunc = parseEnterpriseRelease
	case util.Exists(rhRelease):
		releaseFile = rhRelease
		parseReleaseFunc = parseEnterpriseRelease
	}

	b, err := ioutil.ReadFile(releaseFile)
	if err != nil {
		oi = &OSInfo{ShortName: Linux}
	} else {
		oi = parseReleaseFunc(string(b))
	}

	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return oi, fmt.Errorf("unix.Uname error: %v", err)
	}
	// unix.Utsname Fields are [65]byte so we need to trim any trailing null characters.
	oi.Hostname = string(bytes.TrimRight(uts.Nodename[:], "\x00"))
	oi.Architecture = Architecture(string(bytes.TrimRight(uts.Machine[:], "\x00")))
	oi.KernelVersion = string(bytes.TrimRight(uts.Version[:], "\x00"))
	oi.KernelRelease = string(bytes.TrimRight(uts.Release[:], "\x00"))

	return oi, nil
}
