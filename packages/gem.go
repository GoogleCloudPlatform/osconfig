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

package packages

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	gem string

	gemListArgs        = []string{"list", "--local"}
	gemOutdatedArgs    = []string{"outdated", "--local"}
	gemListTimeout     = 15 * time.Second
	gemOutdatedTimeout = 15 * time.Second
)

func init() {
	if runtime.GOOS != "windows" {
		gem = "/usr/bin/gem"
	}
	GemExists = util.Exists(gem)
}

// GemUpdates queries for all available gem updates.
func GemUpdates(ctx context.Context) ([]*PkgInfo, error) {
	stdout, err := runWithDeadline(ctx, gemOutdatedTimeout, gem, gemOutdatedArgs)
	if err != nil {
		return nil, err
	}
	/*
	   foo (1.2.8 < 1.3.2)
	   bar (1.0.0 < 1.1.2)
	   ...
	*/

	input := strings.TrimSpace(string(stdout))
	if input == "" {
		return nil, nil
	}

	lines := strings.Split(input, "\n")

	var pkgs []*PkgInfo
	for _, ln := range lines {
		pkg := strings.Fields(ln)
		if len(pkg) != 4 {
			clog.Debugf(ctx, "%q does not represent a gem update\n", ln)
			continue
		}
		ver := strings.Trim(pkg[3], ")")
		pkgs = append(pkgs, &PkgInfo{Name: pkg[0], Arch: noarch, Version: ver})
	}

	return pkgs, nil
}

// InstalledGemPackages queries for all installed gem packages.
func InstalledGemPackages(ctx context.Context) ([]*PkgInfo, error) {
	stdout, err := runWithDeadline(ctx, gemListTimeout, gem, gemListArgs)
	if err != nil {
		return nil, err
	}

	/*

	   *** LOCAL GEMS ***

	   foo (1.2.3, 1.2.4)
	   bar (1.2.3)
	   csv (default: 3.1.2)
	   csv (default: 3.1.2, 1.5.8)
	   ...
	*/
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")

	redundantLinesCount := 3
	if len(lines) < redundantLinesCount {
		clog.Debugf(ctx, "No gems installed.")
		return nil, nil
	}

	redundantLinesEndPosition := redundantLinesCount - 1
	gems := lines[redundantLinesEndPosition:]

	var pkgs []*PkgInfo
	for _, ln := range gems {
		gems := parseGemEntry(ln)

		pkgs = append(pkgs, gems...)
	}

	return pkgs, nil
}

func parseGemEntry(entry string) []*PkgInfo {
	versionStarter := strings.Index(entry, "(")

	name := strings.TrimSpace(entry[:versionStarter])
	versions := strings.Split(strings.Trim(entry[versionStarter:], "()"), ",")

	var pkgs []*PkgInfo
	for _, ver := range versions {
		ver = strings.TrimSpace(ver)
		ver = strings.TrimPrefix(ver, "default: ")

		pkgs = append(pkgs, &PkgInfo{Name: name, Arch: noarch, Version: ver})
	}

	return pkgs
}
