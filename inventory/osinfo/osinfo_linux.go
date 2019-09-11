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
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

var (
	entRelVerRgx = regexp.MustCompile(`\d+(\.\d+)?(\.\d+)?`)
	getUname     = func() ([]byte, error) {
		return exec.Command("/bin/uname", "-r").CombinedOutput()
	}
)

const (
	osRelease = "/etc/os-release"
	oRelease  = "/etc/oracle-release"
	rhRelease = "/etc/redhat-release"
)

func parseOsRelease(releaseDetails string) (*DistributionInfo, error) {
	di := &DistributionInfo{}

	scanner := bufio.NewScanner(bytes.NewReader([]byte(releaseDetails)))
	for scanner.Scan() {
		entry := strings.Split(scanner.Text(), "=")
		switch entry[0] {
		case "":
			continue
		case "PRETTY_NAME":
			di.LongName = strings.Trim(entry[1], `"`)
		case "VERSION_ID":
			di.Version = strings.Trim(entry[1], `"`)
		case "ID":
			di.ShortName = strings.Trim(entry[1], `"`)
		}
		if di.LongName != "" && di.Version != "" && di.ShortName != "" {
			break
		}
	}

	if di.ShortName == "" {
		di.ShortName = Linux
	}

	return di, nil
}

func parseEnterpriseRelease(releaseDetails string) (*DistributionInfo, error) {
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

	return &DistributionInfo{
		ShortName: sn,
		LongName:  strings.Replace(rel, " release ", " ", 1),
		Version:   entRelVerRgx.FindString(rel),
	}, nil
}

// GetDistributionInfo reports DistributionInfo.
func GetDistributionInfo() (*DistributionInfo, error) {
	var di *DistributionInfo
	var err error
	var b []byte
	switch {
	// Check for /etc/os-release first.
	case exists(osRelease):
		b, err = ioutil.ReadFile(osRelease)
		if err != nil {
			di = &DistributionInfo{ShortName: Linux}
		} else {
			di, err = parseOsRelease(string(b))
		}
	case exists(oRelease):
		b, err = ioutil.ReadFile(oRelease)
		if err != nil {
			di = &DistributionInfo{ShortName: Linux}
		} else {
			di, err = parseEnterpriseRelease(string(b))
		}
	case exists(rhRelease):
		b, err = ioutil.ReadFile(rhRelease)
		if err != nil {
			di = &DistributionInfo{ShortName: Linux}
		} else {
			di, err = parseEnterpriseRelease(string(b))
		}
	default:
		err = errors.New("unable to obtain release info, no known /etc/*-release exists")
	}
	if err != nil {
		return nil, err
	}

	out, err := getUname()
	if err != nil {
		return nil, err
	}
	di.Kernel = strings.TrimSpace(string(out))
	// No need to get fancy here, assume the binary architecture
	// is the same as the system.
	di.Architecture = Architecture(runtime.GOARCH)
	return di, nil
}

func exists(name string) bool {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false
	}
	return true
}
