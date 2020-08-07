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
	dpkgquery string
	aptGet    string

	dpkgInstallArgs   = []string{"--install"}
	dpkgQueryArgs     = []string{"-W", "-f", "${Package} ${Architecture} ${Version}\n"}
	dpkgRepairArgs    = []string{"--configure", "-a"}
	aptGetInstallArgs = []string{"install", "-y"}
	aptGetRemoveArgs  = []string{"remove", "-y"}
	aptGetUpdateArgs  = []string{"update"}

	aptGetUpgradeCmd     = "upgrade"
	aptGetFullUpgradeCmd = "full-upgrade"
	aptGetDistUpgradeCmd = "dist-upgrade"
	aptGetUpgradableArgs = []string{"--just-print", "-qq"}
)

func init() {
	if runtime.GOOS != "windows" {
		dpkg = "/usr/bin/dpkg"
		dpkgquery = "/usr/bin/dpkg-query"
		aptGet = "/usr/bin/apt-get"
	}
	AptExists = util.Exists(aptGet)
	DpkgExists = util.Exists(dpkg)
	DpkgQueryExists = util.Exists(dpkgquery)
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
	upgradeType AptUpgradeType
	showNew     bool
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

func dpkgRepair(ctx context.Context, out []byte) bool {
	// Error code 100 may occur for non repairable errors, just check the output.
	if !bytes.Contains(out, []byte("dpkg --configure -a")) {
		return false
	}
	clog.Debugf(ctx, "apt-get error, attempting dpkg repair.")
	// Ignore error here, just log and rerun apt-get.
	out, _ = run(ctx, exec.Command(dpkg, dpkgRepairArgs...))
	clog.Debugf(ctx, "dpkg %q output:\n%s", dpkgRepairArgs, strings.ReplaceAll(string(out), "\n", "\n "))

	return true
}

// InstallAptPackages installs apt packages.
func InstallAptPackages(ctx context.Context, pkgs []string) error {
	args := append(aptGetInstallArgs, pkgs...)
	install := exec.Command(aptGet, args...)
	install.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)
	out, err := run(ctx, install)
	clog.Debugf(ctx, "apt-get %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		if dpkgRepair(ctx, out) {
			out, err = run(ctx, install)
			clog.Debugf(ctx, "apt-get %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
		}
	}

	if err != nil {
		err = fmt.Errorf("error running apt-get with args %q: %v, stdout: %s", args, err, out)
	}
	return err
}

// RemoveAptPackages removes apt packages.
func RemoveAptPackages(ctx context.Context, pkgs []string) error {
	args := append(aptGetRemoveArgs, pkgs...)
	remove := exec.Command(aptGet, args...)
	remove.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)
	out, err := run(ctx, remove)
	if err != nil {
		if dpkgRepair(ctx, out) {
			out, err = run(ctx, remove)
			clog.Debugf(ctx, "apt-get %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
		}
	}

	clog.Debugf(ctx, "apt-get %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		err = fmt.Errorf("error running apt-get with args %q: %v, stdout: %s", args, err, out)
	}
	return err
}

func parseAptUpdates(ctx context.Context, data []byte, showNew bool) []PkgInfo {
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

	var pkgs []PkgInfo
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
		pkgs = append(pkgs, PkgInfo{Name: string(pkg[0]), Arch: osinfo.Architecture(string(arch)), Version: string(ver)})
	}
	return pkgs
}

// AptUpdates returns all the packages that will be installed when running
// apt-get [dist-|full-]upgrade.
func AptUpdates(ctx context.Context, opts ...AptGetUpgradeOption) ([]PkgInfo, error) {
	aptOpts := &aptGetUpgradeOpts{
		upgradeType: AptGetUpgrade,
		showNew:     false,
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

	if out, err := run(ctx, exec.Command(aptGet, aptGetUpdateArgs...)); err != nil {
		return nil, fmt.Errorf("error running apt-get with args %q: %v, stdout: %s", aptGetUpdateArgs, err, out)
	}

	out, err := run(ctx, exec.Command(aptGet, args...))
	clog.Debugf(ctx, "apt-get %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		return nil, fmt.Errorf("error running apt-get with args %q: %v, stdout: %s", args, err, out)
	}

	return parseAptUpdates(ctx, out, aptOpts.showNew), nil
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
func InstalledDebPackages(ctx context.Context) ([]PkgInfo, error) {
	out, err := run(ctx, exec.Command(dpkgquery, dpkgQueryArgs...))
	clog.Debugf(ctx, "dpkgquery %q output:\n%s", dpkgQueryArgs, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		return nil, fmt.Errorf("error running dpkgquery with args %q: %v, stdout: %s", dpkgQueryArgs, err, out)
	}
	return parseInstalledDebpackages(out), nil
}

// DpkgInstall installs a deb package.
func DpkgInstall(ctx context.Context, path string) error {
	args := append(dpkgInstallArgs, path)
	out, err := run(ctx, exec.Command(dpkg, args...))
	clog.Debugf(ctx, "dpkg %q output:\n%s", args, strings.ReplaceAll(string(out), "\n", "\n "))
	if err != nil {
		err = fmt.Errorf("error running dpkg with args %q: %v, stdout: %s", args, err, out)
	}
	return err
}
