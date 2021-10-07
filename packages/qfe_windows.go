/*
Copyright 2019 Google Inc. All Rights Reserved.
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
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/StackExchange/wmi"
)

type win32QuickFixEngineering struct {
	Caption, Description, HotFixID, InstalledOn string
}

// QuickFixEngineering queries the wmi object win32_QuickFixEngineering for a list of installed updates.
func QuickFixEngineering(ctx context.Context) ([]*QFEPackage, error) {
	var updts []win32QuickFixEngineering
	query := "SELECT Caption, Description, HotFixID, InstalledOn FROM Win32_QuickFixEngineering"
	clog.Debugf(ctx, "Querying WMI for installed QuickFixEngineering updates, query=%q.", query)
	if err := wmi.Query(query, &updts); err != nil {
		return nil, fmt.Errorf("wmi.Query(%q) error: %v", query, err)
	}
	qfe := make([]*QFEPackage, len(updts))
	for i, update := range updts {
		qfe[i] = &QFEPackage{
			Caption:     update.Caption,
			Description: update.Description,
			HotFixID:    update.HotFixID,
			InstalledOn: update.InstalledOn,
		}
	}
	return qfe, nil
}
