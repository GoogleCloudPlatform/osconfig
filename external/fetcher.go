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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

// FetchGCSObject fetches data from GCS bucket
func FetchGCSObject(ctx context.Context, client *storage.Client, object, bucket string, generation int64) (io.ReadCloser, error) {
	oh := client.Bucket(bucket).Object(object)
	if generation != 0 {
		oh = oh.Generation(generation)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// FetchRemoteObjectHTTP fetches data from remote location
func FetchRemoteObjectHTTP(client *http.Client, url string) (io.ReadCloser, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http status %d when attempting to download artifact", resp.StatusCode)
	}

	return resp.Body, nil
}

// DownloadStream fetches data from an input stream
func DownloadStream(r io.ReadCloser, checksum, localPath string) error {
	localPath, err := util.NormPath(localPath)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
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
