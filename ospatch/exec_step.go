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

package ospatch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"cloud.google.com/go/storage"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/common"
	"github.com/GoogleCloudPlatform/osconfig/external"
)

var getGCSClient = func(ctx context.Context) (*storage.Client, error) {
	return storage.NewClient(ctx)
}

func getExecutablePath(ctx context.Context, logger *common.Logger, stepConfig *osconfigpb.ExecStepConfig) (string, error) {
	if gcsObject := stepConfig.GetGcsObject(); gcsObject != nil {
		var reader io.ReadCloser
		cl, err := getGCSClient(ctx)
		if err != nil {
			return "", fmt.Errorf("error creating gcs client: %v", err)
		}
		gf := &external.GCS_fetcher{Client: cl, Bucket: gcsObject.Bucket, Object: gcsObject.Object, Generation: gcsObject.GenerationNumber}
		if err != nil {
			return "", fmt.Errorf("error reading GCS object: %s", err)
		}
		reader, err = gf.Fetch(ctx)
		defer reader.Close()
		logger.Debugf("Fetched GCS object bucket %s object %s generation number %d", gcsObject.GetBucket(), gcsObject.GetObject(), gcsObject.GetGenerationNumber())

		localPath := filepath.Join(os.TempDir(), path.Base(gcsObject.GetObject()))
		if err := downloadFile(logger, reader, localPath); err != nil {
			return "", err
		}
		return localPath, nil
	}

	return stepConfig.GetLocalPath(), nil
}

func executeCommand(logger *common.Logger, path string, exitCodes []int32, args ...string) error {
	logger.Debugf("Running command %s with args %s", path, args)
	cmdObj := exec.Command(path, args...)

	stdoutStderr, err := cmdObj.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			allowedCodes := append(exitCodes, 0)
			for _, code := range allowedCodes {
				if int32(exitErr.ExitCode()) == code {
					return nil
				}
			}
		}
		return err
	}
	logger.Infof("%s\n", stdoutStderr)
	return nil
}

func downloadFile(logger *common.Logger, reader io.ReadCloser, localPath string) error {
	if err := external.DownloadStream(reader, "", localPath); err != nil {
		return fmt.Errorf("error downloading GCS object: %s", err)
	}
	if err := os.Chmod(localPath, 0755); err != nil {
		return fmt.Errorf("error making file executable: %s", err)
	}
	logger.Debugf("Downloaded to local path %s", localPath)
	return nil
}
