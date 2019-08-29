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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/common"
	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
)

var (
	// dpkg-query
	dpkgquery string
	aptGet    string

	dpkgQueryArgs        = []string{"-W", "-f", "${Package} ${Architecture} ${Version}\n"}
	aptGetInstallArgs    = []string{"install", "-y"}
	aptGetRemoveArgs     = []string{"remove", "-y"}
	aptGetUpdateArgs     = []string{"update"}
	aptGetUpgradableArgs = []string{"full-upgrade", "--just-print", "-V"}
)

func init() {
	if runtime.GOOS != "windows" {
		dpkgquery = "/usr/bin/dpkg-query"
		aptGet = "/usr/bin/apt-get"
	}
	AptExists = common.Exists(aptGet)
}

// InstallAptPackages installs apt packages.
func InstallAptPackages(pkgs []string) error {
	args := append(aptGetInstallArgs, pkgs...)
	install := exec.Command(aptGet, args...)
	install.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)
	out, err := run(install)
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("apt install output:\n%s", msg)
	return err
}

// RemoveAptPackages removes apt packages.
func RemoveAptPackages(pkgs []string) error {
	args := append(aptGetRemoveArgs, pkgs...)
	remove := exec.Command(aptGet, args...)
	remove.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)
	out, err := run(remove)
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("apt remove output:\n%s", msg)
	return err
}

func parseAptUpdates(data []byte) []PkgInfo {
	/*
		Reading package lists... Done
		Building dependency tree
		Reading state information... Done
		Calculating upgrade... Done
		The following NEW packages will be installed:
		  firmware-linux-free linux-image-4.9.0-9-amd64
		The following packages will be upgraded:
		  google-cloud-sdk linux-image-amd64
		2 upgraded, 2 newly installed, 0 to remove and 0 not upgraded.
		Inst libldap-common [2.4.45+dfsg-1ubuntu1.2] (2.4.45+dfsg-1ubuntu1.3 Ubuntu:18.04/bionic-updates, Ubuntu:18.04/bionic-security [all])
		Inst firmware-linux-free (3.4 Debian:9.9/stable [all])
		Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		Inst linux-image-4.9.0-9-amd64 (4.9.168-1+deb9u2 Debian-Security:9/stable [amd64])
		Inst linux-image-amd64 [4.9+80+deb9u6] (4.9+80+deb9u7 Debian:9.9/stable [amd64])
		Conf firmware-linux-free (3.4 Debian:9.9/stable [all])
		Conf google-cloud-sdk (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		Conf linux-image-4.9.0-9-amd64 (4.9.168-1+deb9u2 Debian-Security:9/stable [amd64])
		Conf linux-image-amd64 (4.9+80+deb9u7 Debian:9.9/stable [amd64])
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) < 4 || string(pkg[0]) != "Inst" {
			continue
		}
		if strings.HasPrefix(string(pkg[2]), "(") {
			// We don't want to record new installs.
			// Inst firmware-linux-free (3.4 Debian:9.9/stable [all])
			continue
		}
		// Make sure this line matches expectations:
		// Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		if !strings.HasPrefix(string(pkg[2]), "[") || !strings.HasPrefix(string(pkg[3]), "(") || !strings.HasSuffix(string(pkg[len(pkg)-1]), ")") {
			continue
		}
		ver := bytes.Trim(pkg[3], "(")
		arch := bytes.Trim(pkg[len(pkg)-1], "[])")
		pkgs = append(pkgs, PkgInfo{Name: string(pkg[1]), Arch: osinfo.Architecture(string(arch)), Version: string(ver)})
	}
	return pkgs
}

// AptUpdates queries for all available apt updates.
func AptUpdates() ([]PkgInfo, error) {
	if _, err := run(exec.Command(aptGet, aptGetUpdateArgs...)); err != nil {
		return nil, err
	}

	out, err := run(exec.Command(aptGet, aptGetUpgradableArgs...))
	if err != nil {
		return nil, err
	}

	return parseAptUpdates(out), nil
}

func parseInstalledDebpackages(data []byte) []PkgInfo {
	/*
	   foo amd64 1.2.3-4
	   bar noarch 1.2.3-4
	   ...
	*/
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := strings.Fields(ln)
		if len(pkg) != 3 {
			continue
		}

		pkgs = append(pkgs, PkgInfo{Name: pkg[0], Arch: osinfo.Architecture(pkg[1]), Version: pkg[2]})
	}
	return pkgs
}

// InstalledDebPackages queries for all installed deb packages.
func InstalledDebPackages() ([]PkgInfo, error) {
	out, err := run(exec.Command(dpkgquery, dpkgQueryArgs...))
	if err != nil {
		return nil, err
	}
	return parseInstalledDebpackages(out), nil
}
