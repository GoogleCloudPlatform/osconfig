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
	"os"
	"os/exec"
	"runtime"
	"slices"
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
	yumListUpdatesArgs       = []string{"update", "--assumeno", "--cacheonly", "--color=never"}
	yumListUpdateMinimalArgs = []string{"update-minimal", "--assumeno", "--cacheonly", "--color=never"}
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
	_, err := run(ctx, yum, append(yumInstallArgs, pkgs...))
	return err
}

// RemoveYumPackages removes yum packages.
func RemoveYumPackages(ctx context.Context, pkgs []string) error {
	_, err := run(ctx, yum, append(yumRemoveArgs, pkgs...))
	return err
}

func parseYumUpdates(data []byte) []*PkgInfo {
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

	var pkgs []*PkgInfo
	var upgrading bool
	packagesInstallOrUpdateKeywords := []string{"Upgrading:", "Updating:", "Installing:", "Installing dependencies:", "Installing weak dependencies:"}
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) == 0 {
			continue
		}
		// Continue until we see one of the installing/upgrading keywords section.
		// Yum has this as Updating, dnf is Upgrading.
		if slices.Contains(packagesInstallOrUpdateKeywords, string(bytes.Join(pkg, []byte(" ")))) {
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
		pkgs = append(pkgs, &PkgInfo{Name: string(pkg[0]), Arch: osinfo.NormalizeArchitecture(string(pkg[1])), RawArch: string(pkg[1]), Version: string(pkg[2])})
	}
	return pkgs
}

func getYumTXFile(data []byte) string {
	/* The last lines of a non-complete yum update where the transaction
	   is saved look like:

	     Exiting on user command
	     Your transaction was saved, rerun it with:
	      yum load-transaction /tmp/yum_save_tx.2022-10-12.20-26.j3auah.yumtx
	*/
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	for _, ln := range lines {
		flds := bytes.Fields(ln)
		if len(flds) == 3 && string(flds[1]) == "load-transaction" && strings.HasPrefix(string(flds[2]), "/tmp/yum_save_tx.") {
			return string(flds[2])
		}
	}
	return ""
}

// YumUpdates queries for all available yum updates.
func YumUpdates(ctx context.Context, opts ...YumUpdateOption) ([]*PkgInfo, error) {
	// We just use check-update to ensure all repo keys are synced as we run
	// update with --assumeno.
	stdout, stderr, err := runner.Run(ctx, exec.CommandContext(ctx, yum, yumCheckUpdateArgs...))
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
		return nil, fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", yum, yumCheckUpdateArgs, err, stdout, stderr)
	}

	return listAndParseYumPackages(ctx, opts...)
}

func listAndParseYumPackages(ctx context.Context, opts ...YumUpdateOption) ([]*PkgInfo, error) {
	yumOpts := &yumUpdateOpts{
		security: false,
		minimal:  false,
	}

	for _, opt := range opts {
		opt(yumOpts)
	}

	args := yumListUpdatesArgs
	if yumOpts.minimal {
		args = yumListUpdateMinimalArgs
	}
	if yumOpts.security {
		args = append(args, "--security")
	}

	stdout, stderr, err := ptyrunner.Run(ctx, exec.CommandContext(ctx, yum, args...))
	if err != nil {
		return nil, fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", yum, args, err, stdout, stderr)
	}
	if stdout == nil {
		return nil, nil
	}

	// Some versions of yum will leave a transaction file in /tmp when update
	// is run with --assumeno. This will attempt to delete that file.
	yumTXFile := getYumTXFile(stdout)
	if yumTXFile != "" {
		clog.Debugf(ctx, "Removing yum tx file: %s", yumTXFile)
		if err := os.Remove(yumTXFile); err != nil {
			clog.Debugf(ctx, "Error deleting yum tx file %s: %v", yumTXFile, err)
		}
	}

	pkgs := parseYumUpdates(stdout)
	if len(pkgs) == 0 {
		// This means we could not parse any packages and instead got an error from yum.
		return nil, fmt.Errorf("error checking for yum updates, non-zero error code from 'yum update' but no packages parsed, stdout: %q", stdout)
	}
	return pkgs, nil
}
