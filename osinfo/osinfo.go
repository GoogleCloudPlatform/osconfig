/*
Copyright 2017 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package osinfo provides basic system info functions for Windows and
// Linux.
package osinfo

import (
	"context"
)

const (
	// DefaultShortNameLinux is the default shortname used for a Linux system.
	DefaultShortNameLinux = "linux"
	// DefaultShortNameWindows is the default shortname used for Windows system.
	DefaultShortNameWindows = "windows"
)

// Provider is an interface for OSInfo extraction on different systems.
type Provider interface {
	GetOSInfo(context.Context) (OSInfo, error)
}

// NewProvider returns fully function provider.
func NewProvider() Provider {
	return defaultProvider{}
}

type defaultProvider struct{}

// GetOSInfo extract return OSInfo for current platform.
func (defaultProvider) GetOSInfo(ctx context.Context) (OSInfo, error) {
	return Get()
}

// OSInfo describes an operating system.
type OSInfo struct {
	Hostname, LongName, ShortName, Version, KernelVersion, KernelRelease, Architecture string
}

// NormalizeArchitecture attempts to standardize architecture naming.
func NormalizeArchitecture(arch string) string {
	switch arch {
	case "amd64", "64-bit":
		arch = "x86_64"
	case "i386", "i686", "32-bit":
		arch = "x86_32"
	case "noarch":
		arch = "all"
	}
	return arch
}

type osNameAndVersionProvider func() (shortName string, longName string, version string)
