//  Copyright 2021 Google Inc. All Rights Reserved.
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

package packages

import (
	"context"
	"strconv"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"golang.org/x/sys/windows/registry"
)

func parseDate(dateString string) time.Time {
	if len(dateString) != 8 {
		return time.Time{}
	}

	year, err := strconv.ParseInt(dateString[0:4], 10, 32)
	if err != nil {
		return time.Time{}
	}
	month, err := strconv.ParseInt(dateString[4:6], 10, 32)
	if err != nil {
		return time.Time{}
	}
	day, err := strconv.ParseInt(dateString[6:8], 10, 32)
	if err != nil {
		return time.Time{}
	}

	return time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.Now().Location())
}

func getWindowsApplication(ctx context.Context, k *registry.Key) *WindowsApplication {
	displayName, _, errName := k.GetStringValue("DisplayName")
	_, _, errUninstall := k.GetStringValue("UninstallString")

	if errName == nil && errUninstall == nil {
		displayVersion, _, _ := k.GetStringValue("DisplayVersion")
		publisher, _, _ := k.GetStringValue("Publisher")
		installDate, _, _ := k.GetStringValue("InstallDate")
		helpLink, _, _ := k.GetStringValue("HelpLink")
		return &WindowsApplication{
			DisplayName:    displayName,
			DisplayVersion: displayVersion,
			Publisher:      publisher,
			InstallDate:    parseDate(installDate),
			HelpLink:       helpLink,
		}
	}
	return nil
}

func GetWindowsApplications(ctx context.Context) ([]*WindowsApplication, error) {
	directories := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	var allApps []*WindowsApplication

	for _, dir := range directories {
		clog.Debugf(ctx, "Loading windows applications from: %v", dir)
		apps, err := getWindowsApplications(ctx, dir)
		if err != nil {
			clog.Errorf(ctx, "error loading windows applications from registry: %v, error: %v", dir, err)
			continue
		}
		allApps = append(allApps, apps...)
	}
	return allApps, nil
}

func getWindowsApplications(ctx context.Context, directory string) ([]*WindowsApplication, error) {
	dirKey, err := registry.OpenKey(registry.LOCAL_MACHINE, directory, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer dirKey.Close()

	var result []*WindowsApplication
	subkeys, err := dirKey.ReadSubKeyNames(0)
	if err != nil {
		return nil, err
	}
	for _, subkey := range subkeys {
		k, err := registry.OpenKey(dirKey, subkey, registry.QUERY_VALUE)
		if err != nil {
			clog.Debugf(ctx, "error when opening registry key: %v", err)
			continue
		}
		app := getWindowsApplication(ctx, &k)
		if app != nil {
			result = append(result, app)
		}
		k.Close()
	}
	return result, nil
}
