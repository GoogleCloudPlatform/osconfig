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

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestLoadValidConfig(t *testing.T) {
	content := `{
		"project": "test-project",
		"zone": "us-west1-b",
		"network": "custom-network",
		"test_timeout": "15m",
		"poll_interval": "5s",
		"cleanup_timeout": "3m",
		"max_concurrent_vms": 5
	}`
	tmp := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	want := Config{
		Project:          "test-project",
		Zone:             "us-west1-b",
		Network:          "custom-network",
		ServiceAccount:   "default",
		TestTimeout:      15 * time.Minute,
		PollInterval:     5 * time.Second,
		CleanupTimeout:   3 * time.Minute,
		MaxConcurrentVMs: 5,
		ArtifactDir:      filepath.Join(filepath.Dir(tmp), "artifacts"),
		JUnitFile:        filepath.Join(filepath.Dir(tmp), "artifacts", "junit.xml"),
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("Load() mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadDefaults(t *testing.T) {
	content := `{"project": "test-project", "zone": "us-central1-a", "max_concurrent_vms": 10}`
	tmp := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	want := Config{
		Project:          "test-project",
		Zone:             "us-central1-a",
		Network:          "global/networks/default",
		ServiceAccount:   "default",
		TestTimeout:      5 * time.Minute,
		PollInterval:     10 * time.Second,
		CleanupTimeout:   5 * time.Minute,
		MaxConcurrentVMs: 10,
		ArtifactDir:      filepath.Join(filepath.Dir(tmp), "artifacts"),
		JUnitFile:        filepath.Join(filepath.Dir(tmp), "artifacts", "junit.xml"),
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("Load() defaults mismatch (-want +got):\n%s", diff)
	}
}
