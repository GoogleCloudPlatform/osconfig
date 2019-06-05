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
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
)

type Artifact struct {
	name           string
	protocol       Protocol
	url            string
	checksum       string
	allow_insecure bool
}

type Protocol string

const (
	GCS   Protocol = "gcs"
	Https Protocol = "https"
	Http  Protocol = "http"
)

var (
	// gcs://(bucket)/(path)
	gcsRegex          = regexp.MustCompile("gcs://([^/]+)/(.+)")
	testStorageClient *storage.Client
	testHttpClient    *http.Client
)

func newStorageClient(ctx context.Context) (*storage.Client, error) {
	if testStorageClient != nil {
		return testStorageClient, nil
	}
	return storage.NewClient(ctx)
}

func newHttpClient() *http.Client {
	if testHttpClient != nil {
		return testHttpClient
	}
	return &http.Client{}
}

func fetchArtifacts(ctx context.Context, artifacts []Artifact, directory string) (map[string]string, error) {
	localNames := make(map[string]string)

	for _, a := range artifacts {
		path, err := fetchArtifact(ctx, a, directory)
		if err != nil {
			return nil, err
		}
		localNames[a.name] = path
	}

	return localNames, nil
}

func fetchArtifact(ctx context.Context, new Artifact, directory string) (string, error) {
	path := path.Join(directory, new.name)
	switch new.protocol {
	case GCS:
		err := fetchFromGCS(ctx, new, path)
		if err != nil {
			return "", err
		}
	case Https, Http:
		err := fetchViaHttp(ctx, new, path)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("Protocol %q not supported", new.protocol)
	}

	return path, nil
}

func fetchFromGCS(ctx context.Context, a Artifact, path string) error {
	matches := gcsRegex.FindStringSubmatch(path)
	if matches == nil || len(matches) < 3 {
		return fmt.Errorf("couldn't parse gcs url %q", path)
	}
	bucket := matches[1]
	object := matches[2]

	client, err := newStorageClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	r, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error reading object %q: %v", object, err)
	}
	defer r.Close()

	return fetchStream(r, a, path)
}

func fetchStream(r io.Reader, a Artifact, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	io.Copy(io.MultiWriter(file, hasher), r)
	checksum := fmt.Sprintf("%64x", hasher.Sum(nil))
	if !strings.EqualFold(checksum, a.checksum) {
		return fmt.Errorf("Checksum for artifact with id %q is %q expected %q", a.name, checksum, a.checksum)
	}
	return nil
}

func fetchViaHttp(ctx context.Context, a Artifact, path string) error {
	resp, err := newHttpClient().Get(a.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("todo")
	}

	return fetchStream(resp.Body, a, path)
}

type FakeArtifactRepository struct {
	AddedArtifacts []Artifact
	Artifacts      map[string]io.Reader
}

func (fake *FakeArtifactRepository) AddArtifact(new Artifact) error {
	_ = append(fake.AddedArtifacts, new)
	return nil
}

func (fake *FakeArtifactRepository) GetArtifact(name string) (io.Reader, func() error, error) {
	a, ok := fake.Artifacts[name]
	if !ok {
		return nil, nil, fmt.Errorf("Artifact with name %q doesn't exist", name)
	}
	return a, func() error { return nil }, nil
}
