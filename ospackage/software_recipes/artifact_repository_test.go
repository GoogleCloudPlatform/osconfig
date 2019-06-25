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

package software_recipes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"

	"google.golang.org/api/option"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

func TestGetArtifacts_NoArtifacts(t *testing.T) {
	files, err := FetchArtifacts(context.Background(), []*osconfigpb.SoftwareRecipe_Artifact{}, "/test")
	if err != nil {
		t.Fatalf("FetchArtifacts(ctx, {}, \"/test\") returned unexpected error %q", err)
	}
	if len(files) != 0 {
		t.Fatalf("FetchArtifacts(ctx, {}, \"/test\") = %v, want {}", files)
	}
}

func createHandler(responseMap map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		resp, ok := responseMap[path]
		if !ok {
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("No mapping found for path %q", path)))
		}
		w.Write([]byte(resp))
	})
}

func TestGetHttpArtifact(t *testing.T) {
	s := httptest.NewTLSServer(createHandler(map[string]string{
		"/testartifact": "testartifact body",
	}))
	defer s.Close()
	fh := &FakeFileHandler{
		CreatedFiles: map[string]*StringWriteCloser{},
	}
	testHttpClient = s.Client()
	testFileHandler = fh
	directory := "/testdir"
	wantLocation := "/testdir/test"
	wantMap := map[string]string{"test": wantLocation}

	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{{
		Id:     "test",
		Uri:      s.URL + "/testartifact",
		Checksum: "d53a628153c63429b3709e0a50a326efea2fc40b7c4afd70101c4b5bc16054ae",
	}}

	files, err := FetchArtifacts(context.Background(), artifacts, directory)
	if err != nil {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) returned unexpected error %q", artifacts, directory, err)
	}
	if !cmp.Equal(files, wantMap) {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) = %v, wanted map with one entry", artifacts, directory, files)
	}
	wc, ok := fh.CreatedFiles[wantLocation]
	if !ok {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) did not create expected file %s", artifacts, directory, wantLocation)
	}
	if wc.GetWrittenString() != "testartifact body" {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) wrote %q to file, expected %q", artifacts, directory, wc.GetWrittenString(), "testartifact body")
	}
}

func TestGetGCSArtifact(t *testing.T) {
	ctx := context.Background()
	urls := []string{"gs://testbucket/testobject", "https://testbucket.storage.googleapis.com/testobject", "http://storage.googleapis.com/testbucket/testobject", "https://storage.googleapis.com/testbucket/testobject"}
	s := httptest.NewServer(createHandler(map[string]string{
		"/testartifact": "testartifact body",
	}))
	defer s.Close()
	fh := &FakeFileHandler{
		CreatedFiles: map[string]*StringWriteCloser{},
	}
	var err error
	testStorageClient, err = storage.NewClient(ctx, option.WithEndpoint(s.URL), option.WithHTTPClient(s.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}

	testHttpClient = s.Client()
	testFileHandler = fh

	directory := "/testdir"
	wantLocation := "/testdir/test"
	wantMap := map[string]string{"test": wantLocation}
	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			artifacts := []*osconfigpb.SoftwareRecipe_Artifact{{
				Id:     "test",
				Uri:      url,
				Checksum: "d53a628153c63429b3709e0a50a326efea2fc40b7c4afd70101c4b5bc16054ae",
			}}
			files, err := FetchArtifacts(ctx, artifacts, directory)
			if err != nil {
				t.Fatalf("FetchArtifacts(ctx, %v, %q) returned unexpected error %q", artifacts, directory, err)
			}
			if !cmp.Equal(files, wantMap) {
				t.Fatalf("FetchArtifacts(ctx, %v, %q) = %v, wanted map with one entry", artifacts, directory, files)
			}
			wc, ok := fh.CreatedFiles[wantLocation]
			if !ok {
				t.Fatalf("FetchArtifacts(ctx, %v, %q) did not create expected file %s", artifacts, directory, wantLocation)
			}
			if wc.GetWrittenString() != "testartifact body" {
				t.Fatalf("FetchArtifacts(ctx, %v, %q) wrote %q to file, expected %q", artifacts, directory, wc.GetWrittenString(), "testartifact body")
			}
		})
	}
}
