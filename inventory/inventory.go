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
	"context"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltrace"
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
	InstalledPackages    *packages.Packages
	PackageUpdates       *packages.Packages
	LastUpdated          string
}

type clock interface {
	Now() time.Time
}

type defaultClock struct{}

func newDefaultClock() clock {
	return defaultClock{}
}

func (dc defaultClock) Now() time.Time {
	return time.Now()
}

// Provider extract all inventormation and returns InstanceInventory aggregate
type Provider interface {
	Get(context.Context) *InstanceInventory
}

type defaultInventoryProvider struct {
	osInfoProvider osinfo.Provider

	packageUpdatesProvider    packages.PackageUpdatesProvider
	installedPackagesProvider packages.InstalledPackagesProvider

	clock clock
}

// NewProvider returns ready to work default provider
func NewProvider() Provider {
	osInfoProvider := osinfo.NewProvider()
	installedPackagesProvider := packages.NewInstalledPackagesProvider()
	if agentconfig.TraceGetInventory() {
		ctx := context.Background()
		installedPackagesProvider = packages.TracingInstalledPackagesProvider(installedPackagesProvider, func(stats utiltrace.TraceMemoryResult, duration time.Duration) {
			osinfo, _ := osInfoProvider.GetOSInfo(ctx)
			clog.Infof(
				ctx,
				"GetInstalledPackages: %.3fs, memory %+.2f MB (=%.2f-%.2f), peak %.2f MB, mean %.2f MB (%d samples), OS: %s@%s",
				duration.Seconds(),
				stats.MemAfterMB-stats.MemBeforeMB,
				stats.MemAfterMB,
				stats.MemBeforeMB,
				stats.MemPeakMB,
				stats.MemMeanMB,
				stats.SampleCount,
				osinfo.ShortName,
				osinfo.KernelRelease,
			)
		})
	}
	return &defaultInventoryProvider{
		osInfoProvider:            osInfoProvider,
		packageUpdatesProvider:    packages.NewPackageUpdatesProvider(),
		installedPackagesProvider: installedPackagesProvider,
		clock:                     newDefaultClock(),
	}
}

// Get extracts all required data from the VM and returns it as InstanceInventory aggregate
func (p *defaultInventoryProvider) Get(ctx context.Context) *InstanceInventory {
	clog.Debugf(ctx, "Gathering instance inventory.")

	installedPackages, err := p.installedPackagesProvider.GetInstalledPackages(ctx)
	if err != nil {
		clog.Errorf(ctx, "packages.GetInstalledPackages() error: %v", err)
	}

	packageUpdates, err := p.packageUpdatesProvider.GetPackageUpdates(ctx)
	if err != nil {
		clog.Errorf(ctx, "packages.GetPackageUpdates() error: %v", err)
	}

	oi, err := p.osInfoProvider.GetOSInfo(ctx)
	if err != nil {
		clog.Errorf(ctx, "osinfo.Get() error: %v", err)
	}

	return &InstanceInventory{
		Hostname:             oi.Hostname,
		LongName:             oi.LongName,
		ShortName:            oi.ShortName,
		Version:              oi.Version,
		KernelVersion:        oi.KernelVersion,
		KernelRelease:        oi.KernelRelease,
		Architecture:         oi.Architecture,
		OSConfigAgentVersion: agentconfig.Version(),
		InstalledPackages:    &installedPackages,
		PackageUpdates:       &packageUpdates,
		LastUpdated:          p.clock.Now().UTC().Format(time.RFC3339),
	}
}
