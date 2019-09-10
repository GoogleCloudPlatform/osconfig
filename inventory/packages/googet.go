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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	googet string

	googetUpdateQueryArgs    = []string{"update"}
	googetInstalledQueryArgs = []string{"installed"}
	googetInstallArgs        = []string{"-noconfirm", "install"}
	googetRemoveArgs         = []string{"-noconfirm", "remove"}
)

func init() {
	if runtime.GOOS == "windows" {
		googet = filepath.Join(os.Getenv("GooGetRoot"), "googet.exe")
	}
	GooGetExists = exists(googet)
}

func parseGooGetUpdates(data []byte) []PkgInfo {
	/*
	   Searching for available updates...
	   foo.noarch, 3.5.4@1 --> 3.6.7@1 from repo
	   ...
	   Perform update? (y/N):
	*/
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := strings.Fields(ln)
		if len(pkg) < 4 {
			continue
		}

		p := strings.Split(pkg[0], ".")
		if len(p) != 2 {
			continue
		}
		pkgs = append(pkgs, PkgInfo{Name: p[0], Arch: strings.Trim(p[1], ","), Version: pkg[3]})
	}
	return pkgs
}

// GooGetUpdates queries for all available googet updates.
func GooGetUpdates() ([]PkgInfo, error) {
	out, err := util.Run(exec.Command(googet, googetUpdateQueryArgs...), DebugLogger)
	if err != nil {
		return nil, err
	}

	return parseGooGetUpdates(out), nil
}

// InstallGooGetPackages installs GooGet packages.
func InstallGooGetPackages(pkgs []string) error {
	args := append(googetInstallArgs, pkgs...)
	out, err := util.Run(exec.Command(googet, args...), DebugLogger)
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf("  %s\n", s)
	}
	DebugLogger.Printf("GooGet install output:\n%s", msg)
	return err
}

// RemoveGooGetPackages installs GooGet packages.
func RemoveGooGetPackages(pkgs []string) error {
	args := append(googetRemoveArgs, pkgs...)
	out, err := util.Run(exec.Command(googet, args...), DebugLogger)
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf("  %s\n", s)
	}
	DebugLogger.Printf("GooGet remove output:\n%s", msg)
	return err
}

func parseInstalledGooGetPackages(data []byte) []PkgInfo {
	/*
	   Installed Packages:
	   foo.x86_64 1.2.3@4
	   bar.noarch 1.2.3@4
	   ...
	*/
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []PkgInfo
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) != 2 {
			continue
		}

		p := bytes.Split(pkg[0], []byte("."))
		if len(p) != 2 {
			continue
		}

		pkgs = append(pkgs, PkgInfo{Name: string(p[0]), Arch: string(p[1]), Version: string(pkg[1])})
	}
	return pkgs
}

// InstalledGooGetPackages queries for all installed googet packages.
func InstalledGooGetPackages() ([]PkgInfo, error) {
	out, err := util.Run(exec.Command(googet, googetInstalledQueryArgs...), DebugLogger)
	if err != nil {
		return nil, err
	}

	return parseInstalledGooGetPackages(out), nil
}
