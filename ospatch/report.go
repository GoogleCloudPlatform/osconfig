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

package ospatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
)

// repLabels holds the labels by which the log entries for the patch report
// are retrieved
var repLabels = map[string]string{"package-report": "true"}

// opsToReport represents packages and patches that are to be logged
// as part of patch reporting
type opsToReport struct {
	packages []*packages.PkgInfo
	patches  []*packages.ZypperPatch
}

func formatPatches(patches []*packages.ZypperPatch) string {
	names := []string{}
	for _, p := range patches {
		names = append(names, p.Name)
	}
	return strings.Join(names, ", ")
}

// logPackages logs the intent to patch the packages in pkgs
// for the purpose of patch report.
func logOps(ctx context.Context, ops opsToReport) {
	msg := ""
	sep := ""
	if len(ops.packages) > 0 {
		msg = fmt.Sprintf("Updating %d packages: %q", len(ops.packages), ops.packages)
		sep = "; "
	}
	if len(ops.patches) > 0 {
		msg = msg + fmt.Sprintf("%sInstalling %d patches: %s", sep, len(ops.patches), formatPatches(ops.patches))
	}

	clog.Infof(clog.WithLabels(ctx, repLabels), msg)
}

// logSuccess logs the success of patching the packages in pkgs
// for the purpose of patch report.
func logSuccess(ctx context.Context, ops opsToReport) {
	sep := ""
	msg := ""
	pkgs, patches := ops.packages, ops.patches
	if len(pkgs) > 0 {
		msg = fmt.Sprintf("Updated %d packages: %q", len(pkgs), pkgs)
		sep = "; "
	}
	if len(patches) > 0 {
		msg = msg + fmt.Sprintf("%sApplied %d patches: %s", sep, len(patches), formatPatches(patches))
	}
	msg = fmt.Sprintf("Success. %s", msg)
	clog.Infof(clog.WithLabels(ctx, repLabels), msg)
}

// logFailure logs the failure of patching the packages in pkgs caused by err,
// for the purpose of patch report.
func logFailure(ctx context.Context, ops opsToReport, err error) {
	sep := ""
	msg := ""
	pkgs, patches := ops.packages, ops.patches
	if len(pkgs) > 0 {
		msg = fmt.Sprintf("Tried to update %d packages: %q", len(pkgs), pkgs)
		sep = "; "
	}
	if len(patches) > 0 {
		msg = msg + fmt.Sprintf("%sTried to apply %d patches: %s", sep, len(patches), formatPatches(patches))
	}
	msg = fmt.Sprintf("Failure: %s. Error: %v", msg, err)
	clog.Infof(clog.WithLabels(ctx, repLabels), msg)
}
