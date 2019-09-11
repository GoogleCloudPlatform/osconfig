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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/fsouza/fake-gcs-server/fakestorage"
)

func TestFetchArtifacts_GCS_happyCase(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			BucketName: "some-bucket",
			Name:       "some-object.txt",
			Content:    []byte("inside the file"),
		},
	})
	defer server.Stop()

	getGCSClient = func(ctx context.Context) (*storage.Client, error) {
		return server.Client(), nil
	}

	gcsartifact := osconfigpb.SoftwareRecipe_Artifact_Gcs{
		Object:     "some-object.txt",
		Bucket:     "some-bucket",
		Generation: 1,
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Gcs_{Gcs: &gcsartifact},
			AllowInsecure: true,
		},
	}
	FetchArtifacts(context.Background(), artifacts, "/tmp/")

	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if !exists("/tmp/id1.txt") {
		t.Errorf("expected fetched artifacts")
	}

	if err = os.Remove("/tmp/id1.txt"); err != nil {
		t.Fatalf("could not clean up test data")
	}

	if art["id1"] != "/tmp/id1.txt" {
		t.Errorf("expected artifact entry(%s), got(%s)\n", "/tmp/id1", art["id1"])
	}
}

func TestFetchArtifacts_GCS_ReadError(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			BucketName: "some-bucket",
			Name:       "some-object.txt",
			Content:    []byte("inside the file"),
		},
	})
	defer server.Stop()

	getGCSClient = func(ctx context.Context) (*storage.Client, error) {
		return server.Client(), nil
	}

	gcsartifact := osconfigpb.SoftwareRecipe_Artifact_Gcs{
		Object:     "some-other-object.txt",
		Bucket:     "some-bucket",
		Generation: 1,
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Gcs_{Gcs: &gcsartifact},
			AllowInsecure: true,
		},
	}
	FetchArtifacts(context.Background(), artifacts, "/tmp/")

	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if len(art) != 0 {
		t.Errorf("unexpted artifact fetched")
	}

	if err == nil {
		t.Errorf("Expected (reader error), got(%+v)\n", err)
	}
}

func TestFetchArtifacts_GCS_ClientError(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			BucketName: "some-bucket",
			Name:       "some-object.txt",
			Content:    []byte("inside the file"),
		},
	})
	defer server.Stop()

	getGCSClient = func(ctx context.Context) (*storage.Client, error) {
		return nil, errors.New("Error creating storage client")
	}

	gcsartifact := osconfigpb.SoftwareRecipe_Artifact_Gcs{
		Object: "some-object.txt",
		Bucket: "some-bucket",
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Gcs_{Gcs: &gcsartifact},
			AllowInsecure: true,
		},
	}
	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if len(art) != 0 {
		t.Errorf("unexpted artifact fetched")
	}

	if err == nil {
		t.Errorf("Expected (client error), got(%+v)\n", err)
	}
}

func TestFetchArtifacts_http_happycase(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		rw.Write([]byte("ok"))
		// Send response to be tested
	}))
	// Close the server when test finishes
	defer server.Close()

	getHTTPClient = func() (*http.Client, error) {
		return server.Client(), nil
	}
	remoteartifact := osconfigpb.SoftwareRecipe_Artifact_Remote{
		Uri:      fmt.Sprintf("%s/agent.deb", server.URL),
		Checksum: "",
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Remote_{Remote: &remoteartifact},
			AllowInsecure: true,
		},
	}

	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if !exists("/tmp/id1.deb") {
		t.Errorf("expected fetched artifacts")
	}

	if err = os.Remove("/tmp/id1.deb"); err != nil {
		t.Fatalf("could not clean up test data")
	}

	if art["id1"] != "/tmp/id1.deb" {
		t.Errorf("expected file entry, but not present")
	}
}

func TestFetchArtifacts_http_InvalidURL(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		rw.Write([]byte("ok"))
		// Send response to be tested
	}))
	// Close the server when test finishes
	defer server.Close()

	getHTTPClient = func() (*http.Client, error) {
		return server.Client(), nil
	}
	remoteartifact := osconfigpb.SoftwareRecipe_Artifact_Remote{
		Uri:      fmt.Sprintf("%sagent.deb", server.URL),
		Checksum: "",
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Remote_{Remote: &remoteartifact},
			AllowInsecure: true,
		},
	}

	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if len(art) != 0 {
		t.Errorf("unexpected artifact downloaded")
	}

	if err == nil || !strings.Contains(err.Error(), "Could not parse url") {
		t.Errorf("expected url error, found(%+v)", err)
	}

}

func TestFetchArtifacts_http_NonHTTPRemote(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		rw.Write([]byte("ok"))
		// Send response to be tested
	}))
	// Close the server when test finishes
	defer server.Close()

	getHTTPClient = func() (*http.Client, error) {
		return server.Client(), nil
	}

	url := strings.Replace(server.URL, "http", "ftp", -1)
	remoteartifact := osconfigpb.SoftwareRecipe_Artifact_Remote{
		Uri:      fmt.Sprintf("%s/agent.deb", url),
		Checksum: "",
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Remote_{Remote: &remoteartifact},
			AllowInsecure: true,
		},
	}

	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if len(art) != 0 {
		t.Errorf("unexpected artifact downloaded")
	}

	if err == nil || !strings.Contains(err.Error(), "unsupported protocol") {
		t.Errorf("expected url error, found(%+v)", err)
	}

}

func TestFetchArtifacts_http_FetchError(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		rw.Write([]byte("ok"))
		// Send response to be tested
	}))
	// Close the server when test finishes
	defer server.Close()

	getHTTPClient = func() (*http.Client, error) {
		return server.Client(), nil
	}

	remoteartifact := osconfigpb.SoftwareRecipe_Artifact_Remote{
		Uri:      fmt.Sprintf("http://non-existent-domain.com/agent.deb"),
		Checksum: "",
	}
	artifacts := []*osconfigpb.SoftwareRecipe_Artifact{
		&osconfigpb.SoftwareRecipe_Artifact{
			Id:            "id1",
			Artifact:      &osconfigpb.SoftwareRecipe_Artifact_Remote_{Remote: &remoteartifact},
			AllowInsecure: true,
		},
	}

	art, err := FetchArtifacts(context.Background(), artifacts, "/tmp/")

	if len(art) != 0 {
		t.Errorf("unexpected artifact downloaded")
	}

	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected fetch error, found(%+v)", err)
	}

}
