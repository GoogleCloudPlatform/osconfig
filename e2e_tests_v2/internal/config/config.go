//  Copyright 2026 Google LLC
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

// Package config manages E2E test configuration.
package config

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

// Default configuration values.
const (
	DefaultProject          = "gcloud-parity-testing"
	DefaultZone             = "us-central1-a"
	DefaultNetwork          = "global/networks/default"
	DefaultSubnetwork       = ""
	DefaultServiceAccount   = "default"
	DefaultTestTimeout      = 60 * time.Minute
	DefaultPollInterval     = 10 * time.Second
	DefaultCleanupTimeout   = 5 * time.Minute
	DefaultMaxConcurrentVMs = 5
	DefaultArtifactFile     = "./artifacts/junit.xml"
)

var (
	projectFlag          = flag.String("project", DefaultProject, "GCP project ID for E2E tests")
	zoneFlag             = flag.String("zone", DefaultZone, "GCP zone for E2E test VMs")
	networkFlag          = flag.String("network", DefaultNetwork, "VPC network for E2E test VMs")
	subnetworkFlag       = flag.String("subnetwork", DefaultSubnetwork, "VPC subnetwork for E2E test VMs")
	serviceAccountFlag   = flag.String("service_account", DefaultServiceAccount, "Service account for E2E test VMs")
	testTimeoutFlag      = flag.Duration("test_timeout", DefaultTestTimeout, "Timeout for E2E test execution")
	pollIntervalFlag     = flag.Duration("poll_interval", DefaultPollInterval, "Interval between status polls")
	cleanupTimeoutFlag   = flag.Duration("cleanup_timeout", DefaultCleanupTimeout, "Timeout for resource cleanup")
	maxConcurrentVMsFlag = flag.Int("max_concurrent_vms", DefaultMaxConcurrentVMs, "Maximum number of VMs running concurrently")
	artifactFileFlag     = flag.String("artifact_file", DefaultArtifactFile, "Path to artifact file (JUnit XML report)")
)

// Config contains runtime settings for E2E tests.
type Config struct {
	Project          string        `json:"project"`
	Zone             string        `json:"zone"`
	Network          string        `json:"network"`
	Subnetwork       string        `json:"subnetwork,omitempty"`
	ServiceAccount   string        `json:"service_account"`
	TestTimeout      time.Duration `json:"test_timeout"`
	PollInterval     time.Duration `json:"poll_interval"`
	CleanupTimeout   time.Duration `json:"cleanup_timeout"`
	MaxConcurrentVMs int           `json:"max_concurrent_vms"`
	ArtifactFile     string        `json:"artifact_file"`
}

// Load returns configuration from command-line flags and applies defaults.
func Load() (Config, error) {
	if !flag.Parsed() {
		flag.Parse()
	}

	cfg := Config{
		Project:          strings.TrimSpace(*projectFlag),
		Zone:             strings.TrimSpace(*zoneFlag),
		Network:          valueOrDefault(*networkFlag, DefaultNetwork),
		Subnetwork:       strings.TrimSpace(*subnetworkFlag),
		ServiceAccount:   valueOrDefault(*serviceAccountFlag, DefaultServiceAccount),
		TestTimeout:      *testTimeoutFlag,
		PollInterval:     *pollIntervalFlag,
		CleanupTimeout:   *cleanupTimeoutFlag,
		MaxConcurrentVMs: *maxConcurrentVMsFlag,
		ArtifactFile:     valueOrDefault(*artifactFileFlag, DefaultArtifactFile),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if strings.TrimSpace(c.Project) == "" {
		return fmt.Errorf("project is required in configuration")
	}
	if strings.TrimSpace(c.Zone) == "" {
		return fmt.Errorf("zone is required in configuration")
	}
	if c.TestTimeout <= 0 {
		return fmt.Errorf("test_timeout must be positive, got %v", c.TestTimeout)
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("poll_interval must be positive, got %v", c.PollInterval)
	}
	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("cleanup_timeout must be positive, got %v", c.CleanupTimeout)
	}
	if c.MaxConcurrentVMs <= 0 {
		return fmt.Errorf("invalid max VMs amount: %v", c.MaxConcurrentVMs)
	}
	return nil
}

func valueOrDefault(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return strings.TrimSpace(val)
}
