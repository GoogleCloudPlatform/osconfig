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

package ospatch

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/GoogleCloudPlatform/osconfig/packages"
)

const (
	rpmquery = "/usr/bin/rpmquery"
)

func getBtime(stat string) (int, error) {
	f, err := os.Open(stat)
	if err != nil {
		return 0, fmt.Errorf("error opening %s: %v", stat, err)
	}
	defer f.Close()

	var btime int
	scnr := bufio.NewScanner(f)
	for scnr.Scan() {
		if bytes.HasPrefix(scnr.Bytes(), []byte("btime")) {
			split := bytes.SplitN(scnr.Bytes(), []byte(" "), 2)
			if len(split) != 2 {
				return 0, fmt.Errorf("error parsing btime from %s: %q", stat, scnr.Text())
			}
			btime, err = strconv.Atoi(string(bytes.TrimSpace(split[1])))
			if err != nil {
				return 0, fmt.Errorf("error parsing btime: %v", err)
			}
			break
		}
	}
	if err := scnr.Err(); err != nil && btime == 0 {
		return 0, fmt.Errorf("error scanning %s: %v", stat, err)
	}
	if btime == 0 {
		return 0, fmt.Errorf("could not find btime in %s", stat)
	}

	return btime, nil
}

func rpmRebootRequired(pkgs []byte, btime int) bool {
	// Scanning this output is best effort, false negatives are much prefered
	// to false positives, and keeping this as simple as possible is
	// beneficial.
	scnr := bufio.NewScanner(bytes.NewReader(pkgs))
	for scnr.Scan() {
		itime, err := strconv.Atoi(scnr.Text())
		if err != nil {
			continue
		}
		if itime > btime {
			return true
		}
	}

	return false
}

// rpmReboot returns whether an rpm based system should reboot in order to
// finish installing updates.
// To get this signal we look at a set of well known packages and whether
// install time > system boot time. This list is not meant to be exhastive,
// just to provide a signal when core system packages are updated.
func rpmReboot() (bool, error) {
	provides := []string{
		// Common packages.
		"kernel", "glibc", "gnutls",
		// EL packages.
		"linux-firmware", "openssl-libs", "dbus",
		// Suse packages.
		"kernel-firmware", "libopenssl1_1", "libopenssl1_0_0", "dbus-1",
	}
	args := append([]string{"--queryformat", "%{INSTALLTIME}\n", "--whatprovides"}, provides...)
	out, err := exec.Command(rpmquery, args...).Output()
	if err != nil {
		// We don't care about return codes as we know some of these packages won't be installed.
		if _, ok := err.(*exec.ExitError); !ok {
			return false, fmt.Errorf("error running %s: %v", rpmquery, err)
		}
	}

	btime, err := getBtime("/proc/stat")
	if err != nil {
		return false, err
	}

	return rpmRebootRequired(out, btime), nil
}

func containsString(ss []string, c string) bool {
	for _, s := range ss {
		if s == c {
			return true
		}
	}
	return false
}

func filterPackages(pkgs []packages.PkgInfo, exclusivePackages, excludes []string) ([]packages.PkgInfo, error) {
	if len(exclusivePackages) != 0 && len(excludes) != 0 {
		return nil, errors.New("exclusivePackages and excludes can not both be non 0")
	}
	var fPkgs []packages.PkgInfo
	for _, pkg := range pkgs {
		if containsString(excludes, pkg.Name) {
			continue
		}
		if exclusivePackages == nil || containsString(exclusivePackages, pkg.Name) {
			fPkgs = append(fPkgs, pkg)
		}
	}
	return fPkgs, nil
}
