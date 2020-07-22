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
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	yum string

	yumInstallArgs           = []string{"install", "--assumeyes"}
	yumRemoveArgs            = []string{"remove", "--assumeyes"}
	yumCheckUpdateArgs       = []string{"check-update", "--assumeyes"}
	yumUpdateArgs            = []string{"update", "--assumeyes"}
	yumListUpdatesArgs       = []string{"update", "--assumeno", "--cacheonly"}
	yumListUpdateMinimalArgs = []string{"update-minimal", "--assumeno", "--cacheonly"}
)

func init() {
	if runtime.GOOS != "windows" {
		yum = "/usr/bin/yum"
	}
	YumExists = util.Exists(yum)
}

type yumUpdateOpts struct {
	security bool
	minimal  bool
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

// InstallYumPackages installs yum packages.
func InstallYumPackages(ctx context.Context, pkgs []string) error {
	args := append(yumInstallArgs, pkgs...)
	out, err := run(ctx, exec.Command(yum, args...))
	clog.Debugf(ctx, "yum %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		err = fmt.Errorf("error running yum with args %q: %v, stdout: %s", args, err, out)
	}
	return err
}

// UpdateYumPackages updates yum packages.
func UpdateYumPackages(ctx context.Context, pkgs []string) error {
	args := append(yumUpdateArgs, pkgs...)
	out, err := run(ctx, exec.Command(yum, args...))
	clog.Debugf(ctx, "yum %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		err = fmt.Errorf("error running yum with args %q: %v, stdout: %s", args, err, out)
	}
	return err
}

// RemoveYumPackages removes yum packages.
func RemoveYumPackages(ctx context.Context, pkgs []string) error {
	args := append(yumRemoveArgs, pkgs...)
	out, err := run(ctx, exec.Command(yum, args...))
	clog.Debugf(ctx, "yum %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		err = fmt.Errorf("error running yum with args %q: %v, stdout: %s", args, err, out)
	}
	return err
}

func parseYumUpdates(data []byte) []PkgInfo {
	/*
				Last metadata expiration check: 0:11:22 ago on Tue 12 Nov 2019 12:13:38 AM UTC.
				Dependencies resolved.
				=================================================================================================================================================================================
				 Package                                      Arch                           Version                                              Repository                                Size
				=================================================================================================================================================================================
				Installing:
		 		 kernel                                       x86_64                         2.6.32-754.24.3.el6                                  updates                                   32 M
				Updating:
		 		 google-compute-engine                        noarch                         1:20190916.00-g2.el6                                 google-compute-engine                     18 k
				 kernel-firmware                              noarch                         2.6.32-754.24.3.el6                                  updates                                   29 M
		 	 	 libudev                                      x86_64                         147-2.74.el6_10                                      updates                                   78 k
		 		 nspr                                         x86_64                         4.21.0-1.el6_10                                      updates                                  114 k
				 google-cloud-sdk                             noarch                         270.0.0-1                                            google-cloud-sdk                          36 M

				Transaction Summary
				=================================================================================================================================================================================
				Upgrade  5 Packages

				Total download size: 36 M
				Operation aborted.
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []PkgInfo
	var upgrading bool
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) == 0 {
			continue
		}
		// Continue until we see the installing/upgrading section.
		// Yum has this as Updating, dnf is Upgrading.
		if string(pkg[0]) == "Upgrading:" || string(pkg[0]) == "Updating:" || string(pkg[0]) == "Installing:" {
			upgrading = true
			continue
		} else if !upgrading {
			continue
		}
		// A package line should have 6 fields, break unless this is a 'replacing' entry.
		if len(pkg) < 6 {
			if string(pkg[0]) == "replacing" {
				continue
			}
			break
		}
		pkgs = append(pkgs, PkgInfo{Name: string(pkg[0]), Arch: osinfo.Architecture(string(pkg[1])), Version: string(pkg[2])})
	}
	return pkgs
}

// YumUpdates queries for all available yum updates.
func YumUpdates(ctx context.Context, opts ...YumUpdateOption) ([]PkgInfo, error) {
	yumOpts := &yumUpdateOpts{
		security: false,
		minimal:  false,
	}

	for _, opt := range opts {
		opt(yumOpts)
	}

	args := yumUpdateArgs
	if yumOpts.minimal {
		args = yumListUpdateMinimalArgs
	}
	if yumOpts.security {
		args = append(args, "--security")
	}

	// We just use check-update to ensure all repo keys are synced as we run
	// update with --assumeno.
	out, err := run(ctx, exec.Command(yum, yumCheckUpdateArgs...))
	clog.Debugf(ctx, "yum %q output:\n%s", yumCheckUpdateArgs, strings.ReplaceAll(string(out), "\n", "\n "))
	// Exit code 0 means no updates, 100 means there are updates.
	if err == nil {
		return nil, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 100 {
			err = nil
		}
	}
	// Since we don't get good error codes from 'yum update' exit now if there is an issue.
	if err != nil {
		return nil, fmt.Errorf("error running yum with args %q: %v, stdout: %s", yumCheckUpdateArgs, err, out)
	}

	out, err = runWithPty(exec.Command(yum, yumListUpdatesArgs...))
	clog.Debugf(ctx, "yum %q output:\n%s", yumListUpdatesArgs, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		return nil, fmt.Errorf("error running yum with args %q: %v, stdout: %s", yumListUpdatesArgs, err, out)
	}
	if out == nil {
		return nil, nil
	}

	pkgs := parseYumUpdates(out)
	if len(pkgs) == 0 {
		// This means we could not parse any packages and instead got an error from yum.
		return nil, fmt.Errorf("error checking for yum updates, non-zero error code from 'yum update' but no packages parsed, stdout: %s", out)
	}
	return pkgs, nil
}
