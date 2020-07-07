//  Copyright 2017 Google Inc. All Rights Reserved.
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

// Package inventory scans the current inventory (patches and package installed and available)
// and writes them to Guest Attributes.
package inventory

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/attributes"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
)

const (
	inventoryURL = agentconfig.ReportURL + "/guestInventory"
)

// InstanceInventory is an instances inventory data.
type InstanceInventory struct {
	Hostname             string
	LongName             string
	ShortName            string
	Version              string
	Architecture         string
	KernelVersion        string
	KernelRelease        string
	OSConfigAgentVersion string
	InstalledPackages    packages.Packages
	PackageUpdates       packages.Packages
	LastUpdated          string
}

func write(state *InstanceInventory, url string) {
	logger.Debugf("Writing instance inventory.")

	e := reflect.ValueOf(state).Elem()
	t := e.Type()
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		u := fmt.Sprintf("%s/%s", url, t.Field(i).Name)
		switch f.Kind() {
		case reflect.String:
			logger.Debugf("postAttribute %s: %+v", u, f)
			if err := attributes.PostAttribute(u, strings.NewReader(f.String())); err != nil {
				logger.Errorf("postAttribute error: %v", err)
			}
		case reflect.Struct:
			logger.Debugf("postAttributeCompressed %s: %+v", u, f)
			if err := attributes.PostAttributeCompressed(u, f.Interface()); err != nil {
				logger.Errorf("postAttributeCompressed error: %v", err)
			}
		}
	}
}

// Get generates inventory data.
func Get() *InstanceInventory {
	logger.Debugf("Gathering instance inventory.")

	hs := &InstanceInventory{}

	installedPackages, err := packages.GetInstalledPackages()
	if err != nil {
		logger.Errorf("packages.GetInstalledPackages() error: %v", err)
	}

	packageUpdates, err := packages.GetPackageUpdates()
	if err != nil {
		logger.Errorf("packages.GetPackageUpdates() error: %v", err)
	}

	oi, err := osinfo.Get()
	if err != nil {
		logger.Errorf("osinfo.Get() error: %v", err)
	}

	hs.Hostname = oi.Hostname
	hs.LongName = oi.LongName
	hs.ShortName = oi.ShortName
	hs.Version = oi.Version
	hs.KernelVersion = oi.KernelVersion
	hs.KernelRelease = oi.KernelRelease
	hs.Architecture = oi.Architecture
	hs.OSConfigAgentVersion = agentconfig.Version()
	hs.InstalledPackages = installedPackages
	hs.PackageUpdates = packageUpdates

	hs.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	return hs
}

// Run gathers and records inventory information using tasker.Enqueue.
func Run() {
	tasker.Enqueue("Run OSInventory", func() { write(Get(), inventoryURL) })
}
