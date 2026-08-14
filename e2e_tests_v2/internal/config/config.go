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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const envConfigPath = "E2E_CONFIG"

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
	ArtifactDir      string        `json:"artifact_dir"`
	JUnitFile        string        `json:"junit_file"`
}

type fileConfig struct {
	Project          string `json:"project"`
	Zone             string `json:"zone"`
	Network          string `json:"network"`
	Subnetwork       string `json:"subnetwork"`
	ServiceAccount   string `json:"service_account"`
	TestTimeout      string `json:"test_timeout"`
	PollInterval     string `json:"poll_interval"`
	CleanupTimeout   string `json:"cleanup_timeout"`
	MaxConcurrentVMs int    `json:"max_concurrent_vms"`
	ArtifactDir      string `json:"artifact_dir"`
	JUnitFile        string `json:"junit_file"`
}

// LoadFromEnvironment loads configuration from the file specified in E2E_CONFIG.
func LoadFromEnvironment() (Config, error) {
	path := strings.TrimSpace(os.Getenv(envConfigPath))
	if path == "" {
		return Config{}, fmt.Errorf("%s must point to a JSON configuration file", envConfigPath)
	}
	return Load(path)
}

// Load reads and validates a JSON configuration file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}
	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}

	project := strings.TrimSpace(raw.Project)
	if project == "" {
		return Config{}, fmt.Errorf("project is required in configuration")
	}

	zone := strings.TrimSpace(raw.Zone)
	if zone == "" {
		return Config{}, fmt.Errorf("zone is required in configuration")
	}

	testTimeout, err := durationOrDefault(raw.TestTimeout, 5*time.Minute)
	if err != nil {
		return Config{}, fmt.Errorf("invalid test_timeout: %w", err)
	}

	pollInterval, err := durationOrDefault(raw.PollInterval, 10*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("invalid poll_interval: %w", err)
	}

	cleanupTimeout, err := durationOrDefault(raw.CleanupTimeout, 5*time.Minute)
	if err != nil {
		return Config{}, fmt.Errorf("invalid cleanup_timeout: %w", err)
	}

	artifactDir := strings.TrimSpace(raw.ArtifactDir)
	if artifactDir == "" {
		artifactDir = filepath.Join(filepath.Dir(path), "artifacts")
	} else if !filepath.IsAbs(artifactDir) {
		artifactDir = filepath.Join(filepath.Dir(path), artifactDir)
	}
	junitFile := filepath.Join(artifactDir, "junit.xml")

	maxVMs := raw.MaxConcurrentVMs
	if maxVMs <= 0 {
		return Config{}, fmt.Errorf("invalid max VMs amount: %v", maxVMs)
	}

	return Config{
		Project:          project,
		Zone:             zone,
		Network:          valueOrDefault(raw.Network, "global/networks/default"),
		Subnetwork:       strings.TrimSpace(raw.Subnetwork),
		ServiceAccount:   valueOrDefault(raw.ServiceAccount, "default"),
		TestTimeout:      testTimeout,
		PollInterval:     pollInterval,
		CleanupTimeout:   cleanupTimeout,
		MaxConcurrentVMs: maxVMs,
		ArtifactDir:      artifactDir,
		JUnitFile:        junitFile,
	}, nil
}

func durationOrDefault(val string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(val) == "" {
		return fallback, nil
	}
	return time.ParseDuration(val)
}

func valueOrDefault(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return strings.TrimSpace(val)
}
