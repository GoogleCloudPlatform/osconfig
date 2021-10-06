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

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
)

// repLabels holds the labels by which the log entries for the patch report
// are retrieved
var repLabels = map[string]string{"package-report": "true"}

// logPackages logs the intent to patch the packages in pkgs
// for the purpose of patch report.
func logPackages(ctx context.Context, pkgs []*packages.PkgInfo) {
	msg := fmt.Sprintf("Updating %d packages: %q", len(pkgs), pkgs)
	clog.Infof(clog.WithLabels(ctx, repLabels), msg)
}

// logSuccess logs the success of patching the packages in pkgs
// for the purpose of patch report.
func logSuccess(ctx context.Context, pkgs []*packages.PkgInfo) {
	msg := fmt.Sprintf("Success. Updated %d packages: %q", len(pkgs), pkgs)
	clog.Infof(clog.WithLabels(ctx, repLabels), msg)
}

// logFailure logs the failure of patching the packages in pkgs caused by err,
// for the purpose of patch report.
func logFailure(ctx context.Context, pkgs []*packages.PkgInfo, err error) {
	var pkgNames []string
	msg := fmt.Sprintf("Failure updating %d packages: %q: %v", len(pkgNames), pkgNames, err)
	clog.Infof(clog.WithLabels(ctx, repLabels), msg)
}
