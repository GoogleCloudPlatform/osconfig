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
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	zypper string

	// zypperInstallArgs is zypper command to install patches, packages
	zypperInstallArgs     = []string{"--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses"}
	zypperRemoveArgs      = []string{"--non-interactive", "remove"}
	zypperListUpdatesArgs = []string{"--gpg-auto-import-keys", "-q", "list-updates"}
	zypperListPatchesArgs = []string{"--gpg-auto-import-keys", "-q", "list-patches"}
	zypperPatchInfoArgs   = []string{"info", "-t", "patch"}
)

func init() {
	if runtime.GOOS != "windows" {
		zypper = "/usr/bin/zypper"
	}
	ZypperExists = util.Exists(zypper)
}

type zypperListPatchOpts struct {
	categories   []string
	severities   []string
	withOptional bool
	all          bool
}

// ZypperListOption is patch list options
type ZypperListOption func(opts *zypperListPatchOpts)

// ZypperListPatchCategories is zypper list option to provide category filter
func ZypperListPatchCategories(categories []string) ZypperListOption {
	return func(args *zypperListPatchOpts) {
		args.categories = categories
	}
}

// ZypperListPatchSeverities is zypper list option to provide severity filter
func ZypperListPatchSeverities(severities []string) ZypperListOption {
	return func(args *zypperListPatchOpts) {
		args.severities = severities
	}
}

// ZypperListPatchWithOptional is zypper list option to also list optional patches
func ZypperListPatchWithOptional(withOptional bool) ZypperListOption {
	return func(args *zypperListPatchOpts) {
		args.withOptional = withOptional
	}
}

// ZypperListPatchAll is zypper list option to all all patches
func ZypperListPatchAll(all bool) ZypperListOption {
	return func(args *zypperListPatchOpts) {
		args.all = all
	}
}

// InstallZypperPackages Installs zypper packages
func InstallZypperPackages(ctx context.Context, pkgs []string) error {
	_, err := run(ctx, zypper, append(zypperInstallArgs, pkgs...))
	return err
}

// ZypperInstall installs zypper patches and packages
func ZypperInstall(ctx context.Context, patches []*ZypperPatch, pkgs []*PkgInfo) error {
	args := zypperInstallArgs

	// https://www.mankier.com/8/zypper#Concepts-Package_Types use patch install
	// for single patch and package installs
	for _, patch := range patches {
		args = append(args, "patch:"+patch.Name)
	}
	for _, pkg := range pkgs {
		args = append(args, "package:"+pkg.Name)
	}

	stdout, stderr, err := runner.Run(ctx, exec.Command(zypper, args...))
	// https://en.opensuse.org/SDB:Zypper_manual#EXIT_CODES
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// ZYPPER_EXIT_INF_REBOOT_NEEDED
			if exitErr.ExitCode() == 102 {
				err = nil
			}
		} else {
			err = fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", zypper, args, err, stdout, stderr)
		}
	}

	return err
}

// RemoveZypperPackages installed Zypper packages.
func RemoveZypperPackages(ctx context.Context, pkgs []string) error {
	_, err := run(ctx, zypper, append(zypperRemoveArgs, pkgs...))
	return err
}

func parseZypperUpdates(data []byte) []*PkgInfo {
	/*
		      S | Repository          | Name                   | Current Version | Available Version | Arch
		      --+---------------------+------------------------+-----------------+-------------------+-------
		      v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64
		      v | SLES12-SP3-Updates  | autoyast2-installation | 3.2.17-1.3      | 3.2.22-2.9.2      | noarch
			   ...
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []*PkgInfo
	for _, ln := range lines {
		pkg := bytes.Split(ln, []byte("|"))
		if len(pkg) != 6 || string(bytes.TrimSpace(pkg[0])) != "v" {
			continue
		}
		name := string(bytes.TrimSpace(pkg[2]))
		arch := string(bytes.TrimSpace(pkg[5]))
		ver := string(bytes.TrimSpace(pkg[4]))
		pkgs = append(pkgs, &PkgInfo{Name: name, Arch: osinfo.Architecture(arch), Version: ver})
	}
	return pkgs
}

// ZypperUpdates queries for all available zypper updates.
func ZypperUpdates(ctx context.Context) ([]*PkgInfo, error) {
	out, err := run(ctx, zypper, zypperListUpdatesArgs)
	if err != nil {
		return nil, err
	}
	return parseZypperUpdates(out), nil
}

func parseZypperPatches(data []byte) ([]*ZypperPatch, []*ZypperPatch) {
	/*
		Repository                          | Name                                        | Category    | Severity  | Interactive | Status     | Summary
		------------------------------------+---------------------------------------------+-------------+-----------+-------------+------------+------------------------------------------------------------
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1206 | security    | low       | ---         | applied    | Security update for bzip2
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1221 | security    | moderate  | ---         | applied    | Security update for libxslt
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1229 | recommended | moderate  | ---         | not needed | Recommended update for sensors
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var ins []*ZypperPatch
	var avail []*ZypperPatch
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
			avail = append(avail, &ZypperPatch{Name: name, Category: cat, Severity: sev, Summary: sum})
		case "applied":
			ins = append(ins, &ZypperPatch{Name: name, Category: cat, Severity: sev, Summary: sum})
		default:
			continue
		}
	}
	return ins, avail
}

func zypperPatches(ctx context.Context, opts ...ZypperListOption) ([]byte, error) {
	zOpts := &zypperListPatchOpts{
		categories:   nil,
		severities:   nil,
		withOptional: false,
		all:          false,
	}

	for _, opt := range opts {
		opt(zOpts)
	}

	args := zypperListPatchesArgs
	for _, c := range zOpts.categories {
		args = append(args, "--category="+c)
	}

	for _, s := range zOpts.severities {
		args = append(args, "--severity="+s)
	}

	if zOpts.withOptional {
		args = append(args, "--with-optional")
	}

	//  As per zypper's current implementation,
	// --all is ignored if we have any filters on any other
	// field.
	if zOpts.all || (len(zOpts.severities)+len(zOpts.categories)) <= 0 {
		args = append(args, "--all")
	}

	return run(ctx, zypper, args)
}

// ZypperPatches queries for all available zypper patches.
func ZypperPatches(ctx context.Context, opts ...ZypperListOption) ([]*ZypperPatch, error) {
	out, err := zypperPatches(ctx, opts...)
	if err != nil {
		return nil, err
	}
	_, patches := parseZypperPatches(out)
	return patches, nil
}

// ZypperInstalledPatches queries for all installed zypper patches.
func ZypperInstalledPatches(ctx context.Context, opts ...ZypperListOption) ([]*ZypperPatch, error) {
	out, err := zypperPatches(ctx, opts...)
	if err != nil {
		return nil, err
	}
	patches, _ := parseZypperPatches(out)
	return patches, nil
}

func zypperPatchInfo(ctx context.Context, patches []string) ([]byte, error) {
	args := zypperPatchInfoArgs
	for _, name := range patches {
		args = append(args, name)
	}
	return run(ctx, zypper, args)
}

func parseZypperPatchInfo(out []byte) (map[string][]string, error) {
	/*
		Loading repository data...
		Reading installed packages...
		Information for patch SUSE-SLE-SERVER-12-SP4-2019-2974:
		-------------------------------------------------------
		Repository  : SLES12-SP4-Updates
		Name        : SUSE-SLE-SERVER-12-SP4-2019-2974
		Version     : 1
		Arch        : noarch
		Vendor      : maint-coord@suse.de
		Status      : needed
		Category    : recommended
		Severity    : important
		Created On  : Thu Nov 14 13:17:48 2019
		Interactive : ---
		Summary     : Recommended update for irqbalance
		Description :
		    This update for irqbalance fixes the following issues:
		    - Irqbalanced spreads the IRQs between the available virtual machines. (bsc#1119465, bsc#1154905)
		Provides    : patch:SUSE-SLE-SERVER-12-SP4-2019-2974 = 1
		Conflicts   : [2]
		    irqbalance.src < 1.1.0-9.3.1
		    irqbalance.x86_64 < 1.1.0-9.3.1
	*/
	patchInfo := make(map[string][]string)
	var validConflictLine = regexp.MustCompile(`\s*Conflicts\s*:\s*\[\d*\]\s*`)
	var conflictLineExtract = regexp.MustCompile(`\[[0-9]+\]`)
	var nameLine = regexp.MustCompile(`\s*Name\s*:\s*`)
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	i := 0
	for {
		// find the name line
		for ; i < len(lines); i++ {
			b := nameLine.Find([]byte(lines[i]))
			if b != nil {
				break
			}
		}
		if i >= len(lines) {
			// we do not have any more patch info blobs
			break
		}

		parts := strings.Split(string(lines[i]), ":")
		i++

		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid name output")
		}
		patchName := strings.Trim(parts[1], " ")

		for ; i < len(lines); i++ {
			b := validConflictLine.Find([]byte(lines[i]))
			if b != nil {
				//	Conflicts   : [2]
				break
			}
		}

		if i >= len(lines) {
			// did not find conflicting packages
			// this should not happen
			return nil, nil
		}

		matches := conflictLineExtract.FindAllString(string(lines[i]), -1)
		if len(matches) != 1 {
			return nil, fmt.Errorf("invalid patch info")
		}

		// get the number of package lines to parse
		conflicts := strings.Trim(matches[0], "[")
		conflicts = strings.Trim(conflicts, "]")
		conflictLines, err := strconv.Atoi(conflicts)
		if err != nil {
			return nil, fmt.Errorf("invalid patch info: invalid conflict info")
		}
		ctr := i + 1
		ctrEnd := ctr + conflictLines
		for ; ctr < ctrEnd; ctr++ {
			//libsolv.src < 0.6.36-2.27.19.8
			//libsolv-tools.x86_64 < 0.6.36-2.27.19.8
			//libzypp.src < 16.20.2-27.60.4
			//libzypp.x86_64 < 16.20.2-27.60.4
			//perl-solv.x86_64 < 0.6.36-2.27.19.8
			//python-solv.x86_64 < 0.6.36-2.27.19.8
			//zypper.src < 1.13.54-18.40.2
			//zypper.x86_64 < 1.13.54-18.40.2
			//zypper-log.noarch < 1.13.54-18.40.2
			parts := strings.Split(string(lines[ctr]), "<")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid package info")
			}
			nameArch := strings.Split(parts[0], ".")
			if len(nameArch) != 2 {
				return nil, fmt.Errorf("invalid package info")
			}

			pkgName := strings.Trim(nameArch[0], " ")
			patches, ok := patchInfo[pkgName]
			if !ok {
				patches = make([]string, 0)
			}
			patches = append(patches, patchName)
			patchInfo[pkgName] = patches
		}

		// set i for next patch information blob
		i = ctrEnd
	}
	// TODO: instead of returning a map of <string, []string>
	// make it more concrete type returns with more information
	// about the package and patch
	if len(patchInfo) == 0 {
		return nil, fmt.Errorf("invalid patch information, did not find patch blobs")
	}
	return patchInfo, nil
}

// ZypperPackagesInPatch returns the list of patches, a package upgrade belongs to
func ZypperPackagesInPatch(ctx context.Context, patches []*ZypperPatch) (map[string][]string, error) {
	var patchNames []string
	for _, patch := range patches {
		patchNames = append(patchNames, patch.Name)
	}
	out, err := zypperPatchInfo(ctx, patchNames)
	if err != nil {
		return nil, err
	}
	return parseZypperPatchInfo(out)
}
