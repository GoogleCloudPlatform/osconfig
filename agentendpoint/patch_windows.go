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

// +build windows

package agentendpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

var classifications = map[agentendpointpb.WindowsUpdateSettings_Classification]string{
	agentendpointpb.WindowsUpdateSettings_CRITICAL:      "e6cf1350-c01b-414d-a61f-263d14d133b4",
	agentendpointpb.WindowsUpdateSettings_SECURITY:      "0fa1201d-4330-4fa8-8ae9-b877473b6441",
	agentendpointpb.WindowsUpdateSettings_DEFINITION:    "e0789628-ce08-4437-be74-2495b842f43b",
	agentendpointpb.WindowsUpdateSettings_DRIVER:        "ebfc1fc5-71a4-4f7b-9aca-3b9a503104a0",
	agentendpointpb.WindowsUpdateSettings_FEATURE_PACK:  "b54e7d24-7add-428f-8b75-90a396fa584f",
	agentendpointpb.WindowsUpdateSettings_SERVICE_PACK:  "68c5b0a3-d1a6-4553-ae49-01d3a7827828",
	agentendpointpb.WindowsUpdateSettings_TOOL:          "b4832bd8-e735-4761-8daf-37f882276dab",
	agentendpointpb.WindowsUpdateSettings_UPDATE_ROLLUP: "28bc880e-0592-4cbf-8f95-c79b17911d5f",
	agentendpointpb.WindowsUpdateSettings_UPDATE:        "cd5ffd1e-e932-4e3a-bf74-18bf0b1bbd83",
}

func classFilter(cs []agentendpointpb.WindowsUpdateSettings_Classification) ([]string, error) {
	var cf []string
	for _, c := range cs {
		sc, ok := classifications[c]
		if !ok {
			return nil, fmt.Errorf("Unknown classification: %s", c)
		}
		cf = append(cf, sc)
	}

	return cf, nil
}

func (r *patchTask) installWUAUpdates(ctx context.Context, cf []string) (int32, error) {
	r.infof("Searching for available Windows updates.")
	session, err := packages.NewUpdateSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()

	updts, err := ospatch.GetWUAUpdates(session, cf, r.Task.GetPatchConfig().GetWindowsUpdate().GetExcludes(), r.Task.GetPatchConfig().GetWindowsUpdate().GetExclusivePatches())
	if err != nil {
		return 0, err
	}

	count, err := updts.Count()
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	r.infof("%d Windows updates to install", count)

	for i := int32(0); i < count; i++ {
		if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
			return i, err
		}
		updt, err := updts.Item(int(i))
		if err != nil {
			return i, err
		}
		defer updt.Release()

		if err := session.InstallWUAUpdate(updt); err != nil {
			return i, fmt.Errorf(`installUpdate(class, excludes, updt): %v`, err)
		}
	}

	return count, nil
}

func (r *patchTask) wuaUpdates(ctx context.Context) error {
	cf, err := classFilter(r.Task.GetPatchConfig().GetWindowsUpdate().GetClassifications())
	if err != nil {
		return err
	}

	// We keep searching for and installing updates until the count == 0 or there is an error.
	retries := 20
	for i := 0; i < retries; i++ {
		count, err := r.installWUAUpdates(ctx, cf)
		if err != nil {
			return err
		}
		if count == 0 {
			r.infof("No Windows updates available to install")
			return nil
		}
	}

	return fmt.Errorf("failed to install all updates after trying %d times", retries)
}

func (r *patchTask) runUpdates(ctx context.Context) error {
	if err := retryFunc(30*time.Minute, "installing Windows updates", func() error { return r.wuaUpdates(ctx) }); err != nil {
		return err
	}

	if packages.GooGetExists {
		if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
			return err
		}

		r.debugf("Installing GooGet package updates.")
		opts := []ospatch.GooGetUpdateOption{}
		if err := retryFunc(3*time.Minute, "installing GooGet package updates", func() error { return ospatch.RunGooGetUpdate(opts...) }); err != nil {
			return err
		}
	}

	return nil
}
