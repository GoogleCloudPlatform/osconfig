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
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	osconfig "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/cloud.google.com/go/osconfig/apiv1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/kylelemons/godebug/pretty"
	"google.golang.org/api/option"
	"google.golang.org/grpc/status"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

var dump = &pretty.Config{IncludeUnexported: true}

func run(ctx context.Context, res string) {
	client, err := osconfig.NewClient(ctx, option.WithEndpoint(config.SvcEndpoint()), option.WithCredentialsFile(config.OAuthPath()))
	if err != nil {
		logger.Errorf("osconfig.NewClient Error: %v", err)
		return
	}
	defer client.Close()

	resp, err := lookupEffectivePolicies(ctx, client, res)
	if err != nil {
		logger.Errorf("lookupEffectivePolicies error: %v", err)
		return
	}

	// We don't check the error from ospackage.SetConfig as all errors are already logged.
	setConfig(resp)
}

// Run looks up osconfigs and applies them using tasker.Enqueue.
func Run(ctx context.Context, res string) {
	tasker.Enqueue("Run GuestPolicies", func() { run(ctx, res) })
}

func lookupEffectivePolicies(ctx context.Context, client *osconfig.Client, instance string) (*osconfigpb.LookupEffectiveGuestPoliciesResponse, error) {
	var shortName, version, arch string
	if config.OSInventoryEnabled() {
		logger.Debugf("OS Inventory enabled for instance, gathering DistributionInfo for LookupEffectiveGuestPoliciesRequest")
		info, err := osinfo.GetDistributionInfo()
		if err != nil {
			return nil, err
		}
		shortName = info.ShortName
		version = info.Version
		arch = info.Architecture
	} else {
		logger.Debugf("OS Inventory not enabled for instance, not gathering DistributionInfo for LookupEffectiveGuestPoliciesRequest")
	}

	identityToken, err := metadata.Get(config.IdentityTokenPath)
	if err != nil {
		return nil, err
	}

	req := &osconfigpb.LookupEffectiveGuestPoliciesRequest{
		Instance:        instance,
		OsShortName:     shortName,
		OsVersion:       version,
		OsArchitecture:  arch,
		InstanceIdToken: identityToken,
	}
	logger.Debugf("LookupEffectiveGuestPolicies request: {Instance: %s, OsShortName: %s, OsVersion: %s, OsArchitecture: %s}",
		req.GetInstance(), req.GetOsShortName(), req.GetOsVersion(), req.GetOsArchitecture())

	res, err := client.LookupEffectiveGuestPolicies(ctx, req)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			return nil, fmt.Errorf("code: %q, message: %q, details: %q", s.Code(), s.Message(), s.Details())
		}
		return nil, err
	}
	logger.Debugf("LookupEffectiveGuestPolicies response:\n%s\n\n", dump.Sprint(res))

	return res, nil
}

func setConfig(res *osconfigpb.LookupEffectiveGuestPoliciesResponse) error {
	var aptRepos []*osconfigpb.AptRepository
	var yumRepos []*osconfigpb.YumRepository
	var zypperRepos []*osconfigpb.ZypperRepository
	var gooRepos []*osconfigpb.GooRepository
	for _, repo := range res.GetPackageRepositories() {
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

	var gooInstallPkgs, gooRemovePkgs, gooUpdatePkgs []*osconfigpb.Package
	var aptInstallPkgs, aptRemovePkgs, aptUpdatePkgs []*osconfigpb.Package
	var yumInstallPkgs, yumRemovePkgs, yumUpdatePkgs []*osconfigpb.Package
	var zypperInstallPkgs, zypperRemovePkgs, zypperUpdatePkgs []*osconfigpb.Package
	for _, pkg := range res.GetPackages() {
		switch pkg.GetPackage().GetManager() {
		case osconfigpb.Package_ANY, osconfigpb.Package_MANAGER_UNSPECIFIED:
			switch pkg.GetPackage().GetDesiredState() {
			case osconfigpb.DesiredState_INSTALLED, osconfigpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				gooInstallPkgs = append(gooInstallPkgs, pkg.GetPackage())
				aptInstallPkgs = append(aptInstallPkgs, pkg.GetPackage())
				yumInstallPkgs = append(yumInstallPkgs, pkg.GetPackage())
				zypperInstallPkgs = append(zypperInstallPkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_REMOVED:
				gooRemovePkgs = append(gooRemovePkgs, pkg.GetPackage())
				aptRemovePkgs = append(aptRemovePkgs, pkg.GetPackage())
				yumRemovePkgs = append(yumRemovePkgs, pkg.GetPackage())
				zypperRemovePkgs = append(zypperRemovePkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_UPDATED:
				gooUpdatePkgs = append(gooUpdatePkgs, pkg.GetPackage())
				aptUpdatePkgs = append(aptUpdatePkgs, pkg.GetPackage())
				yumUpdatePkgs = append(yumUpdatePkgs, pkg.GetPackage())
				zypperUpdatePkgs = append(zypperUpdatePkgs, pkg.GetPackage())
			}
		case osconfigpb.Package_GOO:
			switch pkg.GetPackage().GetDesiredState() {
			case osconfigpb.DesiredState_INSTALLED, osconfigpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				gooInstallPkgs = append(gooInstallPkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_REMOVED:
				gooRemovePkgs = append(gooRemovePkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_UPDATED:
				gooUpdatePkgs = append(gooUpdatePkgs, pkg.GetPackage())
			}
		case osconfigpb.Package_APT:
			switch pkg.GetPackage().GetDesiredState() {
			case osconfigpb.DesiredState_INSTALLED, osconfigpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				aptInstallPkgs = append(aptInstallPkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_REMOVED:
				aptRemovePkgs = append(aptRemovePkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_UPDATED:
				aptUpdatePkgs = append(aptUpdatePkgs, pkg.GetPackage())
			}
		case osconfigpb.Package_YUM:
			switch pkg.GetPackage().GetDesiredState() {
			case osconfigpb.DesiredState_INSTALLED, osconfigpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				yumInstallPkgs = append(yumInstallPkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_REMOVED:
				yumRemovePkgs = append(yumRemovePkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_UPDATED:
				yumUpdatePkgs = append(yumUpdatePkgs, pkg.GetPackage())
			}
		case osconfigpb.Package_ZYPPER:
			switch pkg.GetPackage().GetDesiredState() {
			case osconfigpb.DesiredState_INSTALLED, osconfigpb.DesiredState_DESIRED_STATE_UNSPECIFIED:
				zypperInstallPkgs = append(zypperInstallPkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_REMOVED:
				zypperRemovePkgs = append(zypperRemovePkgs, pkg.GetPackage())
			case osconfigpb.DesiredState_UPDATED:
				zypperUpdatePkgs = append(zypperUpdatePkgs, pkg.GetPackage())
			}

		}

	}

	var errs []string
	if packages.GooGetExists {
		if _, err := os.Stat(config.GooGetRepoFilePath()); os.IsNotExist(err) {
			logger.Debugf("Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.GooGetRepoFilePath()), 07550); err != nil {
				logger.Errorf("Error creating repo file: %v", err)
				errs = append(errs, fmt.Sprintf("error creating googet repo file: %v", err))
			}
		}
		if err := googetRepositories(gooRepos, config.GooGetRepoFilePath()); err != nil {
			logger.Errorf("Error writing googet repo file: %v", err)
			errs = append(errs, fmt.Sprintf("error writing googet repo file: %v", err))
		}
		if err := googetChanges(gooInstallPkgs, gooRemovePkgs, gooUpdatePkgs); err != nil {
			errs = append(errs, fmt.Sprintf("error performing googet changes: %v", err))
		}
	}

	if packages.AptExists {
		if _, err := os.Stat(config.AptRepoFilePath()); os.IsNotExist(err) {
			logger.Debugf("Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.AptRepoFilePath()), 07550); err != nil {
				logger.Errorf("Error creating repo file: %v", err)
				errs = append(errs, fmt.Sprintf("error creating apt repo file: %v", err))
			}
		}
		if err := aptRepositories(aptRepos, config.AptRepoFilePath()); err != nil {
			logger.Errorf("Error writing apt repo file: %v", err)
			errs = append(errs, fmt.Sprintf("error writing apt repo file: %v", err))
		}
		if err := aptChanges(aptInstallPkgs, aptRemovePkgs, aptUpdatePkgs); err != nil {
			errs = append(errs, fmt.Sprintf("error performing apt changes: %v", err))
		}
	}

	if packages.YumExists {
		if _, err := os.Stat(config.YumRepoFilePath()); os.IsNotExist(err) {
			logger.Debugf("Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.YumRepoFilePath()), 07550); err != nil {
				logger.Errorf("Error creating repo file: %v", err)
				errs = append(errs, fmt.Sprintf("error creating yum repo file: %v", err))
			}
		}
		if err := yumRepositories(yumRepos, config.YumRepoFilePath()); err != nil {
			logger.Errorf("Error writing yum repo file: %v", err)
			errs = append(errs, fmt.Sprintf("error writing yum repo file: %v", err))
		}
		if err := yumChanges(yumInstallPkgs, yumRemovePkgs, yumUpdatePkgs); err != nil {
			errs = append(errs, fmt.Sprintf("error performing yum changes: %v", err))
		}
	}

	if packages.ZypperExists {
		if _, err := os.Stat(config.ZypperRepoFilePath()); os.IsNotExist(err) {
			logger.Debugf("Repo file does not exist, will create one...")
			if err := os.MkdirAll(filepath.Dir(config.ZypperRepoFilePath()), 07550); err != nil {
				logger.Errorf("Error creating repo file: %v", err)
				errs = append(errs, fmt.Sprintf("error creating zypper repo file: %v", err))
			}
		}
		if err := zypperRepositories(zypperRepos, config.ZypperRepoFilePath()); err != nil {
			logger.Errorf("Error writing zypper repo file: %v", err)
			errs = append(errs, fmt.Sprintf("error writing zypper repo file: %v", err))
		}
		if err := zypperChanges(zypperInstallPkgs, zypperRemovePkgs, zypperUpdatePkgs); err != nil {
			errs = append(errs, fmt.Sprintf("error performing zypper changes: %v", err))
		}
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}

func checksum(r io.Reader) hash.Hash {
	hash := sha256.New()
	io.Copy(hash, r)
	return hash
}

func writeIfChanged(content []byte, path string) error {
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

	logger.Infof("Writing repo file %s with updated contents", path)
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
