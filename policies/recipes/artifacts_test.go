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

package recipes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
)

// TestGetHTTPArtifact verifies downloading artifacts via HTTP/HTTPS and handles various error scenarios.
func TestGetHTTPArtifact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "test data")
	}))
	defer ts.Close()

	ts404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts404.Close()

	tests := []struct {
		name    string
		uri     string
		client  *http.Client
		want    string
		wantErr error
	}{
		{
			name:    "valid URL, want test data and nil error",
			uri:     ts.URL,
			client:  ts.Client(),
			want:    "test data",
			wantErr: nil,
		},
		{
			name:    "unsupported protocol ftp, want unsupported protocol scheme error",
			uri:     "ftp://google.com/agent.deb",
			client:  http.DefaultClient,
			want:    "",
			wantErr: errors.New("error, unsupported protocol scheme ftp"),
		},
		{
			name:    "http status 404, want got http status 404 error",
			uri:     ts404.URL,
			client:  ts404.Client(),
			want:    "",
			wantErr: errors.New("got http status 404 when attempting to download artifact"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.uri)
			reader, err := getHTTPArtifact(context.Background(), tt.client, *u)

			utiltest.AssertErrorMatchAndSkip(t, err, tt.wantErr)

			defer reader.Close()
			data, _ := io.ReadAll(reader)
			utiltest.AssertEquals(t, string(data), tt.want)
		})
	}
}

// setupFetchArtifactsTest sets up a mock HTTP server and GCS emulator environment for fetchArtifacts tests.
func setupFetchArtifactsTest(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if strings.Contains(r.URL.Path, "test-bucket/test-object") {
				fmt.Fprint(w, "gcs data")
				return
			}
			if strings.Contains(r.URL.Path, "remote-artifact") {
				fmt.Fprint(w, "remote data")
				return
			}
			fmt.Fprint(w, "data")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)

	t.Setenv("STORAGE_EMULATOR_HOST", ts.URL)
	return ts
}

// TestFetchArtifacts verifies that single or multiple artifacts can be correctly downloaded from HTTP/HTTPS sources and GCS.
func TestFetchArtifacts(t *testing.T) {
	ts := setupFetchArtifactsTest(t)

	tests := []struct {
		name      string
		artifacts []*agentendpointpb.SoftwareRecipe_Artifact
		want      map[string]string
		wantErr   error
	}{
		{
			name: "remote artifact, want remote data and nil error",
			artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
				{
					Id: "remote-art",
					Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
						Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
							Uri: ts.URL + "/remote-artifact",
						},
					},
				},
			},
			want: map[string]string{
				"remote-art": "remote data",
			},
		},
		{
			name: "GCS artifact, want gcs data and nil error",
			artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
				{
					Id: "gcs-art",
					Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Gcs_{
						Gcs: &agentendpointpb.SoftwareRecipe_Artifact_Gcs{
							Bucket: "test-bucket",
							Object: "test-object",
						},
					},
				},
			},
			want: map[string]string{
				"gcs-art": "gcs data",
			},
		},
		{
			name: "multiple artifacts, want data and nil error",
			artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
				{
					Id: "art1",
					Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
						Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
							Uri: ts.URL,
						},
					},
				},
				{
					Id: "art2",
					Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
						Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
							Uri: ts.URL,
						},
					},
				},
			},
			want: map[string]string{
				"art1": "data",
				"art2": "data",
			},
		},
		{
			name: "remote artifact unsupported protocol, want protocol error",
			artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
				{
					Id: "ftp-art",
					Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
						Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
							Uri: "ftp://localhost",
						},
					},
				},
			},
			wantErr: fmt.Errorf("error fetching artifact %q: error, unsupported protocol scheme ftp", "ftp-art"),
		},
		{
			name: "unknown artifact type, want unknown artifact error",
			artifacts: []*agentendpointpb.SoftwareRecipe_Artifact{
				{
					Id: "unknown-art",
				},
			},
			wantErr: fmt.Errorf("unknown artifact type for artifact unknown-art"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gotArtifacts, gotErr := fetchArtifacts(context.Background(), tt.artifacts, tmpDir)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, len(gotArtifacts), len(tt.want))
			for id, wantContent := range tt.want {
				path, _ := gotArtifacts[id]
				utiltest.AssertFileContents(t, path, wantContent)
			}
		})
	}
}

func TestRecipesgetStoragePathWithExtension(t *testing.T) {
	expect := "/tmp/artifact-id-1.txt"
	localpath := getStoragePath("/tmp", "artifact-id-1", ".txt")
	if localpath != expect {
		t.Errorf("Expected(%s); got(%s)", expect, localpath)
	}
}

func TestRecipesgetStoragePathWitouthExtension(t *testing.T) {
	expect := "/tmp/artifact-id-1"
	localpath := getStoragePath("/tmp", "artifact-id-1", "")
	if localpath != expect {
		t.Errorf("Expected(%s); got(%s)", expect, localpath)
	}
}
