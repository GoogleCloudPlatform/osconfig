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

// Package guestpolicy configures OS repos, installs or removes system packages,
// and handles software recipes as directed by applicable guest policy objects.
package guestpolicy

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

	"github.com/GoogleCloudPlatform/compute-image-tools/go/osinfo"
	"github.com/GoogleCloudPlatform/compute-image-tools/go/packages"
	osconfig "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/cloud.google.com/go/osconfig/apiv1alpha1"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha1"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/logger"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/kylelemons/godebug/pretty"
	"google.golang.org/api/option"
	"google.golang.org/grpc/status"
)

var dump = &pretty.Config{IncludeUnexported: true}

func run(ctx context.Context, res string) {
	client, err := osconfig.NewClient(ctx, option.WithEndpoint(config.SvcEndpoint()), option.WithCredentialsFile(config.OAuthPath()))
	if err != nil {
		logger.Errorf("osconfig.NewClient Error: %v", err)
		return
	}
	defer client.Close()

	resp, err := lookupConfigs(ctx, client, res)
	if err != nil {
		logger.Errorf("LookupConfigs error: %v", err)
		return
	}

	if err = doPackages(resp); err != nil {
		logger.Errorf("Package or repo errors: %v", err)
		return
	}
	//doRecipes(resp)
}

// Run looks up osconfigs and applies them using tasker.Enqueue.
func Run(ctx context.Context, res string) {
	tasker.Enqueue("Run OSPackage", func() { run(ctx, res) })
}

func lookupConfigs(ctx context.Context, client *osconfig.Client, resource string) (*osconfigpb.LookupConfigsResponse, error) {
	info, err := osinfo.GetDistributionInfo()
	if err != nil {
		return nil, err
	}

	req := &osconfigpb.LookupConfigsRequest{
		Resource: resource,
		OsInfo: &osconfigpb.LookupConfigsRequest_OsInfo{
			OsLongName:     info.LongName,
			OsShortName:    info.ShortName,
			OsVersion:      info.Version,
			OsKernel:       info.Kernel,
			OsArchitecture: info.Architecture,
		},
		ConfigTypes: []osconfigpb.LookupConfigsRequest_ConfigType{
			osconfigpb.LookupConfigsRequest_GOO,
			osconfigpb.LookupConfigsRequest_WINDOWS_UPDATE,
			osconfigpb.LookupConfigsRequest_APT,
			osconfigpb.LookupConfigsRequest_YUM,
			osconfigpb.LookupConfigsRequest_ZYPPER,
		},
	}
	logger.Debugf("LookupConfigs request:\n%s\n\n", dump.Sprint(req))

	res, err := client.LookupConfigs(ctx, req)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			return nil, fmt.Errorf("code: %q, message: %q, details: %q", s.Code(), s.Message(), s.Details())
		}
		return nil, err
	}
	logger.Debugf("LookupConfigs response:\n%s\n\n", dump.Sprint(res))

	return res, nil
}

func doPackages(res *osconfigpb.LookupConfigsResponse) error {
	var errs []string

	// Create repos.
	// current logic is:
	//   match type,
	//   create repo,
	//   get repo handle,
	//   do packages
	// new is:
	//   if repos, pass repos to func or block
	//   if packages, pass packages to func or block
	//   if recipes, pass recipes to func or block
	// where each func or block will:
	//   iterate list
	//   maybe make a list of matching upfront?
	//   for each type we could support, iterate the desired packages and append to
	//   installable
	//   do packages/repos/recipes
	// interim is recipe as osconfig
	// does that wire here or top level?
	if res.Goo != nil && packages.GooGetExists {
		if _, err := os.Stat(config.GoogetRepoFilePath()); os.IsNotExist(err) {
			logger.Infof("Repo dir does not exist, will create it...")
			if err := os.MkdirAll(filepath.Dir(config.GoogetRepoFilePath()), 0755); err != nil {
				errStr := fmt.Sprintf("Error creating repo dir: %v", err)
				logger.Errorf(errStr)
				errs = append(errs, errStr)
			}
		}
		if err := writeGoogetRepos(res.Goo.Repositories, config.GoogetRepoFilePath()); err != nil {
			errStr := fmt.Sprintf("Error writing googet repo file: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
		if err := updateGoogetPackages(res.Goo.PackageInstalls, res.Goo.PackageRemovals); err != nil {
			errStr := fmt.Sprintf("Error performing googet changes: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
	}

	if res.Apt != nil && packages.AptExists {
		if _, err := os.Stat(config.AptRepoFilePath()); os.IsNotExist(err) {
			logger.Infof("Repo dir does not exist, will create it...")
			if err := os.MkdirAll(filepath.Dir(config.AptRepoFilePath()), 0755); err != nil {
				errStr := fmt.Sprintf("Error creating repo file: %v", err)
				logger.Errorf(errStr)
				errs = append(errs, errStr)
			}
		}
		if err := writeAptRepos(res.Apt.Repositories, config.AptRepoFilePath()); err != nil {
			errStr := fmt.Sprintf("Error writing apt repo file: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
		if err := updateAptPackages(res.Apt.PackageInstalls, res.Apt.PackageRemovals); err != nil {
			errStr := fmt.Sprintf("error performing apt changes: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
	}

	if res.Yum != nil && packages.YumExists {
		if _, err := os.Stat(config.YumRepoFilePath()); os.IsNotExist(err) {
			logger.Infof("Repo dir does not exist, will create it...")
			if err := os.MkdirAll(filepath.Dir(config.YumRepoFilePath()), 0755); err != nil {
				errStr := fmt.Sprintf("Error creating yum repo dir: %v", err)
				logger.Errorf(errStr)
				errs = append(errs, errStr)
			}
		}
		if err := writeYumRepos(res.Yum.Repositories, config.YumRepoFilePath()); err != nil {
			errStr := fmt.Sprintf("Error writing yum repo file: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
		if err := updateYumPackages(res.Yum.PackageInstalls, res.Yum.PackageRemovals); err != nil {
			errStr := fmt.Sprintf("Error performing yum changes: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
	}

	if res.Zypper != nil && packages.ZypperExists {
		if _, err := os.Stat(config.ZypperRepoFilePath()); os.IsNotExist(err) {
			logger.Infof("Repo dir does not exist, will create it...")
			if err := os.MkdirAll(filepath.Dir(config.ZypperRepoFilePath()), 0755); err != nil {
				errStr := fmt.Sprintf("Error creating repo dir: %v", err)
				logger.Errorf(errStr)
				errs = append(errs, errStr)
			}
		}
		if err := writeZypperRepos(res.Zypper.Repositories, config.ZypperRepoFilePath()); err != nil {
			errStr := fmt.Sprintf("Error writing zypper repo file: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
		if err := updateZypperPackages(res.Zypper.PackageInstalls, res.Zypper.PackageRemovals); err != nil {
			errStr := fmt.Sprintf("Error performing zypper changes: %v", err)
			logger.Errorf(errStr)
			errs = append(errs, errStr)
		}
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}

/*
func doRecipes(res *osconfigpb.LookupConfigsResponse) error {
	var errs []string

	for _, recipe := range res.Recipes {
		if installedVersion, ok := installedRecipes[recipe.Name]; ok {
			if installedVersion.Lower(recipe.Version) || installedVersion.Equals(recipe.Version) {
				continue
			}
			if recipe.Status != recipes.UP_TO_DATE {
				continue
			}
			if recipe.UpgradeSteps == nil {
				continue
			}
			if err = upgrade(recipe); err != nil {
				errs = append(errs, err)
			}
		} else {
			if err = install(recipe); err != nil {
				errs = append(errs, err)
			}
		}
	}
}
*/

func checksum(r io.Reader) hash.Hash {
	hash := sha256.New()
	io.Copy(hash, r)
	return hash
}

func writeIfChanged(content []byte, path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
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
	if _, err := file.WriteAt(content, 0); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}
