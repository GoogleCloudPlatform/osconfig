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
	"os"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"golang.org/x/sys/windows/registry"
)

// DisableAutoUpdates disables system auto updates.
func DisableAutoUpdates() {
	k, openedExisting, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, registry.ALL_ACCESS)
	if err != nil {
		logger.Errorf("error disabling Windows auto updates, error: %v", err)
	}
	defer k.Close()

	if openedExisting {
		val, _, err := k.GetIntegerValue("NoAutoUpdate")
		if err == nil && val == 1 {
			return
		}
	}
	logger.Debugf("Disabling Windows Auto Updates")

	if err := k.SetDWordValue("NoAutoUpdate", 1); err != nil {
		logger.Errorf("error disabling Windows auto updates, error: %v", err)
	}

	if _, err := os.Stat(`C:\Program Files\Google\Compute Engine\tools\auto_updater.ps1`); err == nil {
		logger.Debugf("Removing google-compute-engine-auto-updater package")
		if err := packages.RemoveGooGetPackages([]string{"google-compute-engine-auto-updater"}); err != nil {
			logger.Errorf(err.Error())
		}
	}
}
