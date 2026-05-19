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

// TestFetchArtifact_Remote ensures remote artifacts are correctly downloaded and saved to a local directory.
func TestFetchArtifact_Remote(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "remote data")
	}))
	defer ts.Close()

	tdir := t.TempDir()
	artifact := &agentendpointpb.SoftwareRecipe_Artifact{
		Id: "test-artifact",
		Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Remote_{
			Remote: &agentendpointpb.SoftwareRecipe_Artifact_Remote{
				Uri: ts.URL,
			},
		},
	}

	path, err := fetchArtifact(context.Background(), artifact, tdir)
	utiltest.AssertErrorMatch(t, err, nil)
	utiltest.AssertFileContents(t, path, "remote data")
}

// TestFetchArtifact_GCS ensures artifacts can be downloaded from GCS using a storage emulator.
func TestFetchArtifact_GCS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "test-bucket/test-object") {
			fmt.Fprint(w, "gcs data")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	t.Setenv("STORAGE_EMULATOR_HOST", ts.URL)

	tdir := t.TempDir()
	artifact := &agentendpointpb.SoftwareRecipe_Artifact{
		Id: "gcs-artifact",
		Artifact: &agentendpointpb.SoftwareRecipe_Artifact_Gcs_{
			Gcs: &agentendpointpb.SoftwareRecipe_Artifact_Gcs{
				Bucket: "test-bucket",
				Object: "test-object",
			},
		},
	}

	path, err := fetchArtifact(context.Background(), artifact, tdir)
	utiltest.AssertErrorMatch(t, err, nil)
	utiltest.AssertFileContents(t, path, "gcs data")
}

// TestFetchArtifacts verifies that multiple artifacts can be downloaded and mapped to their local paths.
func TestFetchArtifacts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data")
	}))
	defer ts.Close()

	tdir := t.TempDir()
	artifacts := []*agentendpointpb.SoftwareRecipe_Artifact{
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
	}

	res, err := fetchArtifacts(context.Background(), artifacts, tdir)
	utiltest.AssertErrorMatch(t, err, nil)
	utiltest.AssertEquals(t, len(res), 2)
	for _, path := range res {
		utiltest.AssertFileContents(t, path, "data")
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
