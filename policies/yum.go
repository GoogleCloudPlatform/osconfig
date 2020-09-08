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

func yumRepositories(ctx context.Context, repos []*agentendpointpb.YumRepository, repoFile string) error {
	// TODO: Would it be easier to just use templates?
	/*
		# Repo file managed by Google OSConfig agent
		[repo1]
		name=repo1-name
		baseurl=https://repo1-url
		enabled=1
		gpgcheck=1
		repo_gpgcheck=1
		gpgkey=http://repo1-url/gpg
		[repo2]
		display_name=repo2-name
		baseurl=https://repo2-url
		enabled=1
		gpgcheck=1
		repo_gpgcheck=1
	*/
	var buf bytes.Buffer
	buf.WriteString("# Repo file managed by Google OSConfig agent\n")
	for _, repo := range repos {
		buf.WriteString(fmt.Sprintf("\n[%s]\n", repo.Id))
		if repo.DisplayName == "" {
			buf.WriteString(fmt.Sprintf("name=%s\n", repo.Id))
		} else {
			buf.WriteString(fmt.Sprintf("name=%s\n", repo.DisplayName))
		}
		buf.WriteString(fmt.Sprintf("baseurl=%s\n", repo.BaseUrl))
		buf.WriteString("enabled=1\ngpgcheck=1\nrepo_gpgcheck=1\n")
		if len(repo.GpgKeys) > 0 {
			buf.WriteString(fmt.Sprintf("gpgkey=%s\n", repo.GpgKeys[0]))
			for _, k := range repo.GpgKeys[1:] {
				buf.WriteString(fmt.Sprintf("       %s\n", k))
			}
		}
	}

	return writeIfChanged(ctx, buf.Bytes(), repoFile)
}

func yumChanges(ctx context.Context, yumInstalled, yumRemoved, yumUpdated []*agentendpointpb.Package) error {
	var err error
	var errs []string

	var installed []packages.PkgInfo
	if len(yumInstalled) > 0 || len(yumUpdated) > 0 || len(yumRemoved) > 0 {
		installed, err = packages.InstalledRPMPackages(ctx)
		if err != nil {
			return err
		}
	}

	var updates []packages.PkgInfo
	if len(yumUpdated) > 0 {
		updates, err = packages.YumUpdates(ctx)
		if err != nil {
			return err
		}
	}

	changes := getNecessaryChanges(installed, updates, yumInstalled, yumRemoved, yumUpdated)

	if changes.packagesToInstall != nil {
		clog.Infof(ctx, "Installing packages %s", changes.packagesToInstall)
		if err := packages.InstallYumPackages(ctx, changes.packagesToInstall); err != nil {
			errs = append(errs, fmt.Sprintf("error installing yum packages: %v", err))
		}
	}

	if changes.packagesToUpgrade != nil {
		clog.Infof(ctx, "Upgrading packages %s", changes.packagesToUpgrade)
		if err := packages.InstallYumPackages(ctx, changes.packagesToUpgrade); err != nil {
			errs = append(errs, fmt.Sprintf("error upgrading yum packages: %v", err))
		}
	}

	if changes.packagesToRemove != nil {
		clog.Infof(ctx, "Removing packages %s", changes.packagesToRemove)
		if err := packages.RemoveYumPackages(ctx, changes.packagesToRemove); err != nil {
			errs = append(errs, fmt.Sprintf("error removing yum packages: %v", err))
		}
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}
