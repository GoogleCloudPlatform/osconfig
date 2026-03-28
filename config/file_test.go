//  Copyright 2026 Google Inc. All Rights Reserved.
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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
)

// setupTempDir creates a temporary directory for file downloads and returns its path.
func setupTempDir(t *testing.T) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "osconfig_file_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	return tmpDir
}

// setEnv temporarily sets an environment variable, restoring its previous value
// (or unsetting it) when the test finishes.
func setEnv(t *testing.T, key, val string) {
	t.Helper()
	oldVal, exists := os.LookupEnv(key)
	os.Setenv(key, val)
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, oldVal)
		} else {
			os.Unsetenv(key)
		}
	})
}

// setupMockServer creates a mock HTTP server to simulate remote file downloads.
func setupMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/success" {
			w.Write([]byte("test content"))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// TestChecksum verifies the SHA256 checksum calculation for given io.Reader inputs.
func TestChecksum(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "simple string",
			input: "test content",
			want:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			if got := checksum(r); got != tt.want {
				t.Errorf("checksum() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDownloadFile tests remote file downloading and error handling.
func TestDownloadFile(t *testing.T) {
	mockServer := setupMockServer(t)
	tmpDir := setupTempDir(t)

	// Set STORAGE_EMULATOR_HOST to the mock server to test GCS path without credentials.
	// The mock server will return 404 for GCS requests, producing a predictable error.
	host := strings.TrimPrefix(mockServer.URL, "http://")

	tests := []struct {
		name    string
		file    *agentendpointpb.OSPolicy_Resource_File
		wantErr string
		env     map[string]string
	}{
		{
			name: "HTTP remote file success without checksum",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri: mockServer.URL + "/success",
					},
				},
			},
		},
		{
			name: "HTTP remote file success with correct checksum",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri:            mockServer.URL + "/success",
						Sha256Checksum: "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
					},
				},
			},
		},
		{
			name: "HTTP remote file failure with incorrect checksum",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri:            mockServer.URL + "/success",
						Sha256Checksum: "badchecksum",
					},
				},
			},
			wantErr: `got "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72" for checksum, expected "badchecksum"`,
		},
		{
			name: "Unknown file type",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: nil,
			},
			wantErr: "unknown remote File type: <nil>",
		},
		{
			name: "GCS remote file fetch error",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Gcs_{
					Gcs: &agentendpointpb.OSPolicy_Resource_File_Gcs{
						Bucket: "test-bucket",
						Object: "test-object",
					},
				},
			},
			wantErr: "storage: object doesn't exist",
			env: map[string]string{
				"STORAGE_EMULATOR_HOST": host,
			},
		},
		{
			name: "HTTP remote file fetch error",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri: "httpx://foo/bar",
					},
				},
			},
			wantErr: `Get "httpx://foo/bar": unsupported protocol scheme "httpx"`,
		},
		{
			name: "HTTP remote file 404 Not Found",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri: mockServer.URL + "/notfound",
					},
				},
			},
			wantErr: "got http status 404 when attempting to download artifact",
		},
		{
			name: "GCS client creation error",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Gcs_{
					Gcs: &agentendpointpb.OSPolicy_Resource_File_Gcs{
						Bucket: "test-bucket",
						Object: "test-object",
					},
				},
			},
			wantErr: "error creating gcs client: dialing: open /non/existent/path/to/creds.json: no such file or directory",
			env: map[string]string{
				"STORAGE_EMULATOR_HOST":          "",
				"GOOGLE_APPLICATION_CREDENTIALS": "/non/existent/path/to/creds.json",
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				setEnv(t, k, v)
			}

			ctx := context.Background()
			destPath := filepath.Join(tmpDir, fmt.Sprintf("test_file_%d", i))

			_, err := downloadFile(ctx, destPath, 0644, tt.file)

			if err == nil {
				if tt.wantErr != "" {
					t.Errorf("downloadFile() expected error %q, got nil", tt.wantErr)
				}
			} else if err.Error() != tt.wantErr {
				t.Errorf("downloadFile() error = %q, wantErr %q", err.Error(), tt.wantErr)
			}
		})
	}
}
