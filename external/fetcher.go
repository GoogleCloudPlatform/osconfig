//  Copyright 2018 Google Inc. All Rights Reserved.
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

// Package external is responsible for all the external interactions
package external

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/osconfig/clog"
)

// FetchGCSObject fetches data from GCS bucket
func FetchGCSObject(ctx context.Context, client *storage.Client, bucket, object string, generation int64) (io.ReadCloser, error) {
	clog.Debugf(ctx, "Fetching GCS object: '%s/%s', generation: '%d", bucket, object, generation)
	oh := client.Bucket(bucket).Object(object)
	if generation != 0 {
		oh = oh.Generation(generation)
	}

	return oh.NewReader(ctx)
}

// FetchRemoteObjectHTTP fetches data from remote location
func FetchRemoteObjectHTTP(ctx context.Context, client *http.Client, url string) (io.ReadCloser, error) {
	clog.Debugf(ctx, "Fetching remote object: '%s'", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http status %d when attempting to download artifact", resp.StatusCode)
	}

	return resp.Body, nil
}
