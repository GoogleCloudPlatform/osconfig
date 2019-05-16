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
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha1"
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

func systemRebootRequired() (bool, error) {
	switch {
	case packages.AptExists:
		logger.Debugf("Checking if reboot required by looking at /var/run/reboot-required.")
		rr, err := exists("/var/run/reboot-required")
		if err != nil {
			return false, err
		}
		if rr {
			logger.Debugf("/var/run/reboot-required exists indicating a reboot is required.")
			return true, nil
		}
		logger.Debugf("/var/run/reboot-required does not exist, indicating no reboot is required.")
		return false, nil
	case packages.YumExists:
		logger.Debugf("Checking if reboot required by querying /usr/bin/needs-restarting.")
		if e, _ := exists("/usr/bin/needs-restarting"); e {
			cmd := exec.Command("/usr/bin/needs-restarting", "-r")
			err := cmd.Run()
			if err == nil {
				logger.Debugf("'/usr/bin/needs-restarting -r' exit code 0 indicating no reboot is required.")
				return false, nil
			}
			if eerr, ok := err.(*exec.ExitError); ok {
				switch eerr.ExitCode() {
				case 1:
					logger.Debugf("'/usr/bin/needs-restarting -r' exit code 1 indicating a reboot is required")
					return true, nil
				case 2:
					logger.Infof("/usr/bin/needs-restarting is too old, can't easily determine if reboot is required")
					return false, nil
				}
			}
			return false, err
		}
		logger.Infof("/usr/bin/needs-restarting does not exist, can't check if reboot is required. Try installing the 'yum-utils' package.")
		return false, nil
	case packages.ZypperExists:
		logger.Errorf("systemRebootRequired not implemented for zypper")
		return false, nil
	}
	// TODO: implement something like this for rpm based distros to fall back to:
	// https://bugzilla.redhat.com/attachment.cgi?id=1187437&action=diff

	return false, fmt.Errorf("no recognized package manager installed, can't determine if reboot is required")
}

func updatePackages(r *patchRun) error {
	var errs []string
	if packages.AptExists {
		if r.Job.ReportPatchJobInstanceDetailsResponse.PatchConfig.Apt != nil {
			switch r.Job.ReportPatchJobInstanceDetailsResponse.PatchConfig.Apt.Type {
			case osconfigpb.AptSettings_TYPE_UNSPECIFIED:
				if err := packages.AptUpgrade(); err != nil {
					errs = append(errs, err.Error())
				}
			case osconfigpb.AptSettings_DIST:
				if err := packages.AptDistUpgrade(); err != nil {
					errs = append(errs, err.Error())
				}
			}
		} else {
			if err := packages.AptUpgrade(); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if packages.YumExists {
		if r.Job.ReportPatchJobInstanceDetailsResponse.PatchConfig.Yum != nil {
			opts := []packages.YumUpdateOption{
				packages.YumUpdateSecurity(r.Job.ReportPatchJobInstanceDetailsResponse.PatchConfig.Yum.GetSecurity()),
				packages.YumUpdateMinimal(r.Job.ReportPatchJobInstanceDetailsResponse.PatchConfig.Yum.GetMinimal()),
				packages.YumUpdateExcludes(r.Job.ReportPatchJobInstanceDetailsResponse.PatchConfig.Yum.GetExcludes()),
			}
			if err := packages.YumUpdate(opts...); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if packages.ZypperExists {
		if err := packages.ZypperUpdate(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}

func runUpdates(r *patchRun) error {
	if err := r.reportContinuingState(osconfigpb.Instance_APPLYING_PATCHES); err != nil {
		return err
	}

	logger.Debugf("Installing package updates.")
	return retry(3*time.Minute, "installing package updates", func() error { return updatePackages(r) })
}
