/*
Copyright 2017 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package packages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/util"
	ole "github.com/go-ole/go-ole"
)

func coInitializeEx() error {
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		e, ok := err.(*ole.OleError)
		// S_OK and S_FALSE are both are Success codes.
		// https://docs.microsoft.com/en-us/windows/win32/learnwin32/error-handling-in-com
		if !ok || (e.Code() != S_OK && e.Code() != S_FALSE) {
			return fmt.Errorf(`ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED): %v`, err)
		}
	}
	return nil
}

// In order to work around memory issues with the WUA library we spawn a
// new process for these inventory queries.
func wuaUpdates(ctx context.Context, query string) ([]WUAPackage, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	var wua []WUAPackage
	stdout, stderr, err := runner.Run(ctx, exec.Command(exe, "wuaupdates", query))
	if err != nil {
		return nil, fmt.Errorf("error running agent to query for WUA updates, err: %v, stderr: %q ", err, stderr)
	}
	if err := json.Unmarshal(stdout, &wua); err != nil {
		return nil, err
	}

	return wua, nil
}

// GetPackageUpdates gets available package updates GooGet as well as any
// available updates from Windows Update Agent.
func GetPackageUpdates(ctx context.Context) (*Packages, error) {
	var pkgs Packages
	var errs []string

	if GooGetExists {
		if googet, err := GooGetUpdates(ctx); err != nil {
			msg := fmt.Sprintf("error listing googet updates: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			pkgs.GooGet = googet
		}
	}

	clog.Debugf(ctx, "Searching for available WUA updates.")

	if wua, err := wuaUpdates(ctx, "IsInstalled=0"); err != nil {
		msg := fmt.Sprintf("error listing installed Windows updates: %v", err)
		clog.Debugf(ctx, "Error: %s", msg)
		errs = append(errs, msg)
	} else {
		pkgs.WUA = wua
	}

	var err error
	if len(errs) != 0 {
		err = errors.New(strings.Join(errs, "\n"))
	}
	return &pkgs, err
}

// GetInstalledPackages gets all installed GooGet packages and Windows updates.
// Windows updates are read from Windows Update Agent and Win32_QuickFixEngineering.
func GetInstalledPackages(ctx context.Context) (*Packages, error) {
	var pkgs Packages
	var errs []string

	if util.Exists(googet) {
		if googet, err := InstalledGooGetPackages(ctx); err != nil {
			msg := fmt.Sprintf("error listing installed googet packages: %v", err)
			clog.Debugf(ctx, "Error: %s", msg)
			errs = append(errs, msg)
		} else {
			pkgs.GooGet = googet
		}
	}

	clog.Debugf(ctx, "Searching for installed WUA updates.")

	if wua, err := wuaUpdates(ctx, "IsInstalled=1"); err != nil {
		msg := fmt.Sprintf("error listing installed Windows updates: %v", err)
		clog.Debugf(ctx, "Error: %s", msg)
		errs = append(errs, msg)
	} else {
		pkgs.WUA = wua
	}

	if qfe, err := QuickFixEngineering(ctx); err != nil {
		msg := fmt.Sprintf("error listing installed QuickFixEngineering updates: %v", err)
		clog.Debugf(ctx, "Error: %s", msg)
		errs = append(errs, msg)
	} else {
		pkgs.QFE = qfe
	}

	var err error
	if len(errs) != 0 {
		err = errors.New(strings.Join(errs, "\n"))
	}
	return &pkgs, err
}
