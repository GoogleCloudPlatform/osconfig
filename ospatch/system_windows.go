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
	"os/exec"
	"path/filepath"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"golang.org/x/sys/windows/registry"
)

// disableAutoUpdates disables system auto updates.
func disableAutoUpdates() {
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
		f := func() error {
			out, err := exec.Command(googet, "-noconfirm", "remove", "google-compute-engine-auto-updater").CombinedOutput()
			if err != nil {
				return fmt.Errorf("%v, out: %s", err, out)
			}
			return nil
		}
		if err := retry(1*time.Minute, "removing google-compute-engine-auto-updater package", logger.Debugf, f); err != nil {
			logger.Errorf("Error removing google-compute-engine-auto-updater: %v", msg, err)
		}
	}
}

func rebootSystem() error {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return exec.Command(filepath.Join(root, `System32\shutdown.exe`), "/r", "/t", "00", "/f", "/d", "p:2:3").Run()
}
