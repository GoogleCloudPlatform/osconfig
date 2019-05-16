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
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
)

var (
	yum string

	yumInstallArgs       = []string{"install", "-y"}
	yumRemoveArgs        = []string{"remove", "-y"}
	yumUpdateArgs        = []string{"update", "-y"}
	yumUpdateMinimalArgs = []string{"update-minimal", "-y"}
	yumCheckUpdateArgs   = []string{"-y", "check-update", "--quiet"}
)

func init() {
	if runtime.GOOS != "windows" {
		yum = "/usr/bin/yum"
	}
	YumExists = exists(yum)
}

// InstallYumPackages installs yum packages.
func InstallYumPackages(pkgs []string) error {
	args := append(yumInstallArgs, pkgs...)
	out, err := run(exec.Command(yum, args...))
	if err != nil {
		return err
	}
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("yum install output:\n%s\n", msg)
	return nil
}

// RemoveYumPackages removes yum packages.
func RemoveYumPackages(pkgs []string) error {
	args := append(yumRemoveArgs, pkgs...)
	out, err := run(exec.Command(yum, args...))
	if err != nil {
		return err
	}
	var msg string
	for _, s := range strings.Split(string(out), "\n") {
		msg += fmt.Sprintf(" %s\n", s)
	}
	DebugLogger.Printf("yum remove output:\n%s\n", msg)
	return nil
}

type yumUpdateOpts struct {
	security bool
	minimal  bool
	excludes []string
}

// YumUpdateOption is an option for yum update.
type YumUpdateOption func(*yumUpdateOpts)

// YumUpdateSecurity returns a YumUpdateOption that specifies the --security flag should
// be used.
func YumUpdateSecurity(security bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.security = security
	}
}

// YumUpdateMinimal returns a YumUpdateOption that specifies the update-minimal
// command should be used.
func YumUpdateMinimal(minimal bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.minimal = minimal
	}
}

// YumUpdateExcludes returns a YumUpdateOption that specifies what packages to add to
// the --exclude flag.
func YumUpdateExcludes(excludes []string) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.excludes = excludes
	}
}

// YumUpdate runs yum update.
func YumUpdate(opts ...YumUpdateOption) error {
	yumOpts := &yumUpdateOpts{
		security: false,
		minimal:  false,
		excludes: nil,
	}

	for _, opt := range opts {
		opt(yumOpts)
	}

	args := yumUpdateArgs
	if yumOpts.minimal {
		args = yumUpdateMinimalArgs
	}
	if yumOpts.security {
		args = append(args, "--security")
	}
	for _, e := range yumOpts.excludes {
		args = append(args, "--exclude="+e)
	}

	if _, err := run(exec.Command(yum, args...)); err != nil {
		return err
	}
	return nil
}

// YumUpdates queries for all available yum updates.
func YumUpdates() ([]PkgInfo, error) {
	out, err := run(exec.Command(yum, yumCheckUpdateArgs...))
	// Exit code 0 means no updates, 100 means there are updates.
	if err == nil {
		return nil, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() == 100 {
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error checking yum upgradable packages: %v, stdout: %s", err, out)
	}
	/*

	   foo.noarch 2.0.0-1 repo
	   bar.x86_64 2.0.0-1 repo
	   ...
	   Obsoleting Packages
	   ...
	*/

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return nil, nil
	}

	var pkgs []PkgInfo
	for _, ln := range lines[1:] {
		pkg := strings.Fields(ln)
		if len(pkg) == 2 && pkg[0] == "Obsoleting" && pkg[1] == "Packages" {
			break
		}
		if len(pkg) != 3 {
			DebugLogger.Printf("%s does not represent a yum update\n", ln)
			continue
		}
		name := strings.Split(pkg[0], ".")
		pkgs = append(pkgs, PkgInfo{Name: name[0], Arch: osinfo.Architecture(name[1]), Version: pkg[1]})
	}
	return pkgs, nil
}
