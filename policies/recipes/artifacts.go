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
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"

	"github.com/GoogleCloudPlatform/osconfig/util"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

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
	var reader io.ReadCloser
	var checksum, extension string
	switch v := artifact.Artifact.(type) {
	case *osconfigpb.SoftwareRecipe_Artifact_Gcs_:
		extension = path.Ext(v.Gcs.Object)
		reader, err := util.FetchWithGCS(ctx, v.Gcs.Bucket, v.Gcs.Object, v.Gcs.Generation)
		if err != nil {
			return "", fmt.Errorf("error fetching artifact %q from GCS: %v", artifact.Id, err)
		}
		defer reader.Close()
	case *osconfigpb.SoftwareRecipe_Artifact_Remote_:
		uri, err := url.Parse(v.Remote.Uri)
		if err != nil {
			return "", fmt.Errorf("Could not parse url %q for artifact %q", v.Remote.Uri, artifact.Id)
		}
		extension = path.Ext(uri.Path)
		if uri.Scheme != "http" && uri.Scheme != "https" {
			return "", fmt.Errorf("error, artifact %q has unsupported protocol scheme %s", artifact.Id, uri.Scheme)
		}
		checksum = v.Remote.Checksum
		response, err := util.FetchWithHTTP(ctx, v.Remote.Uri)
		if err != nil {
			return "", fmt.Errorf("error fetching artifact %q with http or https: %v", artifact.Id, err)
		}
		defer response.Body.Close()
		reader = response.Body
	default:
		return "", fmt.Errorf("unknown artifact type %T", v)
	}

	localPath := filepath.Join(directory, artifact.Id)
	if extension != "" {
		localPath = localPath + extension
	}
	err := util.DownloadStream(reader, checksum, localPath)
	if err != nil {
		return "", err
	}
	return localPath, nil
}
