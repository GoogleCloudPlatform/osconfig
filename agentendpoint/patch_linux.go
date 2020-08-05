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

package agentendpoint

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

func (r *patchTask) runUpdates(ctx context.Context) error {
	var errs []string
	const retryPeriod = 3 * time.Minute
	// Check for both apt-get and dpkg-query to give us a clean signal.
	if packages.AptExists && packages.DpkgQueryExists {
		opts := []ospatch.AptGetUpgradeOption{
			ospatch.AptGetDryRun(r.Task.GetDryRun()),
			ospatch.AptGetExcludes(r.Task.GetPatchConfig().GetApt().GetExcludes()),
			ospatch.AptGetExclusivePackages(r.Task.GetPatchConfig().GetApt().GetExclusivePackages()),
		}
		switch r.Task.GetPatchConfig().GetApt().GetType() {
		case agentendpointpb.AptSettings_DIST:
			opts = append(opts, ospatch.AptGetUpgradeType(packages.AptGetDistUpgrade))
		}
		clog.Debugf(ctx, "Installing APT package updates.")
		if err := retryutil.RetryFunc(ctx, retryPeriod, "installing APT package updates", func() error { return ospatch.RunAptGetUpgrade(ctx, opts...) }); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if packages.YumExists && packages.RPMQueryExists {
		opts := []ospatch.YumUpdateOption{
			ospatch.YumUpdateSecurity(r.Task.GetPatchConfig().GetYum().GetSecurity()),
			ospatch.YumUpdateMinimal(r.Task.GetPatchConfig().GetYum().GetMinimal()),
			ospatch.YumUpdateExcludes(r.Task.GetPatchConfig().GetYum().GetExcludes()),
			ospatch.YumExclusivePackages(r.Task.GetPatchConfig().GetYum().GetExclusivePackages()),
			ospatch.YumDryRun(r.Task.GetDryRun()),
		}
		clog.Debugf(ctx, "Installing YUM package updates.")
		if err := retryutil.RetryFunc(ctx, retryPeriod, "installing YUM package updates", func() error { return ospatch.RunYumUpdate(ctx, opts...) }); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if packages.ZypperExists && packages.RPMQueryExists {
		opts := []ospatch.ZypperPatchOption{
			ospatch.ZypperPatchCategories(r.Task.GetPatchConfig().GetZypper().GetCategories()),
			ospatch.ZypperPatchSeverities(r.Task.GetPatchConfig().GetZypper().GetSeverities()),
			ospatch.ZypperUpdateWithUpdate(r.Task.GetPatchConfig().GetZypper().GetWithUpdate()),
			ospatch.ZypperUpdateWithOptional(r.Task.GetPatchConfig().GetZypper().GetWithOptional()),
			ospatch.ZypperUpdateWithExcludes(r.Task.GetPatchConfig().GetZypper().GetExcludes()),
			ospatch.ZypperUpdateWithExclusivePatches(r.Task.GetPatchConfig().GetZypper().GetExclusivePatches()),
			ospatch.ZypperUpdateDryrun(r.Task.GetDryRun()),
		}
		clog.Debugf(ctx, "Installing Zypper updates.")
		if err := retryutil.RetryFunc(ctx, retryPeriod, "installing Zypper updates", func() error { return ospatch.RunZypperPatch(ctx, opts...) }); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}
