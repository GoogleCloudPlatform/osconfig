//  Copyright 2020 Google Inc. All Rights Reserved.
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

package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/osconfig/external"
	"github.com/GoogleCloudPlatform/osconfig/util"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

func checksum(r io.Reader) string {
	hash := sha256.New()
	io.Copy(hash, r)
	return hex.EncodeToString(hash.Sum(nil))
}

func downloadFile(ctx context.Context, path string, file *agentendpointpb.OSPolicy_Resource_File) (string, error) {
	var reader io.ReadCloser
	var err error
	var wantChecksum string

	switch file.GetType().(type) {
	case *agentendpointpb.OSPolicy_Resource_File_Gcs_:
		client, err := storage.NewClient(ctx)
		if err != nil {
			return "", fmt.Errorf("error creating gcs client: %v", err)
		}
		defer client.Close()

		reader, err = external.FetchGCSObject(ctx, client, file.GetGcs().GetBucket(), file.GetGcs().GetObject(), file.GetGcs().GetGeneration())
		if err != nil {
			return "", err
		}

	case *agentendpointpb.OSPolicy_Resource_File_Remote_:
		reader, err = external.FetchRemoteObjectHTTP(&http.Client{}, file.GetRemote().GetUri())
		if err != nil {
			return "", err
		}
		wantChecksum = file.GetRemote().GetSha256Checksum()

	default:
		return "", fmt.Errorf("unknown remote File type: %+v", file.GetType())
	}
	defer reader.Close()
	return util.AtomicWriteFileStream(reader, wantChecksum, path, 0644)
}
