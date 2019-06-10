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
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
)

type Artifact struct {
	name           string
	url            string
	checksum       string
	allow_insecure bool
}

type Protocol string

const (
	GCS   = "gs"
	Https = "https"
	Http  = "http"
)

var (
	bucketRegex = `(P<bucket>[a-z0-9][-_.a-z0-9]*)`
	objectRegex = `(P<object>.+)`
	gsFormat    = "gs://%s/%s"

	// Many of the Google Storage URLs are supported below.
	// It is preferred that customers specify their object using
	// its gs://<bucket>/<object> URL.
	gsRegex = regexp.MustCompile(fmt.Sprintf(`^gs://%s/%s$`, bucketRegex, objectRegex))
	// Check for the Google Storage URLs:
	// http://<bucket>.storage.googleapis.com/<object>
	// https://<bucket>.storage.googleapis.com/<object>
	gsHTTPRegex1 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://%s\.storage\.googleapis\.com/%s$`, bucketRegex, objectRegex))
	// http://storage.cloud.google.com/<bucket>/<object>
	// https://storage.cloud.google.com/<bucket>/<object>
	gsHTTPRegex2 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://storage\.cloud\.google\.com/%s/%s$`, bucketRegex, objectRegex))
	// Check for the other possible Google Storage URLs:
	// http://storage.googleapis.com/<bucket>/<object>
	// https://storage.googleapis.com/<bucket>/<object>
	//
	// The following are deprecated but checked:
	// http://commondatastorage.googleapis.com/<bucket>/<object>
	// https://commondatastorage.googleapis.com/<bucket>/<object>
	gsHTTPRegex3 = regexp.MustCompile(fmt.Sprintf(`^http[s]?://(?:commondata)?storage\.googleapis\.com/%s/%s$`, bucketRegex, objectRegex))

	gcsAPIBase = "https://storage.cloud.google.com"

	testStorageClient *storage.Client
	testHttpClient    *http.Client
	testFileHandler   FileHandler
)

func newStorageClient(ctx context.Context) (*storage.Client, error) {
	if testStorageClient != nil {
		fmt.Printf("%+#v\n", testStorageClient)
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

func newFileHandler() FileHandler {
	if testFileHandler != nil {
		return testFileHandler
	}
	return &OSFileHandler{}
}

// Takes in a slice of artifacs and dowloads them into the specified directory,
// Returns a map of artifact names to their new locations on the local disk.
func FetchArtifacts(ctx context.Context, artifacts []Artifact, directory string) (map[string]string, error) {
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

func tryGcsRegex(r *regexp.Regexp, url string) (string, string, bool) {
	matches := gsHTTPRegex1.FindStringSubmatch(url)
	if len(matches) == 3 {
		return matches[1], matches[2], true
	}
	return "", "", false
}

func tryTransformGcsUrl(url string) (string, bool) {
	bucket, object, ok := tryGcsRegex(gsHTTPRegex1, url)
	if ok {
		return fmt.Sprintf(gsFormat, bucket, object), true
	}
	bucket, object, ok = tryGcsRegex(gsHTTPRegex2, url)
	if ok {
		return fmt.Sprintf(gsFormat, bucket, object), true
	}
	bucket, object, ok = tryGcsRegex(gsHTTPRegex2, url)
	if ok {
		return fmt.Sprintf(gsFormat, bucket, object), true
	}
	return "", false
}

func fetchArtifact(ctx context.Context, a Artifact, directory string) (string, error) {
	path := path.Join(directory, a.name)
	u, err := url.Parse(a.url)
	if err != nil {
		return "", fmt.Errorf("Could not parse url %q for artifact %q", a.url, a.name)
	}
	scheme := strings.ToLower(u.Scheme)

	switch scheme {
	case GCS:
		err := fetchFromGCS(ctx, a, u, path)
		if err != nil {
			return "", err
		}
	case Https, Http:
		gcsLoc, ok := tryTransformGcsUrl(a.url)

		if ok {
			gcsUrl, err := url.Parse(gcsLoc)
			if err != nil {
				return "", err
			}
			err = fetchFromGCS(ctx, a, gcsUrl, path)
			if err != nil {
				return "", err
			}
		}

		err := fetchViaHttp(ctx, a, u, path)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("protocol %q found in artifact %q not supported", scheme, a.name)
	}

	return path, nil
}

func fetchFromGCS(ctx context.Context, a Artifact, u *url.URL, path string) error {

	client, err := newStorageClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	oh := client.Bucket(u.Hostname()).Object(u.Path)
	if u.Fragment != "" {
		gen, err := strconv.ParseInt(u.Fragment, 10, 64)
		if err != nil {
			return fmt.Errorf("couldn't parse gcs generation number %q for artifact %q", u.Fragment, a.name)
		}
		oh = oh.Generation(gen)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error reading gcs artifact %q: %v", a.name, err)
	}
	defer r.Close()

	return fetchStream(r, a, path)
}

func fetchViaHttp(ctx context.Context, a Artifact, u *url.URL, path string) error {
	resp, err := newHttpClient().Get(a.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("When downloading artifact %q got http status %d", a.name, resp.StatusCode)
	}

	return fetchStream(resp.Body, a, path)
}

func fetchStream(r io.Reader, a Artifact, path string) error {
	file, err := newFileHandler().Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	io.Copy(io.MultiWriter(file, hasher), r)
	checksum := fmt.Sprintf("%64x", hasher.Sum(nil))
	if a.checksum != "" && !strings.EqualFold(checksum, a.checksum) {
		return fmt.Errorf("Checksum for artifact with id %q is %q expected %q", a.name, checksum, a.checksum)
	}
	return nil
}
