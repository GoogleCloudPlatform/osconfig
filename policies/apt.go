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
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

var debArchiveTypeMap = map[agentendpointpb.AptRepository_ArchiveType]string{
	agentendpointpb.AptRepository_DEB:     "deb",
	agentendpointpb.AptRepository_DEB_SRC: "deb-src",
}

const aptGPGFile = "/etc/apt/trusted.gpg.d/osconfig_agent_managed.gpg"

func getAptGPGKey(key string) (*openpgp.Entity, error) {
	resp, err := http.Get(key)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.ContentLength > 1024*1024 {
		return nil, fmt.Errorf("key size of %d too large", resp.ContentLength)
	}

	var buf bytes.Buffer
	tee := io.TeeReader(resp.Body, &buf)

	b, err := armor.Decode(tee)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if b == nil {
		return openpgp.ReadEntity(packet.NewReader(&buf))
	}
	return openpgp.ReadEntity(packet.NewReader(b.Body))
}

func containsEntity(es []*openpgp.Entity, e *openpgp.Entity) bool {
	for _, entity := range es {
		if entity.PrimaryKey.Fingerprint == e.PrimaryKey.Fingerprint {
			return true
		}
	}
	return false
}

func aptRepositories(ctx context.Context, repos []*agentendpointpb.AptRepository, repoFile string) error {
	var es []*openpgp.Entity
	var keys []string
	for _, repo := range repos {
		key := repo.GetGpgKey()
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, key := range keys {
		e, err := getAptGPGKey(key)
		if err != nil {
			clog.Errorf(ctx, "Error fetching gpg key %q: %v", key, err)
			continue
		}
		if !containsEntity(es, e) {
			es = append(es, e)
		}
	}

	if len(es) > 0 {
		var buf bytes.Buffer
		for _, e := range es {
			if err := e.Serialize(&buf); err != nil {
				clog.Errorf(ctx, "Error serializing gpg key: %v", err)
			}
		}
		if err := writeIfChanged(ctx, buf.Bytes(), aptGPGFile); err != nil {
			clog.Errorf(ctx, "Error writing gpg key: %v", err)
		}
	}

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

	return writeIfChanged(ctx, buf.Bytes(), repoFile)
}

func aptChanges(ctx context.Context, aptInstalled, aptRemoved, aptUpdated []*agentendpointpb.Package) error {
	var err error
	var errs []string

	var installed []*packages.PkgInfo
	if len(aptInstalled) > 0 || len(aptUpdated) > 0 || len(aptRemoved) > 0 {
		installed, err = packages.InstalledDebPackages(ctx)
		if err != nil {
			return err
		}
	}

	var updates []*packages.PkgInfo
	if len(aptUpdated) > 0 {
		updates, err = packages.AptUpdates(ctx, packages.AptGetUpgradeType(packages.AptGetDistUpgrade), packages.AptGetUpgradeShowNew(false))
		if err != nil {
			return err
		}
	}

	changes := getNecessaryChanges(installed, updates, aptInstalled, aptRemoved, aptUpdated)

	if changes.packagesToInstall != nil {
		// run apt-get update to update to latest changes.
		if _, err := packages.AptUpdate(ctx); err != nil {
			clog.Errorf(ctx, "Error running apt-get update")
		}
		clog.Infof(ctx, "Installing packages %s", changes.packagesToInstall)
		if err := packages.InstallAptPackages(ctx, changes.packagesToInstall); err != nil {
			clog.Errorf(ctx, "Error installing apt packages: %v", err)

			// Try fallback logic to install the packages individually.
			clog.Infof(ctx, "Trying to install packages individually")
			var installPkgErrs []string
			for _, pkg := range changes.packagesToInstall {
				if err = packages.InstallAptPackages(ctx, []string{pkg}); err != nil {
					installPkgErrs = append(installPkgErrs, fmt.Sprintf("Error installing apt package: %v. Error details: %v", pkg, err))
				}
			}

			if len(installPkgErrs) > 0 {
				errorString := strings.Join(installPkgErrs, "\n")
				clog.Errorf(ctx, "Error installing apt packages individually: %v", errorString)
				errs = append(errs, fmt.Sprintf("error installing apt packages: %v", errorString))
			}
		}
	} else {
		clog.Debugf(ctx, "No packages to install.")
	}

	if changes.packagesToUpgrade != nil {
		clog.Infof(ctx, "Upgrading packages %s", changes.packagesToUpgrade)
		if err := packages.InstallAptPackages(ctx, changes.packagesToUpgrade); err != nil {
			clog.Errorf(ctx, "Error upgrading apt packages: %v", err)
			errs = append(errs, fmt.Sprintf("error upgrading apt packages: %v", err))
		}
	} else {
		clog.Debugf(ctx, "No packages to upgrade.")
	}

	if changes.packagesToRemove != nil {
		clog.Infof(ctx, "Removing packages %s", changes.packagesToRemove)
		if err := packages.RemoveAptPackages(ctx, changes.packagesToRemove); err != nil {
			clog.Errorf(ctx, "Error removing apt packages: %v", err)

			// Try fallback logic to remove the packages individually.
			clog.Infof(ctx, "Trying to remove packages individually")
			var removePkgErrs []string
			for _, pkg := range changes.packagesToRemove {
				if err = packages.RemoveAptPackages(ctx, []string{pkg}); err != nil {
					removePkgErrs = append(removePkgErrs, fmt.Sprintf("Error removing apt package: %v. Error details: %v", pkg, err))
				}
			}

			if len(removePkgErrs) > 0 {
				errorString := strings.Join(removePkgErrs, "\n")
				clog.Errorf(ctx, "Error removing apt packages individually: %v", errorString)
				errs = append(errs, fmt.Sprintf("error removing apt packages: %v", errorString))
			}
		}
	} else {
		clog.Debugf(ctx, "No packages to remove.")
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, ",\n"))
}
