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
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	dpkg      string
	dpkgQuery string
	dpkgDeb   string
	aptGet    string

	dpkgInstallArgs   = []string{"--install"}
	dpkgQueryArgs     = []string{"-W", "-f", "${Package} ${Architecture} ${Version} ${db:Status-Status}\n"}
	dpkgRepairArgs    = []string{"--configure", "-a"}
	aptGetInstallArgs = []string{"install", "-y"}
	aptGetRemoveArgs  = []string{"remove", "-y"}
	aptGetUpdateArgs  = []string{"update"}

	aptGetUpgradeCmd     = "upgrade"
	aptGetFullUpgradeCmd = "full-upgrade"
	aptGetDistUpgradeCmd = "dist-upgrade"
	aptGetUpgradableArgs = []string{"--just-print", "-qq"}
	allowDowngradesArg   = "--allow-downgrades"

	dpkgErr = []byte("dpkg --configure -a")
)

func init() {
	if runtime.GOOS != "windows" {
		dpkg = "/usr/bin/dpkg"
		dpkgQuery = "/usr/bin/dpkg-query"
		dpkgDeb = "/usr/bin/dpkg-deb"
		aptGet = "/usr/bin/apt-get"
	}
	AptExists = util.Exists(aptGet)
	DpkgExists = util.Exists(dpkg)
	DpkgQueryExists = util.Exists(dpkgQuery)
}

// AptUpgradeType is the apt upgrade type.
type AptUpgradeType int

const (
	// AptGetUpgrade specifies apt-get upgrade should be run.
	AptGetUpgrade AptUpgradeType = iota
	// AptGetDistUpgrade specifies apt-get dist-upgrade should be run.
	AptGetDistUpgrade
	// AptGetFullUpgrade specifies apt-get full-upgrade should be run.
	AptGetFullUpgrade
)

type aptGetUpgradeOpts struct {
	upgradeType     AptUpgradeType
	showNew         bool
	allowDowngrades bool
}

// AptGetUpgradeOption is an option for apt-get upgrade.
type AptGetUpgradeOption func(*aptGetUpgradeOpts)

// AptGetUpgradeType returns a AptGetUpgradeOption that specifies upgrade type.
func AptGetUpgradeType(upgradeType AptUpgradeType) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.upgradeType = upgradeType
	}
}

// AptGetUpgradeShowNew returns a AptGetUpgradeOption that indicates whether 'new' packages should be returned.
func AptGetUpgradeShowNew(showNew bool) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.showNew = showNew
	}
}

// AptGetUpgradeAllowDowngrades returns a AptGetUpgradeOption that specifies AllowDowngrades.
func AptGetUpgradeAllowDowngrades(allowDowngrades bool) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.allowDowngrades = allowDowngrades
	}
}

func dpkgRepair(ctx context.Context, out []byte) bool {
	// Error code 100 may occur for non repairable errors, just check the output.
	if !bytes.Contains(out, dpkgErr) {
		return false
	}
	clog.Debugf(ctx, "apt-get error, attempting dpkg repair.")
	// Ignore error here, just log and rerun apt-get.
	run(ctx, dpkg, dpkgRepairArgs)

	return true
}

type cmdModifier func(*exec.Cmd)

func runAptGet(ctx context.Context, args []string, cmdModifiers []cmdModifier) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, aptGet, args...)
	for _, modifier := range cmdModifiers {
		modifier(cmd)
	}

	return runner.Run(ctx, cmd)
}

func runAptGetWithDowngradeRetrial(ctx context.Context, args []string, cmdModifiers []cmdModifier) ([]byte, []byte, error) {
	stdout, stderr, err := runAptGet(ctx, args, cmdModifiers)
	if err != nil {
		if strings.Contains(string(stderr), "E: Packages were downgraded and -y was used without --allow-downgrades.") {
			cmdModifiers = append(cmdModifiers, func(cmd *exec.Cmd) {
				cmd.Args = append(cmd.Args, allowDowngradesArg)
			})
			stdout, stderr, err = runAptGet(ctx, args, cmdModifiers)
		}
	}

	return stdout, stderr, err
}

func parseDpkgDeb(data []byte) (*PkgInfo, error) {
	/*
	   new Debian package, version 2.0.
	   size 6731954 bytes: control archive=2138 bytes.
	       498 bytes,    12 lines      control
	      3465 bytes,    31 lines      md5sums
	      2793 bytes,    65 lines   *  postinst             #!/bin/sh
	       938 bytes,    28 lines   *  postrm               #!/bin/sh
	       216 bytes,     7 lines   *  prerm                #!/bin/sh
	   Package: google-guest-agent
	   Version: 1:1dummy-g1
	   Architecture: amd64
	   Maintainer: Google Cloud Team <gc-team@google.com>
	   Installed-Size: 23279
	   Depends: init-system-helpers (>= 1.18~)
	   Conflicts: python-google-compute-engine, python3-google-compute-engine
	   Section: misc
	   Priority: optional
	   Description: Google Compute Engine Guest Agent
	    Contains the guest agent and metadata script runner binaries.
	   Git: https://github.com/GoogleCloudPlatform/guest-agent/tree/c3d526e650c4e45ae3258c07836fd72f85fd9fc8
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	info := &PkgInfo{}
	for _, ln := range lines {
		if info.Name != "" && info.Version != "" && info.Arch != "" {
			break
		}
		fields := bytes.Fields(ln)
		if len(fields) != 2 {
			continue
		}
		if bytes.Contains(fields[0], []byte("Package:")) {
			// Some packages do not adhere to the Debian Policy and might have mix-cased names
			// And dpkg will register the package with lower case anyway so use lower-case package name
			// This is necessary because the compliance check is done between the .deb file descriptor value
			// and the internal dpkg db which register a lower-cased package name
			info.Name = strings.ToLower(string(fields[1]))
			continue
		}
		if bytes.Contains(fields[0], []byte("Version:")) {
			info.Version = string(fields[1])
			continue
		}
		if bytes.Contains(fields[0], []byte("Architecture:")) {
			info.Arch = osinfo.Architecture(string(fields[1]))
			continue
		}
	}
	if info.Name == "" || info.Version == "" || info.Arch == "" {
		return nil, fmt.Errorf("could not parse dpkg-deb output: %q", data)
	}
	return info, nil
}

// DebPkgInfo gets PkgInfo from a deb package.
func DebPkgInfo(ctx context.Context, path string) (*PkgInfo, error) {
	out, err := run(ctx, dpkgDeb, []string{"-I", path})
	if err != nil {
		return nil, err
	}

	return parseDpkgDeb(out)
}

// InstallAptPackages installs apt packages.
func InstallAptPackages(ctx context.Context, pkgs []string) error {
	args := append(aptGetInstallArgs, pkgs...)
	cmdModifiers := []cmdModifier{
		func(cmd *exec.Cmd) {
			cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		},
	}
	stdout, stderr, err := runAptGetWithDowngradeRetrial(ctx, args, cmdModifiers)
	if err != nil {
		if dpkgRepair(ctx, stderr) {
			stdout, stderr, err = runAptGetWithDowngradeRetrial(ctx, args, cmdModifiers)
		}
	}
	if err != nil {
		err = fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", aptGet, args, err, stdout, stderr)
	}
	return err
}

// RemoveAptPackages removes apt packages.
func RemoveAptPackages(ctx context.Context, pkgs []string) error {
	args := append(aptGetRemoveArgs, pkgs...)
	cmdModifiers := []cmdModifier{
		func(cmd *exec.Cmd) {
			cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		},
	}
	stdout, stderr, err := runAptGet(ctx, args, cmdModifiers)
	if err != nil {
		if dpkgRepair(ctx, stderr) {
			stdout, stderr, err = runAptGet(ctx, args, cmdModifiers)
		}
	}
	if err != nil {
		err = fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", aptGet, args, err, stdout, stderr)
	}
	return err
}

func parseAptUpdates(ctx context.Context, data []byte, showNew bool) []*PkgInfo {
	/*
		Inst libldap-common [2.4.45+dfsg-1ubuntu1.2] (2.4.45+dfsg-1ubuntu1.3 Ubuntu:18.04/bionic-updates, Ubuntu:18.04/bionic-security [all])
		Inst firmware-linux-free (3.4 Debian:9.9/stable [all]) []
		Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		Inst linux-image-4.9.0-9-amd64 (4.9.168-1+deb9u2 Debian-Security:9/stable [amd64])
		Inst linux-image-amd64 [4.9+80+deb9u6] (4.9+80+deb9u7 Debian:9.9/stable [amd64])
		Conf firmware-linux-free (3.4 Debian:9.9/stable [all])
		Conf google-cloud-sdk (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		Conf linux-image-4.9.0-9-amd64 (4.9.168-1+deb9u2 Debian-Security:9/stable [amd64])
		Conf linux-image-amd64 (4.9+80+deb9u7 Debian:9.9/stable [amd64])
	*/

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	var pkgs []*PkgInfo
	for _, ln := range lines {
		pkg := bytes.Fields(ln)
		if len(pkg) < 5 || string(pkg[0]) != "Inst" {
			continue
		}
		// Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		pkg = pkg[1:] // ==> google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		if bytes.HasPrefix(pkg[1], []byte("[")) {
			pkg = append(pkg[:1], pkg[2:]...) // ==> google-cloud-sdk (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		} else if !showNew {
			// This is a newly installed package and not an upgrade, ignore if showNew is false.
			continue
		}
		// Drop trailing "[]" if they exist.
		if bytes.Contains(pkg[len(pkg)-1], []byte("[]")) {
			pkg = pkg[:len(pkg)-1]
		}
		if !bytes.HasPrefix(pkg[1], []byte("(")) || !bytes.HasSuffix(pkg[len(pkg)-1], []byte(")")) {
			continue
		}
		ver := bytes.Trim(pkg[1], "(")             // (246.0.0-0 => 246.0.0-0
		arch := bytes.Trim(pkg[len(pkg)-1], "[])") // [all]) => all
		pkgs = append(pkgs, &PkgInfo{Name: string(pkg[0]), Arch: osinfo.Architecture(string(arch)), Version: string(ver)})
	}
	return pkgs
}

// AptUpdates returns all the packages that will be installed when running
// apt-get [dist-|full-]upgrade.
func AptUpdates(ctx context.Context, opts ...AptGetUpgradeOption) ([]*PkgInfo, error) {
	aptOpts := &aptGetUpgradeOpts{
		upgradeType:     AptGetUpgrade,
		showNew:         false,
		allowDowngrades: false,
	}

	for _, opt := range opts {
		opt(aptOpts)
	}

	args := aptGetUpgradableArgs
	switch aptOpts.upgradeType {
	case AptGetUpgrade:
		args = append(aptGetUpgradableArgs, aptGetUpgradeCmd)
	case AptGetDistUpgrade:
		args = append(aptGetUpgradableArgs, aptGetDistUpgradeCmd)
	case AptGetFullUpgrade:
		args = append(aptGetUpgradableArgs, aptGetFullUpgradeCmd)
	default:
		return nil, fmt.Errorf("unknown upgrade type: %q", aptOpts.upgradeType)
	}

	if _, err := AptUpdate(ctx); err != nil {
		return nil, err
	}

	out, _, err := runAptGetWithDowngradeRetrial(ctx, args, []cmdModifier{
		func(cmd *exec.Cmd) {
			cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		},
	})
	if err != nil {
		return nil, err
	}

	return parseAptUpdates(ctx, out, aptOpts.showNew), nil
}

// AptUpdate runs apt-get update.
func AptUpdate(ctx context.Context) ([]byte, error) {
	stdout, _, err := runAptGet(ctx, aptGetUpdateArgs, []cmdModifier{
		func(cmd *exec.Cmd) {
			cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		},
	})
	return stdout, err
}

func parseInstalledDebpackages(data []byte) []*PkgInfo {
	/*
	   foo amd64 1.2.3-4 installed
	   bar noarch 1.2.3-4 installed
	   baz noarch 1.2.3-4 config-files
	   ...
	*/
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	var pkgs []*PkgInfo
	for _, ln := range lines {
		pkg := strings.Fields(ln)
		if len(pkg) != 4 {
			continue
		}

		// Only report on installed packages.
		if pkg[3] != "installed" {
			continue
		}

		pkgs = append(pkgs, &PkgInfo{Name: pkg[0], Arch: osinfo.Architecture(pkg[1]), Version: pkg[2]})
	}
	return pkgs
}

// InstalledDebPackages queries for all installed deb packages.
func InstalledDebPackages(ctx context.Context) ([]*PkgInfo, error) {
	out, err := run(ctx, dpkgQuery, dpkgQueryArgs)
	if err != nil {
		return nil, err
	}
	return parseInstalledDebpackages(out), nil
}

// DpkgInstall installs a deb package.
func DpkgInstall(ctx context.Context, path string) error {
	_, err := run(ctx, dpkg, append(dpkgInstallArgs, path))
	return err
}
