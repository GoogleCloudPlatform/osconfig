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
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"golang.org/x/sys/windows/registry"
)

// SystemRebootRequired checks whether a system reboot is required.
func SystemRebootRequired(ctx context.Context) (bool, error) {
	// https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw#remarks
	clog.Debugf(ctx, "Checking for PendingFileRenameOperations")
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager`, registry.QUERY_VALUE)
	if err == nil {
		val, _, err := k.GetStringsValue("PendingFileRenameOperations")
		if err == nil {
			k.Close()

			if len(val) > 0 {
				clog.Infof(ctx, "PendingFileRenameOperations indicate a reboot is required: %q", val)
				return true, nil
			}
		} else if err != registry.ErrNotExist {
			return false, err
		}
	} else if err != registry.ErrNotExist {
		return false, err
	}

	regKeys := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`,
		// Skip checking CBS for now until we implement rate limiting on reboots, this key
		// will not be reset in some instances for a few minutes after a reboot. This should
		// not prevent updates from running as this mainly indicates a feature install.
		// `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`,
	}
	for _, key := range regKeys {
		clog.Debugf(ctx, "Checking if reboot required by testing the existance of %s", key)
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
		if err == nil {
			k.Close()
			clog.Infof(ctx, "%s exists indicating a reboot is required.", key)
			return true, nil
		} else if err != registry.ErrNotExist {
			return false, err
		}
	}

	return false, nil
}

func checkFilters(ctx context.Context, updt *packages.IUpdate, kbExcludes, classFilter, exclusive_patches []string) (ok bool, err error) {
	title, err := updt.GetProperty("Title")
	if err != nil {
		return false, fmt.Errorf(`updt.GetProperty("Title"): %v`, err)
	}
	defer title.Clear()

	defer func() {
		if ok == true {
			clog.Debugf(ctx, "Update %q not excluded by any filters.", title.ToString())
		}
	}()

	kbArticleIDsRaw, err := updt.GetProperty("KBArticleIDs")
	if err != nil {
		return false, fmt.Errorf(`updt.GetProperty("KBArticleIDs"): %v`, err)
	}
	defer kbArticleIDsRaw.Clear()

	kbArticleIDs := kbArticleIDsRaw.ToIDispatch()
	defer kbArticleIDs.Release()

	kbArticleIDsCount, err := packages.GetCount(kbArticleIDs)
	if err != nil {
		return false, err
	}

	if len(exclusive_patches) > 0 {
		for i := 0; i < int(kbArticleIDsCount); i++ {
			kbRaw, err := kbArticleIDs.GetProperty("Item", i)
			if err != nil {
				return false, err
			}
			defer kbRaw.Clear()
			for _, e := range exclusive_patches {
				if e == kbRaw.ToString() {
					// until now we have only seen at most 1 kbarticles
					// in a patch update. So, if we get a match, we just
					// install the update
					return true, nil
				}
			}
		}
		// since there are exclusive_patches to be installed,
		// other fields like excludes, classfilter are void
		return false, nil
	}

	if len(kbExcludes) > 0 {
		for i := 0; i < int(kbArticleIDsCount); i++ {
			kbRaw, err := kbArticleIDs.GetProperty("Item", i)
			if err != nil {
				return false, err
			}
			defer kbRaw.Clear()
			for _, e := range kbExcludes {
				// kbArticleIDs is just the IDs, but users are used to using the KB prefix.
				if strings.TrimLeft(e, "KkBb") == kbRaw.ToString() {
					clog.Debugf(ctx, "Update %q (%s) matched exclude filter", title.ToString(), kbRaw.ToString())
					return false, nil
				}
			}
		}
	}

	if len(classFilter) == 0 {
		return true, nil
	}

	categoriesRaw, err := updt.GetProperty("Categories")
	if err != nil {
		return false, fmt.Errorf(`updt.GetProperty("Categories"): %v`, err)
	}
	defer categoriesRaw.Clear()

	categories := categoriesRaw.ToIDispatch()
	defer categories.Release()

	categoriesCount, err := packages.GetCount(categories)
	if err != nil {
		return false, err
	}

	for i := 0; i < int(categoriesCount); i++ {
		catRaw, err := categories.GetProperty("Item", i)
		if err != nil {
			return false, fmt.Errorf(`categories.GetProperty("Item", i): %v`, err)
		}
		defer catRaw.Clear()

		cat := catRaw.ToIDispatch()
		defer cat.Release()

		catIdRaw, err := cat.GetProperty("CategoryID")
		if err != nil {
			return false, fmt.Errorf(`cat.GetProperty("CategoryID"): %v`, err)
		}
		defer catIdRaw.Clear()

		for _, c := range classFilter {
			if c == catIdRaw.ToString() {
				return true, nil
			}
		}
	}

	clog.Debugf(ctx, "Update %q not found in classification filter", title.ToString())
	return false, nil
}

// GetWUAUpdates gets WUA updates based on optional classFilter and kbExcludes.
func GetWUAUpdates(ctx context.Context, session *packages.IUpdateSession, classFilter, kbExcludes, exclusivePatches []string) (*packages.IUpdateCollection, error) {
	// Search for all not installed updates but filter out ones that will be installed after a reboot.
	filter := "IsInstalled=0 AND RebootRequired=0"
	clog.Debugf(ctx, "Searching for WUA updates with query %q", filter)
	updts, err := session.GetWUAUpdateCollection(filter)
	if err != nil {
		return nil, fmt.Errorf("GetWUAUpdateCollection error: %v", err)
	}
	if len(classFilter) == 0 && len(kbExcludes) == 0 {
		return updts, nil
	}
	defer updts.Release()

	count, err := updts.Count()
	if err != nil {
		return nil, err
	}
	clog.Debugf(ctx, "Found %d total updates avaiable (pre filter).", count)

	newUpdts, err := packages.NewUpdateCollection()
	if err != nil {
		return nil, err
	}

	clog.Debugf(ctx, "Using filters: Excludes: %q, Classifications: %q, ExclusivePatches: %q", kbExcludes, classFilter, exclusivePatches)
	for i := 0; i < int(count); i++ {
		updt, err := updts.Item(i)
		if err != nil {
			return nil, err
		}

		ok, err := checkFilters(ctx, updt, kbExcludes, classFilter, exclusivePatches)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		if err := newUpdts.Add(updt); err != nil {
			return nil, err
		}
	}

	return newUpdts, nil
}
