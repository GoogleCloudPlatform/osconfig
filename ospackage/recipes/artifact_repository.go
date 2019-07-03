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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/oswrap"

	"cloud.google.com/go/storage"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

var (
	bucketRegex     = `(P<bucket>[a-z0-9][-_.a-z0-9]*)`
	objectRegex     = `(P<object>[^#]+)`
	generationRegex = `(P<generation>.+)`

	// Many of the Google Storage URLs are supported below.
	// It is preferred that customers specify their object using
	// its gs://<bucket>/<object> URL.
	gsRegex = regexp.MustCompile(fmt.Sprintf(`^gs://%s/%s#?%s?$`, bucketRegex, objectRegex, generationRegex))
	// Check for the Google Storage URLs:
	// http://<bucket>.storage.googleapis.com/<object>
	// https://<bucket>.storage.googleapis.com/<object>
	gsHTTPRegex1 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://%s\.storage\.googleapis\.com/%s#?%s?$`, bucketRegex, objectRegex, generationRegex))
	// http://storage.cloud.google.com/<bucket>/<object>
	// https://storage.cloud.google.com/<bucket>/<object>
	gsHTTPRegex2 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://storage\.cloud\.google\.com/%s/%s#?%s?$`, bucketRegex, objectRegex, generationRegex))
	// Check for the other possible Google Storage URLs:
	// http://storage.googleapis.com/<bucket>/<object>
	// https://storage.googleapis.com/<bucket>/<object>
	//
	// The following are deprecated but checked:
	// http://commondatastorage.googleapis.com/<bucket>/<object>
	// https://commondatastorage.googleapis.com/<bucket>/<object>
	gsHTTPRegex3 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://(?:commondata)?storage\.googleapis\.com/%s/%s#?%s?$`, bucketRegex, objectRegex, generationRegex))

	gcsAPIBase = "https://storage.cloud.google.com"

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

type objectLocation struct {
	bucket string
	path string
	generation string
}

func tryGcsRegex(r *regexp.Regexp, url string) (objectLocation, bool) {
	matches := gsHTTPRegex1.FindStringSubmatch(url)
	if len(matches) == 3 {
		return objectLocation{bucket: matches[1], path: matches[2]}, true
	}
	if len(matches) == 4 {
		return objectLocation{bucket: matches[1], path: matches[2], generation: matches[3]}, true
	}
	return objectLocation{}, false
}

func tryParseGcsURL(url string) (objectLocation, bool) {
	if loc, ok := tryGcsRegex(gsHTTPRegex1, url); ok {
		return loc, true
	}
	if loc, ok = tryGcsRegex(gsHTTPRegex2, url); ok {
		return loc, true
	}
	if loc, ok = tryGcsRegex(gsHTTPRegex2, url); ok {
		return loc, true
	}
	return objectLocation{}, false
}

func fetchArtifact(ctx context.Context, artifact *osconfigpb.SoftwareRecipe_Artifact, directory string) (string, error) {
	path := path.Join(directory, artifact.Id)
	uri, err := url.Parse(artifact.Uri)
	if err != nil {
		return "", fmt.Errorf("Could not parse url %q for artifact %q", artifact.Uri, artifact.Id)
	}
	scheme := strings.ToLower(uri.Scheme)

	switch scheme {
	case "gcs":
		loc, ok := tryGcsRegex(gsRegex, artifact.Uri)
		if !ok {
			return "", fmt.Errorf("Could not parse gs url %q for artifact %q", artifact.Uri, artifact.Id)
		}
		err := fetchFromGCS(ctx, artifact, loc, path)
		if err != nil {
			return "", err
		}
	case "https", "http":
		loc, ok  := tryParseGcsURL(artifact.Uri)

		if ok {
			err = fetchFromGCS(ctx, artifact, loc, path)
			if err != nil {
				return "", err
			}
		} else {
			err := fetchViaHTTP(ctx, artifact, path)
			if err != nil {
				return "", err
			}
		}
	default:
		return "", fmt.Errorf("protocol %q found in artifact %q not supported", scheme, artifact.Id)
	}

	return path, nil
}

func fetchFromGCS(ctx context.Context, artifact *osconfigpb.SoftwareRecipe_Artifact, loc objectLocation, path string) error {
	client, err := newStorageClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	oh := client.Bucket(uri.Hostname()).Object(uri.Path)
	if uri.Fragment != "" {
		gen, err := strconv.ParseInt(uri.Fragment, 10, 64)
		if err != nil {
			return fmt.Errorf("couldn't parse gcs generation number %q for artifact %q", uri.Fragment, artifact.Id)
		}
		oh = oh.Generation(gen)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error reading gcs artifact %q: %v", artifact.Id, err)
	}
	defer r.Close()

	return fetchStream(r, artifact, path)
}

func fetchViaHTTP(ctx context.Context, artifact *osconfigpb.SoftwareRecipe_Artifact, path string) error {
	resp, err := newHTTPClient().Get(artifact.Uri)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("When downloading artifact %q got http status %d", artifact.Id, resp.StatusCode)
	}

	return fetchStream(resp.Body, artifact, path)
}

func fetchStream(r io.Reader, a *osconfigpb.SoftwareRecipe_Artifact, path string) error {
	file, err := oswrap.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	_, err = io.Copy(io.MultiWriter(file, hasher), r)
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%64x", hasher.Sum(nil))
	if a.Checksum != "" && !strings.EqualFold(checksum, a.Checksum) {
		return fmt.Errorf("Checksum for artifact with id %q is %q expected %q", a.Id, checksum, a.Checksum)
	}
	return nil
}
