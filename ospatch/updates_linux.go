//  Copyright 2018 Google Inc. All Rights Reserved.
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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

const (
	systemctl = "/bin/systemctl"
	reboot    = "/bin/reboot"
	shutdown  = "/bin/shutdown"
	rpm       = "/usr/bin/rpm"
)

func exists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func getBtime() (int, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("error opening /proc/stat: %v", err)
	}
	defer f.Close()

	var btime int
	scnr := bufio.NewScanner(f)
	for scnr.Scan() {
		if bytes.HasPrefix(scnr.Bytes(), []byte("btime")) {
			split := bytes.SplitN(scnr.Bytes(), []byte(" "), 2)
			if len(split) != 2 {
				return 0, fmt.Errorf("error parsing btime from /proc/stat: %q", scnr.Text())
			}
			btime, err = strconv.Atoi(string(bytes.TrimSpace(split[1])))
			if err != nil {
				return 0, fmt.Errorf("error parsing btime: %v", err)
			}
			break
		}
	}
	if err := scnr.Err(); err != nil && btime == 0 {
		return 0, fmt.Errorf("error scanning /proc/stat: %v", err)
	}
	if btime == 0 {
		return 0, errors.New("could not find btime in /proc/stat")
	}

	return btime, nil
}

func rpmReboot() (bool, error) {
	provides := []string{
		// Common packages.
		"kernel", "glibc", "gnutls",
		// EL packages.
		"linux-firmware", "openssl-libs", "dbus",
		// Suse packages.
		"kernel-firmware", "libopenssl1_1", "libopenssl1_0_0", "dbus-1",
	}
	args := append([]string{"-q", "--queryformat", `"%{INSTALLTIME}\n"`, "--whatprovides"}, provides...)
	out, err := exec.Command(rpm, args...).Output()
	if err != nil {
		// We don't care about return codes as we know some of these packages won't be installed.
		if _, ok := err.(*exec.ExitError); !ok {
			return false, fmt.Errorf("error running %s: %v", rpm, err)
		}
	}

	btime, err := getBtime()
	if err != nil {
		return false, err
	}

	// Scanning this output is best effort, false negatives are much prefered
	// to false positives, and keeping this as simple as possible is
	// beneficial.
	scnr := bufio.NewScanner(bytes.NewReader(out))
	for scnr.Scan() {
		itime, err := strconv.Atoi(scnr.Text())
		if err != nil {
			continue
		}
		if itime > btime {
			return true, nil
		}
	}

	return false, nil
}

func (r *patchRun) systemRebootRequired() (bool, error) {
	if packages.AptExists {
		r.debugf("Checking if reboot required by looking at /var/run/reboot-required.")
		data, err := ioutil.ReadFile("/var/run/reboot-required")
		if os.IsNotExist(err) {
			r.debugf("/var/run/reboot-required does not exist, indicating no reboot is required.")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		r.debugf("/var/run/reboot-required exists indicating a reboot is required, content:\n%s", string(data))
		return true, nil
	}
	if ok, _ := exists(rpm); ok {
		r.debugf("Checking if reboot required by querying rpm database.")
		return rpmReboot()
	}

	return false, errors.New("no recognized package manager installed, can't determine if reboot is required")
}

func (r *patchRun) runUpdates() error {
	var errs []string
	const retryPeriod = 3 * time.Minute
	if packages.AptExists {
		opts := []AptGetUpgradeOption{AptGetUpgradeRunner(patchRunRunner(r))}
		switch r.Job.GetPatchConfig().GetApt().GetType() {
		case osconfigpb.AptSettings_DIST:
			opts = append(opts, AptGetUpgradeType(AptGetDistUpgrade))
		}
		r.debugf("Installing APT package updates.")
		if err := retry(retryPeriod, "installing APT package updates", r.debugf, func() error { return RunAptGetUpgrade(opts...) }); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if packages.YumExists {
		opts := []YumUpdateOption{
			YumUpdateRunner(patchRunRunner(r)),
			YumUpdateSecurity(r.Job.GetPatchConfig().GetYum().GetSecurity()),
			YumUpdateMinimal(r.Job.GetPatchConfig().GetYum().GetMinimal()),
			YumUpdateExcludes(r.Job.GetPatchConfig().GetYum().GetExcludes()),
		}
		r.debugf("Installing YUM package updates.")
		if err := retry(retryPeriod, "installing YUM package updates", r.debugf, func() error { return RunYumUpdate(opts...) }); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if packages.ZypperExists {
		opts := []ZypperUpdateOption{ZypperUpdateRunner(patchRunRunner(r))}
		r.debugf("Installing Zypper package updates.")
		if err := retry(retryPeriod, "installing Zypper package updates", r.debugf, func() error { return RunZypperUpdate(opts...) }); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}
