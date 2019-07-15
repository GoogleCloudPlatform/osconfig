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
	"archive/zip"
	"strings"
	"path/filepath"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
)

type CopyFile struct {
	artifactId string
	dest string
	overwrite bool
	permissions string
}
 
func doCopyFile(step CopyFile, artifacts map[string]string) error {
	fh := newFileHandler()
	permissions, err := parsePermissions(step.permissions)
	if err != nil {
		return err
	}

	exists, err := fh.Exists(step.dest)
	if err != nil {
		return err
	}
	if exists {
		if !step.overwrite {
			logger.Infof("skipping FileCopy step as file at %s already exists", step.dest)
			return nil
		}
	}

	src, ok := artifacts[step.artifactId]
	if !ok {
		return fmt.Errorf("Could not find location for artifact %q in CopyFile step", step.artifactId)
	}
	reader, err := fh.Open(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	writer, err := fh.OpenFile(step.dest, os.O_TRUNC | os.O_WRONLY | os.O_CREATE, permissions)
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
		return 755, nil
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if i < 0 || i > 777 {
		return 0, fmt.Errorf("in CopyFile step expected permissions to be between 0 and 777, was %s", s)
	}
	if i % 10 > 7  || i % 100 / 10 > 7 || i % 1000 / 100 > 7{
		return 0, fmt.Errorf("in CopyFile step expected all digits in permission to be octal digits from 0-7, was %s", s)
	}
	return os.FileMode(i), nil
}

type ArchiveType string

type ExtractArchive struct {
	artifactId string
	destination string
	archiveType ArchiveType
}

const (
	ArchiveTypeUnspecified ArchiveType = ""
	Zip ArchiveType = "zip"
	Tar ArchiveType = "tar"
	TarGzip ArchiveType = "tar.gz"
	TarBzip ArchiveType = "tar.bzip"
	TarLzma ArchiveType = "tar.lzma"
	TarXz ArchiveType = "tar.xz"
)

func doExtractArchive(step ExtractArchive, artifacts map[string]string) error {
	source, ok := artifacts[step.artifactId]
	if !ok {
		return fmt.Errorf("ExtractArchive step couldn't find artifact %q", step.artifactId)
	}
	
	switch step.archiveType {
	case Zip:
		doZip(source, step.destination)
	case Tar:
		f, err := Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		doTar(f, step.Destination)
	case TarGzip:
		f, err := Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		reader := gzip.NewReader(f)
		doTar(reader, step.Destination)
	case TarBzip:
		f, err := Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		reader := bzip2.NewReader(f)
		doTar(reader, step.Destination)
	case TarLzma:
	case TarXz:
	case ArchiveTypeUnspecified:
		return fmt.Errorf("ExtractArchive step received unspecified archive type")
	default:
		return fmt.Errorf("ExtractArchive step didnt recognize archive type %q", step.archiveType)
	}
	return nil
}

func doZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(path, filepath.Clean(dest) + string(os.PathSeparator)) {
			return fmt.Errorf("invalid path %s in zip", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}
		err = createFileZip(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func handleFileZip(f zip.File, path string) error {
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	reader, err := f.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	if _, err = io.Copy(out, reader); err != nil {
		return err
	}
	return nil
}

func doTar(in io.Reader, dest string) error {
	reader := tar.NewReader(in)
	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF
			{
				return nil
			}
			return err
		}

		path := filepath.Join(dest, header.Name)

		if !strings.HasPrefix(path, filepath.Clean(dest) + string(os.PathSeparator)) {
			return fmt.Errorf("invalid path %s in zip", f.Name)
		}

		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(path, os.ModePerm)
			continue
		}
		err := createTarFile(reader, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func createTarFile(reader io.Reader, path string) {
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.Mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, reader); err != nil {
		return err
	}
	return nil
}