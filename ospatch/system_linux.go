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

//+build !test

package ospatch

import (
	"bytes"
	"os"
	"os/exec"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
)

const (
	systemctl = "/bin/systemctl"
)

// DisableAutoUpdates disables system auto updates.
func DisableAutoUpdates() {
	// yum-cron on el systems
	if _, err := os.Stat("/usr/lib/systemd/system/yum-cron.service"); err == nil {
		out, err := exec.Command(systemctl, "is-enabled", "yum-cron.service").CombinedOutput()
		if err != nil {
			if eerr, ok := err.(*exec.ExitError); ok {
				// Error code of 1 indicates disabled.
				if eerr.ExitCode() == 1 {
					return
				}
			}
			logger.Errorf("Error checking status of yum-cron, error: %v, out: %s", err, out)
		}

		logger.Debugf("Disabling yum-cron")
		out, err = exec.Command(systemctl, "stop", "yum-cron.service").CombinedOutput()
		if err != nil {
			logger.Errorf("Error stopping yum-cron, error: %v, out: %s", err, out)
		}
		out, err = exec.Command(systemctl, "disable", "yum-cron.service").CombinedOutput()
		if err != nil {
			logger.Errorf("Error disabling yum-cron, error: %v, out: %s", err, out)
		}
	} else if _, err := os.Stat("/usr/sbin/yum-cron"); err == nil {
		out, err := exec.Command("/sbin/chkconfig", "yum-cron").CombinedOutput()
		if err != nil {
			logger.Errorf("Error checking status of yum-cron, error: %v, out: %s", err, out)
		}
		if bytes.Contains(out, []byte("disabled")) {
			return
		}

		logger.Debugf("Disabling yum-cron")
		out, err = exec.Command("/sbin/chkconfig", "yum-cron", "off").CombinedOutput()
		if err != nil {
			logger.Errorf("Error disabling yum-cron, error: %v, out: %s", err, out)
		}
	}

	// dnf-automatic on el8 systems
	if _, err := os.Stat("/usr/lib/systemd/system/dnf-automatic.timer"); err == nil {
		out, err := exec.Command(systemctl, "list-timers", "dnf-automatic.timer").CombinedOutput()
		if err != nil {
			logger.Errorf("Error checking status of dnf-automatic, error: %v, out: %s", err, out)
		}
		if bytes.Contains(out, []byte("0 timers listed")) {
			return
		}

		logger.Debugf("Disabling dnf-automatic")
		out, err = exec.Command(systemctl, "stop", "dnf-automatic.timer").CombinedOutput()
		if err != nil {
			logger.Errorf("Error stopping dnf-automatic, error: %v, out: %s", err, out)
		}
		out, err = exec.Command(systemctl, "disable", "dnf-automatic.timer").CombinedOutput()
		if err != nil {
			logger.Errorf("Error disabling dnf-automatic, error: %v, out: %s", err, out)
		}
	}

	// apt unattended-upgrades
	// TODO: Removing the package is a bit overkill, look into just managing
	// the configs, this is probably best done by looking through
	// /etc/apt/apt.conf.d/ and setting APT::Periodic::Unattended-Upgrade to 0.
	if _, err := os.Stat("/usr/bin/unattended-upgrades"); err == nil {
		logger.Debugf("Removing unattended-upgrades package")
		if err := packages.RemoveAptPackages([]string{"unattended-upgrades"}); err != nil {
			logger.Errorf(err.Error())
		}
	}
}
