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

package policies

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

func googetRepositories(ctx context.Context, repos []*agentendpointpb.GooRepository, repoFile string) error {
	/*
		# Repo file managed by Google OSConfig agent

		- name: repo1-name
		  url: https://repo1-url
		- name: repo1-name
		  url: https://repo2-url
	*/
	var buf bytes.Buffer
	buf.WriteString("# Repo file managed by Google OSConfig agent\n")
	for _, repo := range repos {
		buf.WriteString(fmt.Sprintf("\n- name: %s\n", repo.Name))
		buf.WriteString(fmt.Sprintf("  url: %s\n", repo.Url))
	}

	return writeIfChanged(ctx, buf.Bytes(), repoFile)
}

func googetChanges(ctx context.Context, gooInstalled, gooRemoved, gooUpdated []*agentendpointpb.Package) error {
	var err error
	var errs []string

	var installed []packages.PkgInfo
	if len(gooInstalled) > 0 || len(gooUpdated) > 0 || len(gooRemoved) > 0 {
		installed, err = packages.InstalledGooGetPackages(ctx)
		if err != nil {
			return err
		}
	}

	var updates []packages.PkgInfo
	if len(gooUpdated) > 0 {
		updates, err = packages.GooGetUpdates(ctx)
		if err != nil {
			return err
		}
	}

	changes := getNecessaryChanges(installed, updates, gooInstalled, gooRemoved, gooUpdated)

	if changes.packagesToInstall != nil {
		clog.Infof(ctx, "Installing packages %s", changes.packagesToInstall)
		if err := packages.InstallGooGetPackages(ctx, changes.packagesToInstall); err != nil {
			errs = append(errs, fmt.Sprintf("error installing googet packages: %v", err))
		}
	}

	if changes.packagesToUpgrade != nil {
		clog.Infof(ctx, "Upgrading packages %s", changes.packagesToUpgrade)
		if err := packages.InstallGooGetPackages(ctx, changes.packagesToUpgrade); err != nil {
			errs = append(errs, fmt.Sprintf("error upgrading googet packages: %v", err))
		}
	}

	if changes.packagesToRemove != nil {
		clog.Infof(ctx, "Removing packages %s", changes.packagesToRemove)
		if err := packages.RemoveGooGetPackages(ctx, changes.packagesToRemove); err != nil {
			errs = append(errs, fmt.Sprintf("error removing googet packages: %v", err))
		}
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}
