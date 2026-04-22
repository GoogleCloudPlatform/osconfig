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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// setupMockServer creates a mock HTTP server that serves:
//   - "/success" for plain HTTP fetches
//   - GCS-style object path "/storage/v1/b/<bucket>/o/<object>" for GCS fetches
//     when STORAGE_EMULATOR_HOST is pointed at this server.
//
// Anything else returns 404.
func setupMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/success",
			strings.HasPrefix(r.URL.Path, "/storage/v1/b/test-bucket/o/test-object"),
			strings.HasPrefix(r.URL.Path, "/test-bucket/test-object"):
			w.Write([]byte("test content"))
		default:
			http.NotFound(w, r)
		}
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
			name:  `empty string, valid checksum`,
			input: "",
			want:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  `"test content", valid checksum`,
			input: "test content",
			want:  "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checksum(strings.NewReader(tt.input))

			utiltest.AssertEquals(t, got, tt.want)
		})
	}
}

// TestDownloadFile tests remote file downloading and error handling.
func TestDownloadFile(t *testing.T) {
	mockServer := setupMockServer(t)
	tmpDir := t.TempDir()

	// Mock host used for STORAGE_EMULATOR_HOST to exercise the GCS path without real creds.
	gcsHost := strings.TrimPrefix(mockServer.URL, "http://")

	tests := []struct {
		name    string
		file    *agentendpointpb.OSPolicy_Resource_File
		wantErr error
		setup   func(t *testing.T, i int) string
	}{
		{
			name: "Remote file success without checksum, successsful download",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri: mockServer.URL + "/success",
					},
				},
			},
		},
		{
			name: "Remote file success with correct checksum, successsful download",
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
			name: "Remote file with incorrect checksum, failed download",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri:            mockServer.URL + "/success",
						Sha256Checksum: "badchecksum",
					},
				},
			},
			wantErr: errors.New(`got "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72" for checksum, expected "badchecksum"`),
		},
		{
			name: "Remote file with unsupported protocol, failed download",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri: "httpx://foo/bar",
					},
				},
			},
			wantErr: &url.Error{Op: "Get", URL: "httpx://foo/bar", Err: errors.New(`unsupported protocol scheme "httpx"`)},
		},
		{
			name: "Remote file not found, failed download",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Remote_{
					Remote: &agentendpointpb.OSPolicy_Resource_File_Remote{
						Uri: mockServer.URL + "/notfound",
					},
				},
			},
			wantErr: errors.New("got http status 404 when attempting to download artifact"),
		},
		{
			name: "Unknown file type, failed download",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: nil,
			},
			wantErr: errors.New("unknown remote File type: <nil>"),
		},
		{
			name: "GCS existing remote file, fetch success",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Gcs_{
					Gcs: &agentendpointpb.OSPolicy_Resource_File_Gcs{
						Bucket: "test-bucket",
						Object: "test-object",
					},
				},
			},
			setup: func(t *testing.T, i int) string {
				t.Setenv("STORAGE_EMULATOR_HOST", gcsHost)
				return filepath.Join(tmpDir, fmt.Sprintf("test_file_%d", i))
			},
		},
		{
			name: "GCS missing remote file, not found error",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Gcs_{
					Gcs: &agentendpointpb.OSPolicy_Resource_File_Gcs{
						Bucket: "missing-bucket",
						Object: "missing-object",
					},
				},
			},
			setup: func(t *testing.T, i int) string {
				t.Setenv("STORAGE_EMULATOR_HOST", gcsHost)
				return filepath.Join(tmpDir, fmt.Sprintf("test_file_%d", i))
			},
			wantErr: errors.New("storage: object doesn't exist"),
		},
		{
			name: "GCS client creation error, failed download",
			file: &agentendpointpb.OSPolicy_Resource_File{
				Type: &agentendpointpb.OSPolicy_Resource_File_Gcs_{
					Gcs: &agentendpointpb.OSPolicy_Resource_File_Gcs{
						Bucket: "test-bucket",
						Object: "test-object",
					},
				},
			},
			setup: func(t *testing.T, i int) string {
				t.Setenv("STORAGE_EMULATOR_HOST", "")
				t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/non/existent/path/to/creds.json")
				return filepath.Join(tmpDir, fmt.Sprintf("test_file_%d", i))
			},
			wantErr: errors.New("error creating gcs client: dialing: open /non/existent/path/to/creds.json: no such file or directory"),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var destPath string
			if tt.setup != nil {
				destPath = tt.setup(t, i)
			} else {
				destPath = filepath.Join(tmpDir, fmt.Sprintf("test_file_%d", i))
			}

			_, err := downloadFile(context.Background(), destPath, 0644, tt.file)

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
