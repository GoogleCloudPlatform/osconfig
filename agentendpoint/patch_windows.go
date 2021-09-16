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

//go:build windows
// +build windows

package agentendpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

func (r *patchTask) classFilter() ([]string, error) {
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

	var cf []string
	for _, c := range r.Task.GetPatchConfig().GetWindowsUpdate().GetClassifications() {
		sc, ok := classifications[c]
		if !ok {
			return nil, fmt.Errorf("Unknown classification: %s", c)
		}
		cf = append(cf, sc)
	}

	return cf, nil
}

func (r *patchTask) installWUAUpdates(ctx context.Context, cf []string) (int32, error) {
	clog.Infof(ctx, "Searching for available Windows updates.")
	session, err := packages.NewUpdateSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()

	updts, err := ospatch.GetWUAUpdates(ctx, session, cf, r.Task.GetPatchConfig().GetWindowsUpdate().GetExcludes(), r.Task.GetPatchConfig().GetWindowsUpdate().GetExclusivePatches())
	if err != nil {
		return 0, err
	}
	defer updts.Release()

	count, err := updts.Count()
	if err != nil {
		return 0, err
	}

	if count == 0 {
		clog.Infof(ctx, "No Windows updates available to install")
		return 0, nil
	}

	clog.Infof(ctx, "%d Windows updates to install", count)

	if r.Task.GetDryRun() {
		clog.Infof(ctx, "Running in dryrun mode, not updating.")
		return 0, nil
	}

	for i := int32(0); i < count; i++ {
		if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
			return i, err
		}
		updt, err := updts.Item(int(i))
		if err != nil {
			return i, err
		}
		defer updt.Release()

		if err := session.InstallWUAUpdate(ctx, updt); err != nil {
			return i, fmt.Errorf(`installUpdate(updt): %v`, err)
		}
	}

	return count, nil
}

func (r *patchTask) wuaUpdates(ctx context.Context) error {
	cf, err := r.classFilter()
	if err != nil {
		return err
	}

	// We keep searching for and installing updates until the count == 0,
	// we get a stop signal, or retries exceed 10.
	retries := 10
	for i := 1; i <= retries; i++ {
		if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
			return err
		}
		count, err := r.installWUAUpdates(ctx, cf)
		if err != nil {
			clog.Errorf(ctx, "Error installing Windows updates (attempt %d): %v", i, err)
			time.Sleep(60 * time.Second)
			continue
		}
		if count == 0 {
			return nil
		}
	}

	return fmt.Errorf("failed to install all updates after trying %d times", retries)
}

func (r *patchTask) runUpdates(ctx context.Context) error {
	// Install GooGet updates first as this will allow us to update the agent prior to any potential WUA bugs/errors.
	if packages.GooGetExists {
		if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
			return err
		}

		clog.Debugf(ctx, "Installing GooGet package updates.")
		opts := []ospatch.GooGetUpdateOption{
			ospatch.GooGetDryRun(r.Task.GetDryRun()),
		}
		if err := retryutil.RetryFunc(ctx, 3*time.Minute, "installing GooGet package updates", func() error { return ospatch.RunGooGetUpdate(ctx, opts...) }); err != nil {
			return err
		}
	}

	// Don't use retry function as wuaUpdates handles it's own retries.
	if err := r.wuaUpdates(ctx); err != nil {
		return err
	}

	return nil
}
