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

// Package policies configures OS Guest Policies based on osconfig API response.
package policies

import (
	"bytes"
	"context"
	"crypto/sha256"
	"hash"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/agentendpoint"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/policies/recipes"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"
	"github.com/GoogleCloudPlatform/osconfig/tasker"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

func run(ctx context.Context) {
	var resp *agentendpointpb.EffectiveGuestPolicy

	client, err := agentendpoint.NewBetaClient(ctx)
	if err != nil {
		clog.Errorf(ctx, "agentendpoint.NewBetaClient Error: %v", err)
	} else {
		defer client.Close()
		resp, err = client.LookupEffectiveGuestPolicies(ctx)
		if err != nil {
			clog.Errorf(ctx, "Error running LookupEffectiveGuestPolicies: %v", err)
		}
	}

	local, err := readLocalConfig(ctx)
	if err != nil {
		clog.Errorf(ctx, "Error reading local software config: %v", err)
	}

	effective := mergeConfigs(local, resp)

	// We don't check the error from setConfig or installRecipes as all errors are already logged.
	setConfig(ctx, effective)
	installRecipes(ctx, effective)
}

// Run looks up osconfigs and applies them using tasker.Enqueue.
func Run(ctx context.Context) {
	tasker.Enqueue(ctx, "Run GuestPolicies", func() { run(ctx) })
}

func installRecipes(ctx context.Context, egp *agentendpointpb.EffectiveGuestPolicy) error {
	for _, recipe := range egp.GetSoftwareRecipes() {
		if r := recipe.GetSoftwareRecipe(); r != nil {
			if err := recipes.InstallRecipe(ctx, r); err != nil {
				clog.Errorf(ctx, "Error installing recipe: %v", err)
			}
		}
	}
	return nil
}

func setConfig(ctx context.Context, egp *agentendpointpb.EffectiveGuestPolicy) {
	var aptRepos []*agentendpointpb.AptRepository
	var yumRepos []*agentendpointpb.YumRepository
	var zypperRepos []*agentendpointpb.ZypperRepository
	var gooRepos []*agentendpointpb.GooRepository
	for _, repo := range egp.GetPackageRepositories() {
		if r := repo.GetPackageRepository().GetGoo(); r != nil {
			gooRepos = append(gooRepos, r)
			continue
		}
		if r := repo.GetPackageRepository().GetApt(); r != nil {
			aptRepos = append(aptRepos, r)
			continue
		}
		if r := repo.GetPackageRepository().GetYum(); r != nil {
			yumRepos = append(yumRepos, r)
			continue
		}
		if r := repo.GetPackageRepository().GetZypper(); r != nil {
			zypperRepos = append(zypperRepos, r)
			continue
		}
	}

	var gooInstallPkgs, gooRemovePkgs, gooUpdatePkgs []*agentendpointpb.Package
	var aptInstallPkgs, aptRemovePkgs, aptUpdatePkgs []*agentendpointpb.Package
	var yumInstallPkgs, yumRemovePkgs, yumUpdatePkgs []*agentendpointpb.Package
	var zypperInstallPkgs, zypperRemovePkgs, zypperUpdatePkgs []*agentendpointpb.Package
	for _, pkg := range egp.GetPackages() {
		switch pkg.GetPackage().GetManager() {
		case agentendpointpb.Package_ANY, agentendpointpb.Package_MANAGER_UNSPECIFIED:
			switch pkg.GetPackage().GetDesiredState() {
			case agentendpointpb.DesiredState_INSTALLED, agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				gooInstallPkgs = append(gooInstallPkgs, pkg.GetPackage())
				aptInstallPkgs = append(aptInstallPkgs, pkg.GetPackage())
				yumInstallPkgs = append(yumInstallPkgs, pkg.GetPackage())
				zypperInstallPkgs = append(zypperInstallPkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_REMOVED:
				gooRemovePkgs = append(gooRemovePkgs, pkg.GetPackage())
				aptRemovePkgs = append(aptRemovePkgs, pkg.GetPackage())
				yumRemovePkgs = append(yumRemovePkgs, pkg.GetPackage())
				zypperRemovePkgs = append(zypperRemovePkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_UPDATED:
				gooUpdatePkgs = append(gooUpdatePkgs, pkg.GetPackage())
				aptUpdatePkgs = append(aptUpdatePkgs, pkg.GetPackage())
				yumUpdatePkgs = append(yumUpdatePkgs, pkg.GetPackage())
				zypperUpdatePkgs = append(zypperUpdatePkgs, pkg.GetPackage())
			}
		case agentendpointpb.Package_GOO:
			switch pkg.GetPackage().GetDesiredState() {
			case agentendpointpb.DesiredState_INSTALLED, agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				gooInstallPkgs = append(gooInstallPkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_REMOVED:
				gooRemovePkgs = append(gooRemovePkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_UPDATED:
				gooUpdatePkgs = append(gooUpdatePkgs, pkg.GetPackage())
			}
		case agentendpointpb.Package_APT:
			switch pkg.GetPackage().GetDesiredState() {
			case agentendpointpb.DesiredState_INSTALLED, agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				aptInstallPkgs = append(aptInstallPkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_REMOVED:
				aptRemovePkgs = append(aptRemovePkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_UPDATED:
				aptUpdatePkgs = append(aptUpdatePkgs, pkg.GetPackage())
			}
		case agentendpointpb.Package_YUM:
			switch pkg.GetPackage().GetDesiredState() {
			case agentendpointpb.DesiredState_INSTALLED, agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				yumInstallPkgs = append(yumInstallPkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_REMOVED:
				yumRemovePkgs = append(yumRemovePkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_UPDATED:
				yumUpdatePkgs = append(yumUpdatePkgs, pkg.GetPackage())
			}
		case agentendpointpb.Package_ZYPPER:
			switch pkg.GetPackage().GetDesiredState() {
			case agentendpointpb.DesiredState_INSTALLED, agentendpointpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				zypperInstallPkgs = append(zypperInstallPkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_REMOVED:
				zypperRemovePkgs = append(zypperRemovePkgs, pkg.GetPackage())
			case agentendpointpb.DesiredState_UPDATED:
				zypperUpdatePkgs = append(zypperUpdatePkgs, pkg.GetPackage())
			}

		}

	}

	if packages.GooGetExists {
		if _, err := os.Stat(config.GooGetRepoFilePath()); os.IsNotExist(err) {
			clog.Debugf(ctx, "Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.GooGetRepoFilePath()), 07550); err != nil {
				clog.Errorf(ctx, "Error creating repo file: %v", err)
			}
		}
		if err := googetRepositories(ctx, gooRepos, config.GooGetRepoFilePath()); err != nil {
			clog.Errorf(ctx, "Error writing googet repo file: %v", err)
		}
		if err := retryutil.RetryFunc(ctx, 1*time.Minute, "Applying googet changes", func() error {
			return googetChanges(ctx, gooInstallPkgs, gooRemovePkgs, gooUpdatePkgs)
		}); err != nil {
			clog.Errorf(ctx, "Error performing googet changes: %v", err)
		}
	}

	if packages.AptExists {
		if _, err := os.Stat(config.AptRepoFilePath()); os.IsNotExist(err) {
			clog.Debugf(ctx, "Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.AptRepoFilePath()), 07550); err != nil {
				clog.Errorf(ctx, "Error creating repo file: %v", err)
			}
		}
		if err := aptRepositories(ctx, aptRepos, config.AptRepoFilePath()); err != nil {
			clog.Errorf(ctx, "Error writing apt repo file: %v", err)
		}
		if err := retryutil.RetryFunc(ctx, 1*time.Minute, "Applying apt changes", func() error {
			return aptChanges(ctx, aptInstallPkgs, aptRemovePkgs, aptUpdatePkgs)
		}); err != nil {
			clog.Errorf(ctx, "Error performing apt changes: %v", err)
		}
	}

	if packages.YumExists {
		if _, err := os.Stat(config.YumRepoFilePath()); os.IsNotExist(err) {
			clog.Debugf(ctx, "Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.YumRepoFilePath()), 07550); err != nil {
				clog.Errorf(ctx, "Error creating repo file: %v", err)
			}
		}
		if err := yumRepositories(ctx, yumRepos, config.YumRepoFilePath()); err != nil {
			clog.Errorf(ctx, "Error writing yum repo file: %v", err)
		}
		if err := retryutil.RetryFunc(ctx, 1*time.Minute, "Applying yum changes", func() error {
			return yumChanges(ctx, yumInstallPkgs, yumRemovePkgs, yumUpdatePkgs)
		}); err != nil {
			clog.Errorf(ctx, "Error performing yum changes: %v", err)
		}
	}

	if packages.ZypperExists {
		if _, err := os.Stat(config.ZypperRepoFilePath()); os.IsNotExist(err) {
			clog.Debugf(ctx, "Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.ZypperRepoFilePath()), 07550); err != nil {
				clog.Errorf(ctx, "Error creating repo file: %v", err)
			}
		}
		if err := zypperRepositories(ctx, zypperRepos, config.ZypperRepoFilePath()); err != nil {
			clog.Errorf(ctx, "Error writing zypper repo file: %v", err)
		}
		if err := retryutil.RetryFunc(ctx, 1*time.Minute, "Applying zypper changes.", func() error {
			return zypperChanges(ctx, zypperInstallPkgs, zypperRemovePkgs, zypperUpdatePkgs)
		}); err != nil {
			clog.Errorf(ctx, "Error performing zypper changes: %v", err)
		}
	}
}

func checksum(r io.Reader) hash.Hash {
	hash := sha256.New()
	io.Copy(hash, r)
	return hash
}

func writeIfChanged(ctx context.Context, content []byte, path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(content)
	h1 := checksum(reader)
	h2 := checksum(file)
	if bytes.Equal(h1.Sum(nil), h2.Sum(nil)) {
		file.Close()
		return nil
	}

	clog.Infof(ctx, "Writing repo file %s with updated contents", path)
	if err := file.Truncate(0); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteAt(content, 0); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}
