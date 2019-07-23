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
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
)

var debArchiveTypeMap = map[osconfigpb.AptRepository_ArchiveType]string{
	osconfigpb.AptRepository_DEB:     "deb",
	osconfigpb.AptRepository_DEB_SRC: "deb-src",
}

func aptRepositories(repos []*osconfigpb.AptRepository, repoFile string) error {
	/*
		# Repo file managed by Google OSConfig agent
		deb http://repo1-url/ repo1 main
		deb http://repo1-url/ repo2 main contrib non-free
	*/
	var buf bytes.Buffer
	buf.WriteString("# Repo file managed by Google OSConfig agent\n")
	for _, repo := range repos {
		archiveType, ok := debArchiveTypeMap[repo.ArchiveType]
		if !ok {
			archiveType = "deb"
		}
		line := fmt.Sprintf("\n%s %s %s", archiveType, repo.Uri, repo.Distribution)
		for _, c := range repo.Components {
			line = fmt.Sprintf("%s %s", line, c)
		}
		buf.WriteString(line + "\n")
	}

	return writeIfChanged(buf.Bytes(), repoFile)
}

func aptChanges(aptInstalled, aptRemoved, aptUpdated []*osconfigpb.Package) error {
	var errs []string

	installed, err := packages.InstalledDebPackages()
	if err != nil {
		return err
	}
	updates, err := packages.AptUpdates()
	if err != nil {
		return err
	}
	changes := getNecessaryChanges(installed, updates, aptInstalled, aptRemoved, aptUpdated)

	if changes.packagesToInstall != nil {
		for _, p := range changes.packagesToInstall {
			logger.Infof("Installing package %s", p)
			if err := packages.InstallAptPackage(p); err != nil {
				logger.Errorf("Error installing apt package '%s': %v", p, err)
				errs = append(errs, fmt.Sprintf("error installing apt package: %v", err))
			}
		}
	}

	if changes.packagesToUpgrade != nil {
		for _, p := range changes.packagesToUpgrade {
			logger.Infof("Upgrading package %s", p)
			if err := packages.InstallAptPackage(p); err != nil {
				logger.Errorf("Error upgrading apt package '%s': %v", p, err)
				errs = append(errs, fmt.Sprintf("error upgrading apt package: %v", err))
			}
		}
	}

	if changes.packagesToRemove != nil {
		for _, p := range changes.packagesToRemove {
			logger.Infof("Removing package %s", p)
			if err := packages.RemoveAptPackage(p); err != nil {
				logger.Errorf("Error removing apt package '%s': %v", p, err)
				errs = append(errs, fmt.Sprintf("error removing apt package: %v", err))
			}
		}
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}
