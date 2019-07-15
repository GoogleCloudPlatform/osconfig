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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

const (
	systemctl = "/bin/systemctl"
	reboot    = "/bin/reboot"
	shutdown  = "/bin/shutdown"
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

func (r *patchRun) systemRebootRequired() (bool, error) {
	switch {
	case packages.AptExists:
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
	case packages.YumExists:
		r.debugf("Checking if reboot required by querying /usr/bin/needs-restarting.")
		if e, _ := exists("/usr/bin/needs-restarting"); e {
			out, err := exec.Command("/usr/bin/needs-restarting", "-r").CombinedOutput()
			r.debugf("'/usr/bin/needs-restarting -r' output:\n%s", string(out))
			if err == nil {
				r.debugf("'/usr/bin/needs-restarting -r' exit code 0 indicating no reboot is required.")
				return false, nil
			}
			if eerr, ok := err.(*exec.ExitError); ok {
				switch eerr.ExitCode() {
				case 1:
					r.debugf("'/usr/bin/needs-restarting -r' exit code 1 indicating a reboot is required")
					return true, nil
				case 2:
					r.infof("/usr/bin/needs-restarting is too old, can't easily determine if reboot is required")
					return false, nil
				}
			}
			return false, err
		}
		r.infof("/usr/bin/needs-restarting does not exist, can't check if reboot is required. Try installing the 'yum-utils' package.")
		return false, nil
	case packages.ZypperExists:
		r.errorf("systemRebootRequired not implemented for zypper")
		return false, nil
	}
	// TODO: implement something like this for rpm based distros to fall back to:
	// https://bugzilla.redhat.com/attachment.cgi?id=1187437&action=diff

	return false, fmt.Errorf("no recognized package manager installed, can't determine if reboot is required")
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
