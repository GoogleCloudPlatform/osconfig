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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/util"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

const defaultFilePerms = 0644

type fileResource struct {
	*agentendpointpb.OSPolicy_Resource_FileResource

	managedFile ManagedFile
}

// ManagedFile is the file that this FileResouce manages.
type ManagedFile struct {
	Path       string
	State      agentendpointpb.OSPolicy_Resource_FileResource_DesiredState
	Permisions os.FileMode

	tempDir  string
	source   string
	checksum string
}

func parsePermissions(s string) (os.FileMode, error) {
	if s == "" {
		return defaultFilePerms, nil
	}

	i, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(i), nil
}

// TODO: use a persistent cache for downloaded files so we dont need to redownload them each time.
func (f *fileResource) download(ctx context.Context) error {
	// No need to download if source is a local file.
	if f.GetFile().GetLocalPath() != "" {
		return nil
	}

	tmpDir, err := ioutil.TempDir("", "osconfig_file_resource_")
	if err != nil {
		return fmt.Errorf("failed to create working dir: %s", err)
	}
	f.managedFile.tempDir = tmpDir

	tmpFile := filepath.Join(tmpDir, filepath.Base(f.GetPath()))
	f.managedFile.source = tmpFile

	switch f.GetSource().(type) {
	case *agentendpointpb.OSPolicy_Resource_FileResource_Content:
		f.managedFile.checksum, err = util.AtomicWriteFileStream(strings.NewReader(f.GetContent()), "", tmpFile, 0644)
		if err != nil {
			return err
		}

	case *agentendpointpb.OSPolicy_Resource_FileResource_File:
		f.managedFile.checksum, err = downloadFile(ctx, tmpFile, f.GetFile())
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognized Source type for FileResource: %q", f.GetSource())
	}

	return nil
}

func (f *fileResource) validate(ctx context.Context) (*ManagedResources, error) {
	switch f.GetState() {
	case agentendpointpb.OSPolicy_Resource_FileResource_ABSENT, agentendpointpb.OSPolicy_Resource_FileResource_PRESENT, agentendpointpb.OSPolicy_Resource_FileResource_CONTENTS_MATCH:
		f.managedFile.State = f.GetState()
	default:
		return nil, fmt.Errorf("unrecognized DesiredState for FileResource: %q", f.GetState())
	}

	f.managedFile.Path = f.GetPath()

	// If desired state is absent, we can return now.
	if f.GetState() == agentendpointpb.OSPolicy_Resource_FileResource_ABSENT {
		return &ManagedResources{Files: []ManagedFile{f.managedFile}}, nil
	}

	perms, err := parsePermissions(f.GetPermissions())
	if err != nil {
		return nil, fmt.Errorf("can't parse permissions %q: %v", f.GetPermissions(), err)
	}
	f.managedFile.Permisions = perms

	if f.GetFile().GetLocalPath() != "" {
		f.managedFile.source = f.GetFile().GetLocalPath()
		file, err := os.Open(f.GetFile().GetLocalPath())
		if err != nil {
			return nil, err
		}
		f.managedFile.checksum = checksum(file)
		file.Close()
	}

	switch f.managedFile.State {
	case agentendpointpb.OSPolicy_Resource_FileResource_ABSENT:
	case agentendpointpb.OSPolicy_Resource_FileResource_PRESENT:
		// If the file is already present no need to downloaded it.
		if !util.Exists(f.managedFile.Path) {
			if err := f.download(ctx); err != nil {
				return nil, err
			}
		}
	case agentendpointpb.OSPolicy_Resource_FileResource_CONTENTS_MATCH:
		if err := f.download(ctx); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unrecognized DesiredState for FileResource: %q", f.managedFile.State)
	}

	return &ManagedResources{Files: []ManagedFile{f.managedFile}}, nil
}

func (f *fileResource) checkState(ctx context.Context) (inDesiredState bool, err error) {
	switch f.managedFile.State {
	case agentendpointpb.OSPolicy_Resource_FileResource_ABSENT:
		return !util.Exists(f.managedFile.Path), nil
	case agentendpointpb.OSPolicy_Resource_FileResource_PRESENT:
		return util.Exists(f.managedFile.Path), nil
	case agentendpointpb.OSPolicy_Resource_FileResource_CONTENTS_MATCH:
		return contentsMatch(f.managedFile.Path, f.managedFile.checksum)
	default:
		return false, fmt.Errorf("unrecognized DesiredState for FileResource: %q", f.managedFile.State)
	}
}

func copyFile(dst, src string, perms os.FileMode) (retErr error) {
	reader, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %v", err)
	}
	defer reader.Close()
	writer, err := os.OpenFile(dst, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, perms)
	if err != nil {
		return fmt.Errorf("error opening destination file: %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			if retErr == nil {
				retErr = fmt.Errorf("error closing destination file: %v", err)
			}
		}
	}()

	if _, err := io.Copy(writer, reader); err != nil {
		return err
	}

	return writer.Chmod(perms)
}

func (f *fileResource) enforceState(ctx context.Context) (inDesiredState bool, err error) {
	switch f.managedFile.State {
	case agentendpointpb.OSPolicy_Resource_FileResource_ABSENT:
		if err := os.Remove(f.managedFile.Path); err != nil {
			return false, fmt.Errorf("error removing %q: %v", f.managedFile.Path, err)
		}
	case agentendpointpb.OSPolicy_Resource_FileResource_PRESENT, agentendpointpb.OSPolicy_Resource_FileResource_CONTENTS_MATCH:
		// Download now if for some reason we got this point and have not.
		if f.managedFile.source == "" {
			if err := f.download(ctx); err != nil {
				return false, err
			}
		}
		if err := copyFile(f.managedFile.Path, f.managedFile.source, f.managedFile.Permisions); err != nil {
			return false, fmt.Errorf("error copying %q to %q: %v", f.managedFile.source, f.managedFile.Path, err)
		}
	default:
		return false, fmt.Errorf("unrecognized DesiredState for FileResource: %q", f.managedFile.State)
	}

	return true, nil
}

func (f *fileResource) cleanup(ctx context.Context) error {
	if f.managedFile.tempDir != "" {
		return os.RemoveAll(f.managedFile.tempDir)
	}
	return nil
}
