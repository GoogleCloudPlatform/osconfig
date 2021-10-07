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

//go:build !test
// +build !test

package ospatch

import (
	"context"
	"errors"
	"io/ioutil"
	"os"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

// SystemRebootRequired checks whether a system reboot is required.
func SystemRebootRequired(ctx context.Context) (bool, error) {
	if packages.AptExists {
		clog.Debugf(ctx, "Checking if reboot required by looking at /var/run/reboot-required.")
		data, err := ioutil.ReadFile("/var/run/reboot-required")
		if os.IsNotExist(err) {
			clog.Debugf(ctx, "/var/run/reboot-required does not exist, indicating no reboot is required.")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		clog.Debugf(ctx, "/var/run/reboot-required exists indicating a reboot is required, content:\n%s", string(data))
		return true, nil
	}
	if ok := util.Exists(rpmquery); ok {
		clog.Debugf(ctx, "Checking if reboot required by querying rpm database.")
		return rpmReboot()
	}

	return false, errors.New("no recognized package manager installed, can't determine if reboot is required")
}

// InstallWUAUpdates is the linux stub for InstallWUAUpdates.
func InstallWUAUpdates(ctx context.Context) error {
	return nil
}
