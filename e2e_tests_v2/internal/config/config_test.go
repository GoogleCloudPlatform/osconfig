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

// Package config tests.
package config

import (
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		flags   map[string]string
		wantCfg Config
		wantErr error
	}{
		{
			name:  "no flags provided, returns default configuration",
			flags: nil,
			wantCfg: Config{
				Project:          DefaultProject,
				Zone:             DefaultZone,
				Network:          DefaultNetwork,
				Subnetwork:       DefaultSubnetwork,
				ServiceAccount:   DefaultServiceAccount,
				TestTimeout:      DefaultTestTimeout,
				PollInterval:     DefaultPollInterval,
				CleanupTimeout:   DefaultCleanupTimeout,
				MaxConcurrentVMs: DefaultMaxConcurrentVMs,
				ArtifactFile:     DefaultArtifactFile,
			},
			wantErr: nil,
		},
		{
			name: "all valid flags provided, returns custom configuration",
			flags: map[string]string{
				"project":            "custom-proj",
				"zone":               "us-west1-b",
				"network":            "custom-net",
				"subnetwork":         "custom-subnet",
				"service_account":    "custom-sa@custom.iam.gserviceaccount.com",
				"test_timeout":       "15m",
				"poll_interval":      "5s",
				"cleanup_timeout":    "3m",
				"max_concurrent_vms": "10",
				"artifact_file":      "/custom/artifacts/custom-junit.xml",
			},
			wantCfg: Config{
				Project:          "custom-proj",
				Zone:             "us-west1-b",
				Network:          "custom-net",
				Subnetwork:       "custom-subnet",
				ServiceAccount:   "custom-sa@custom.iam.gserviceaccount.com",
				TestTimeout:      15 * time.Minute,
				PollInterval:     5 * time.Second,
				CleanupTimeout:   3 * time.Minute,
				MaxConcurrentVMs: 10,
				ArtifactFile:     "/custom/artifacts/custom-junit.xml",
			},
			wantErr: nil,
		},
		{
			name: "subset of valid flags provided, returns partial override with remaining defaults",
			flags: map[string]string{
				"project":            "custom-proj-only",
				"zone":               "europe-west1-c",
				"max_concurrent_vms": "8",
			},
			wantCfg: Config{
				Project:          "custom-proj-only",
				Zone:             "europe-west1-c",
				Network:          DefaultNetwork,
				Subnetwork:       DefaultSubnetwork,
				ServiceAccount:   DefaultServiceAccount,
				TestTimeout:      DefaultTestTimeout,
				PollInterval:     DefaultPollInterval,
				CleanupTimeout:   DefaultCleanupTimeout,
				MaxConcurrentVMs: 8,
				ArtifactFile:     DefaultArtifactFile,
			},
			wantErr: nil,
		},
		{
			name: "flags provided with whitespace, returns trimmed configuration",
			flags: map[string]string{
				"project": "  trimmed-proj  ",
				"zone":    "  us-east1-a  ",
				"network": "  custom-net  ",
			},
			wantCfg: Config{
				Project:          "trimmed-proj",
				Zone:             "us-east1-a",
				Network:          "custom-net",
				Subnetwork:       DefaultSubnetwork,
				ServiceAccount:   DefaultServiceAccount,
				TestTimeout:      DefaultTestTimeout,
				PollInterval:     DefaultPollInterval,
				CleanupTimeout:   DefaultCleanupTimeout,
				MaxConcurrentVMs: DefaultMaxConcurrentVMs,
				ArtifactFile:     DefaultArtifactFile,
			},
			wantErr: nil,
		},
		{
			name: "empty project flag provided, returns project required error",
			flags: map[string]string{
				"project": "",
			},
			wantErr: errors.New("project is required in configuration"),
		},
		{
			name: "whitespace project flag provided, returns project required error",
			flags: map[string]string{
				"project": "   ",
			},
			wantErr: errors.New("project is required in configuration"),
		},
		{
			name: "empty zone flag provided, returns zone required error",
			flags: map[string]string{
				"zone": "",
			},
			wantErr: errors.New("zone is required in configuration"),
		},
		{
			name: "whitespace zone flag provided, returns zone required error",
			flags: map[string]string{
				"zone": "   ",
			},
			wantErr: errors.New("zone is required in configuration"),
		},
		{
			name: "zero test_timeout flag provided, returns positive test_timeout required error",
			flags: map[string]string{
				"test_timeout": "0s",
			},
			wantErr: errors.New("test_timeout must be positive, got 0s"),
		},
		{
			name: "negative test_timeout flag provided, returns positive test_timeout required error",
			flags: map[string]string{
				"test_timeout": "-1m",
			},
			wantErr: errors.New("test_timeout must be positive, got -1m0s"),
		},
		{
			name: "zero poll_interval flag provided, returns positive poll_interval required error",
			flags: map[string]string{
				"poll_interval": "0s",
			},
			wantErr: errors.New("poll_interval must be positive, got 0s"),
		},
		{
			name: "negative poll_interval flag provided, returns positive poll_interval required error",
			flags: map[string]string{
				"poll_interval": "-5s",
			},
			wantErr: errors.New("poll_interval must be positive, got -5s"),
		},
		{
			name: "zero cleanup_timeout flag provided, returns positive cleanup_timeout required error",
			flags: map[string]string{
				"cleanup_timeout": "0s",
			},
			wantErr: errors.New("cleanup_timeout must be positive, got 0s"),
		},
		{
			name: "negative cleanup_timeout flag provided, returns positive cleanup_timeout required error",
			flags: map[string]string{
				"cleanup_timeout": "-2m",
			},
			wantErr: errors.New("cleanup_timeout must be positive, got -2m0s"),
		},
		{
			name: "zero max_concurrent_vms flag provided, returns positive max VMs required error",
			flags: map[string]string{
				"max_concurrent_vms": "0",
			},
			wantErr: errors.New("invalid max VMs amount: 0"),
		},
		{
			name: "negative max_concurrent_vms flag provided, returns positive max VMs required error",
			flags: map[string]string{
				"max_concurrent_vms": "-5",
			},
			wantErr: errors.New("invalid max VMs amount: -5"),
		},
	}

	equateErrorMessages := cmp.Comparer(func(x, y error) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Error() == y.Error()
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFlags(t, tt.flags)

			gotCfg, gotErr := Load()

			if diff := cmp.Diff(tt.wantErr, gotErr, equateErrorMessages); diff != "" {
				t.Errorf("Load() error mismatch (-want +got):\n%s", diff)
				return
			}
			if tt.wantErr == nil {
				if diff := cmp.Diff(tt.wantCfg, gotCfg); diff != "" {
					t.Errorf("Load() config mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func setFlags(t *testing.T, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		f := flag.Lookup(name)
		if f == nil {
			t.Fatalf("flag %q not found on flag.CommandLine", name)
		}
		oldVal := f.Value.String()
		if err := flag.Set(name, value); err != nil {
			t.Fatalf("flag.Set(%q, %q) error: %v", name, value, err)
		}
		t.Cleanup(func() {
			_ = flag.Set(name, oldVal)
		})
	}
}
