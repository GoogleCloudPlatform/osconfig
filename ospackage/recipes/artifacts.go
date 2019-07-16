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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

var (
	testStorageClient *storage.Client
	testHTTPClient    *http.Client
)

func newStorageClient(ctx context.Context) (*storage.Client, error) {
	if testStorageClient != nil {
		return testStorageClient, nil
	}
	return storage.NewClient(ctx)
}

func newHTTPClient() *http.Client {
	if testHTTPClient != nil {
		return testHTTPClient
	}
	return &http.Client{}
}

// FetchArtifacts takes in a slice of artifacs and dowloads them into the specified directory,
// Returns a map of artifact names to their new locations on the local disk.
func FetchArtifacts(ctx context.Context, artifacts []*osconfigpb.SoftwareRecipe_Artifact, directory string) (map[string]string, error) {
	localNames := make(map[string]string)

	for _, a := range artifacts {
		path, err := fetchArtifact(ctx, a, directory)
		if err != nil {
			return nil, err
		}
		localNames[a.Id] = path
	}

	return localNames, nil
}

func fetchArtifact(ctx context.Context, artifact *osconfigpb.SoftwareRecipe_Artifact, directory string) (string, error) {
	localPath := filepath.Join(directory, artifact.Id)
	uri, err := url.Parse(artifact.Uri)
	if err != nil {
		return "", fmt.Errorf("Could not parse url %q for artifact %q", artifact.Uri, artifact.Id)
	}

	var reader io.ReadCloser
	switch strings.ToLower(uri.Scheme) {
	case "gs":
		reader, err = fetchWithGCS(ctx, uri.Host, uri.Path, uri.Fragment)
		if err != nil {
			return "", fmt.Errorf("error fetching artifact %q from GCS: %v", artifact.Id, err)
		}
		defer reader.Close()
	case "https", "http":
		response, err := fetchWithHTTP(ctx, artifact.Uri)
		if err != nil {
			return "", fmt.Errorf("error fetching artifact %q with http or https: %v", artifact.Id, err)
		}
		defer response.Body.Close()
		reader = response.Body
	default:
		return "", fmt.Errorf("artifact %q has unsupported protocol %s", artifact.Id, uri.Scheme)
	}

	err = downloadStream(reader, artifact.Checksum, localPath)
	if err != nil {
		return "", err
	}
	return localPath, nil
}

func fetchWithGCS(ctx context.Context, bucket, path, generation string) (*storage.Reader, error) {
	client, err := newStorageClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	oh := client.Bucket(bucket).Object(path)
	if generation != "" {
		generationNumber, err := strconv.ParseInt(generation, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse gcs generation number %q", generation)
		}
		oh = oh.Generation(generationNumber)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func fetchWithHTTP(ctx context.Context, uri string) (*http.Response, error) {
	resp, err := newHTTPClient().Get(uri)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http status %d when attempting to download artifact", resp.StatusCode)
	}

	return resp, nil
}

func downloadStream(r io.Reader, checksum string, localPath string) error {
	localPath, err := normPath(localPath)
	if err != nil {
		return err
	}
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(file, hasher), r); err != nil {
		return err
	}
	computed := hex.EncodeToString(hasher.Sum(nil))
	if checksum != "" && !strings.EqualFold(checksum, computed) {
		return fmt.Errorf("got %q for checksum, expected %q", computed, checksum)
	}
	return nil
}

// normPath transforms a windows path into an extended-length path as described in
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
// when not running on windows it will just return the input path. Copied from
// https://github.com/google/googet/blob/master/oswrap/oswrap_windows.go
func normPath(path string) (string, error) {
	if runtime.GOOS != "windows" {
		return path, nil
	}

	if strings.HasPrefix(path, "\\\\?\\") {
		return path, nil
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	path = filepath.Clean(path)
	return "\\\\?\\" + path, nil
}
