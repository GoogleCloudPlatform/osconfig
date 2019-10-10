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
	"fmt"
	"os"
	"os/exec"
)

const aptGet = "/usr/bin/apt-get"

var (
	aptGetUpdateArgs      = []string{"update"}
	aptGetUpgradeArgs     = []string{"upgrade", "-y"}
	aptGetFullUpgradeArgs = []string{"full-upgrade", "-y"}
	aptGetDistUpgradeArgs = []string{"dist-upgrade", "-y"}
)

type aptGetUpgradeType int

const (
	aptGetUpgrade aptGetUpgradeType = iota
	// AptGetDistUpgrade specifies apt-get dist-upgrade should be run.
	AptGetDistUpgrade
	// AptGetFullUpgrade specifies apt-get full-upgrade should be run.
	AptGetFullUpgrade
)

type aptGetUpgradeOpts struct {
	upgradeType aptGetUpgradeType
	excludes    []string
	dryrun      bool

	runner func(cmd *exec.Cmd) ([]byte, error)
}

// AptGetUpgradeOption is an option for apt-get update.
type AptGetUpgradeOption func(*aptGetUpgradeOpts)

// AptGetUpgradeType returns a AptGetUpgradeOption that specifies upgrade type.
func AptGetUpgradeType(upgradeType aptGetUpgradeType) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.upgradeType = upgradeType
	}
}

// AptGetExcludes excludes these packages from upgrade.
func AptGetExcludes(excludes []string) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.excludes = excludes
	}
}

// AptGetDryRun performs a dry run.
func AptGetDryRun(dryrun bool) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.dryrun = dryrun
	}
}

// AptGetUpgradeRunner returns a AptGetUpgradeOption that specifies the runner.
func AptGetUpgradeRunner(runner func(cmd *exec.Cmd) ([]byte, error)) AptGetUpgradeOption {
	return func(args *aptGetUpgradeOpts) {
		args.runner = runner
	}
}

// RunAptGetUpgrade runs apt-get upgrade.
func RunAptGetUpgrade(opts ...AptGetUpgradeOption) error {
	aptOpts := &aptGetUpgradeOpts{
		upgradeType: aptGetUpgrade,
		runner:      defaultRunner,
	}

	for _, opt := range opts {
		opt(aptOpts)
	}

	if _, err := aptOpts.runner(exec.Command(aptGet, aptGetUpdateArgs...)); err != nil {
		return err
	}

	var args []string
	switch aptOpts.upgradeType {
	case aptGetUpgrade:
		args = aptGetUpgradeArgs
	case AptGetDistUpgrade:
		args = aptGetDistUpgradeArgs
	case AptGetFullUpgrade:
		args = aptGetFullUpgradeArgs
	default:
		return fmt.Errorf("unknown upgrade type: %q", aptOpts.upgradeType)
	}

	upgrade := exec.Command(aptGet, args...)
	upgrade.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
	)
	if _, err := aptOpts.runner(upgrade); err != nil {
		return err
	}

	return nil
}
