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

	"github.com/GoogleCloudPlatform/osconfig/common"
	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
)

var (
	yum string

	yumInstallArgs     = []string{"install", "-y"}
	yumRemoveArgs      = []string{"remove", "-y"}
	yumCheckUpdateArgs = []string{"-y", "check-update", "--quiet"}
)

func init() {
	if runtime.GOOS != "windows" {
		yum = "/usr/bin/yum"
	}
	YumExists = common.Exists(yum)
}

// InstallYumPackages installs yum packages.
func InstallYumPackages(pkgs []string) error {
	args := append(yumInstallArgs, pkgs...)
	out, err := run(exec.Command(yum, args...))
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("yum install output:\n%s", msg)
	return err
}

// RemoveYumPackages removes yum packages.
func RemoveYumPackages(pkgs []string) error {
	args := append(yumRemoveArgs, pkgs...)
	out, err := run(exec.Command(yum, args...))
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("yum remove output:\n%s", msg)
	return err
}

func parseYumUpdates(data []byte) []PkgInfo {
	/*

	   foo.noarch 2.0.0-1 repo
	   bar.x86_64 2.0.0-1 repo
	   ...
	   Obsoleting Packages
	   ...
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) == 2 && string(pkg[0]) == "Obsoleting" && string(pkg[1]) == "Packages" {
			break
		}
		if len(pkg) != 3 {
			continue
		}
		name := bytes.Split(pkg[0], []byte("."))
		if len(name) != 2 {
			continue
		}
		pkgs = append(pkgs, PkgInfo{Name: string(name[0]), Arch: osinfo.Architecture(string(name[1])), Version: string(pkg[1])})
	}
	return pkgs
}

// YumUpdates queries for all available yum updates.
func YumUpdates() ([]PkgInfo, error) {
	out, err := run(exec.Command(yum, yumCheckUpdateArgs...))
	// Exit code 0 means no updates, 100 means there are updates.
	if err == nil {
		return nil, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 100 {
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error checking yum upgradable packages: %v, stdout: %s", err, out)
	}
	return parseYumUpdates(out), nil
}
