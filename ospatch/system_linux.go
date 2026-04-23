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

//go:build !test
// +build !test

package ospatch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	systemctlPath      = "/bin/systemctl"
	chkconfigPath      = "/sbin/chkconfig"
	yumCronServicePath = "/usr/lib/systemd/system/yum-cron.service"
	yumCronBinPath     = "/usr/sbin/yum-cron"
	dnfAutomaticPath   = "/usr/lib/systemd/system/dnf-automatic.timer"
	unattendedUpgPath  = "/usr/bin/unattended-upgrades"
)

// run executes a command and returns stdout.
var run = func(ctx context.Context, cmd string, args []string) ([]byte, error) {
	stdout, stderr, err := runner.Run(ctx, exec.CommandContext(ctx, cmd, args...))
	if err != nil {
		return nil, fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", cmd, args, err, stdout, stderr)
	}
	return stdout, nil
}

// DisableAutoUpdates disables system auto updates.
func DisableAutoUpdates(ctx context.Context) {
	if util.Exists(yumCronServicePath) {
		if err := disableYumCronSystemd(ctx); err != nil {
			clog.Errorf(ctx, "%v", err)
		}
	} else if util.Exists(yumCronBinPath) {
		if err := disableYumCronChkconfig(ctx); err != nil {
			clog.Errorf(ctx, "%v", err)
		}
	}

	if util.Exists(dnfAutomaticPath) {
		if err := disableDnfAutomatic(ctx); err != nil {
			clog.Errorf(ctx, "%v", err)
		}
	}

	if util.Exists(unattendedUpgPath) {
		if err := disableUnattendedUpgrades(ctx); err != nil {
			clog.Errorf(ctx, "%v", err)
		}
	}
}

// disableYumCronSystemd disables yum-cron via systemctl.
func disableYumCronSystemd(ctx context.Context) error {
	_, err := run(ctx, systemctlPath, []string{"is-enabled", "yum-cron.service"})
	if err != nil {
		var eerr *exec.ExitError
		if errors.As(err, &eerr) && eerr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("error checking status of yum-cron: %v", err)
	}

	clog.Debugf(ctx, "Disabling yum-cron")
	if _, err := run(ctx, systemctlPath, []string{"stop", "yum-cron.service"}); err != nil {
		return fmt.Errorf("error stopping yum-cron: %v", err)
	}
	if _, err := run(ctx, systemctlPath, []string{"disable", "yum-cron.service"}); err != nil {
		return fmt.Errorf("error disabling yum-cron: %v", err)
	}
	return nil
}

// disableYumCronChkconfig disables yum-cron via chkconfig.
func disableYumCronChkconfig(ctx context.Context) error {
	out, err := run(ctx, chkconfigPath, []string{"yum-cron"})
	if err != nil {
		return fmt.Errorf("error checking status of yum-cron: %v", err)
	}
	if bytes.Contains(out, []byte("disabled")) {
		return nil
	}

	clog.Debugf(ctx, "Disabling yum-cron")
	if _, err := run(ctx, chkconfigPath, []string{"yum-cron", "off"}); err != nil {
		return fmt.Errorf("error disabling yum-cron: %v", err)
	}
	return nil
}

// disableDnfAutomatic disables dnf-automatic timer.
func disableDnfAutomatic(ctx context.Context) error {
	out, err := run(ctx, systemctlPath, []string{"list-timers", "dnf-automatic.timer"})
	if err != nil {
		return fmt.Errorf("error checking status of dnf-automatic: %v", err)
	}
	if bytes.Contains(out, []byte("0 timers listed")) {
		return nil
	}

	clog.Debugf(ctx, "Disabling dnf-automatic")
	if _, err := run(ctx, systemctlPath, []string{"stop", "dnf-automatic.timer"}); err != nil {
		return fmt.Errorf("error stopping dnf-automatic: %v", err)
	}
	if _, err := run(ctx, systemctlPath, []string{"disable", "dnf-automatic.timer"}); err != nil {
		return fmt.Errorf("error disabling dnf-automatic: %v", err)
	}
	return nil
}

// disableUnattendedUpgrades removes the unattended-upgrades package.
func disableUnattendedUpgrades(ctx context.Context) error {
	clog.Debugf(ctx, "Removing unattended-upgrades package")
	return packages.RemoveAptPackages(ctx, []string{"unattended-upgrades"})
}
