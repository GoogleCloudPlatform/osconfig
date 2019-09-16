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
	"os/exec"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	zypper string

	zypperInstallArgs     = []string{"install", "--no-confirm"}
	zypperRemoveArgs      = []string{"remove", "--no-confirm"}
	zypperListUpdatesArgs = []string{"-q", "list-updates"}
	zypperListPatchesArgs = []string{"-q", "list-patches", "--all", "--with-optional"}
)

func init() {
	if runtime.GOOS != "windows" {
		zypper = "/usr/bin/zypper"
	}
	ZypperExists = util.Exists(zypper)
}

// InstallZypperPackages Installs zypper packages
func InstallZypperPackages(pkgs []string) error {
	args := append(zypperInstallArgs, pkgs...)
	out, err := run(exec.Command(zypper, args...))
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("Zypper install output:\n%s", msg)
	return err
}

// RemoveZypperPackages installed Zypper packages.
func RemoveZypperPackages(pkgs []string) error {
	args := append(zypperRemoveArgs, pkgs...)
	out, err := run(exec.Command(zypper, args...))
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf("  %s\n", s)
	}
	DebugLogger.Printf("Zypper remove output:\n%s", msg)
	return err
}

func parseZypperUpdates(data []byte) []PkgInfo {
	/*
		      S | Repository          | Name                   | Current Version | Available Version | Arch
		      --+---------------------+------------------------+-----------------+-------------------+-------
		      v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64
		      v | SLES12-SP3-Updates  | autoyast2-installation | 3.2.17-1.3      | 3.2.22-2.9.2      | noarch
			   ...
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := bytes.Split(ln, []byte("|"))
		if len(pkg) != 6 || string(bytes.TrimSpace(pkg[0])) != "v" {
			continue
		}
		name := string(bytes.TrimSpace(pkg[2]))
		arch := string(bytes.TrimSpace(pkg[5]))
		ver := string(bytes.TrimSpace(pkg[4]))
		pkgs = append(pkgs, PkgInfo{Name: name, Arch: osinfo.Architecture(arch), Version: ver})
	}
	return pkgs
}

// ZypperUpdates queries for all available zypper updates.
func ZypperUpdates() ([]PkgInfo, error) {
	out, err := run(exec.Command(zypper, zypperListUpdatesArgs...))
	if err != nil {
		return nil, err
	}
	return parseZypperUpdates(out), nil
}

func parseZypperPatches(data []byte) ([]ZypperPatch, []ZypperPatch) {
	/*
		Repository                          | Name                                        | Category    | Severity  | Interactive | Status     | Summary
		------------------------------------+---------------------------------------------+-------------+-----------+-------------+------------+------------------------------------------------------------
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1206 | security    | low       | ---         | applied    | Security update for bzip2
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1221 | security    | moderate  | ---         | applied    | Security update for libxslt
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1229 | recommended | moderate  | ---         | not needed | Recommended update for sensors
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var ins []ZypperPatch
	var avail []ZypperPatch
	for _, ln := range lines {
		patch := bytes.Split(ln, []byte("|"))
		if len(patch) != 7 {
			continue
		}

		name := string(bytes.TrimSpace(patch[1]))
		cat := string(bytes.TrimSpace(patch[2]))
		sev := string(bytes.TrimSpace(patch[3]))
		status := string(bytes.TrimSpace(patch[5]))
		sum := string(bytes.TrimSpace(patch[6]))
		switch status {
		case "needed":
			avail = append(avail, ZypperPatch{Name: name, Category: cat, Severity: sev, Summary: sum})
		case "applied":
			ins = append(ins, ZypperPatch{Name: name, Category: cat, Severity: sev, Summary: sum})
		default:
			continue
		}
	}
	return ins, avail
}

func zypperPatches() ([]byte, error) {
	return run(exec.Command(zypper, zypperListPatchesArgs...))
}

// ZypperPatches queries for all available zypper patches.
func ZypperPatches() ([]ZypperPatch, error) {
	out, err := zypperPatches()
	if err != nil {
		return nil, err
	}
	_, patches := parseZypperPatches(out)
	return patches, nil
}

// ZypperInstalledPatches queries for all installed zypper patches.
func ZypperInstalledPatches() ([]ZypperPatch, error) {
	out, err := zypperPatches()
	if err != nil {
		return nil, err
	}
	patches, _ := parseZypperPatches(out)
	return patches, nil
}
