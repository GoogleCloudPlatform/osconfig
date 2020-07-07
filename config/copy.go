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

package config

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/GoogleCloudPlatform/osconfig/util"
	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

func copyFile(step *agentendpointpb.SoftwareRecipe_Step_CopyFile, artifacts map[string]string, runEnvs []string, stepDir string) error {
	dest, err := util.NormPath(step.Destination)
	if err != nil {
		return err
	}

	permissions, err := parsePermissions(step.Permissions)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		// file exists
		if !step.Overwrite {
			return fmt.Errorf("file already exists at path %q and Overwrite = false", step.Destination)
		}
		os.Chmod(dest, permissions)
	}

	artifact := step.GetArtifactId()
	src, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("could not find location for artifact %q", artifact)
	}

	reader, err := os.Open(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	writer, err := os.OpenFile(dest, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, permissions)
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, reader)
	if err != nil {
		return err
	}

	return nil
}

func parsePermissions(s string) (os.FileMode, error) {
	if s == "" {
		return 0755, nil
	}

	i, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(i), nil
}
