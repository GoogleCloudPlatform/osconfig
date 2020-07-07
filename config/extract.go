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
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

func extractArchive(step *agentendpointpb.SoftwareRecipe_Step_ExtractArchive, artifacts map[string]string, runEnvs []string, stepDir string) error {
	artifact := step.GetArtifactId()
	filename, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", artifact)
	}
	switch typ := step.GetType(); typ {
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_ZIP:
		return extractZip(filename, step.Destination)
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP,
		agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP,
		agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA,
		agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ,
		agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR:
		return extractTar(filename, step.Destination, typ)
	default:
		return fmt.Errorf("Unrecognized archive type %q", typ)
	}
}

func zipIsDir(name string) bool {
	if os.PathSeparator == '\\' {
		return strings.HasSuffix(name, `\`) || strings.HasSuffix(name, "/")
	}
	return strings.HasSuffix(name, "/")
}

func extractZip(zipPath string, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	// Check for conflicts
	for _, f := range zr.File {
		filen, err := util.NormPath(filepath.Join(dst, f.Name))
		if err != nil {
			return err
		}
		stat, err := os.Stat(filen)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if zipIsDir(f.Name) && stat.IsDir() {
			// it's ok if directories already exist
			continue
		}
		return fmt.Errorf("file exists: %s", filen)
	}

	// Create files.
	for _, f := range zr.File {
		filen, err := util.NormPath(filepath.Join(dst, f.Name))
		if err != nil {
			return err
		}
		if zipIsDir(f.Name) {
			mode := f.Mode()
			if mode == 0 {
				mode = 0755
			}
			if err := os.MkdirAll(filen, mode); err != nil {
				return err
			}
			// Setting to correct permissions in case the directory has already been created
			if err := os.Chmod(filen, mode); err != nil {
				return err
			}
			continue
		}
		filedir := filepath.Dir(filen)
		if err = os.MkdirAll(filedir, 0755); err != nil {
			return err
		}
		reader, err := f.Open()
		if err != nil {
			return err
		}

		mode := f.Mode()
		if mode == 0 {
			mode = 0755
		}

		dst, err := os.OpenFile(filen, os.O_RDWR|os.O_CREATE, mode)
		if err != nil {
			return err
		}
		if _, err = io.Copy(dst, reader); err != nil {
			dst.Close()
			return err
		}

		reader.Close()
		if err := dst.Close(); err != nil {
			return err
		}

		if err := os.Chtimes(filen, time.Now(), f.ModTime()); err != nil {
			return err
		}
	}
	return nil
}

func decompress(reader io.Reader, archiveType agentendpointpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType) (io.Reader, error) {
	switch archiveType {
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP:
		// *gzip.Reader is a io.ReadCloser but it isn't necesary to call Close() on it.
		return gzip.NewReader(reader)
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP:
		return bzip2.NewReader(reader), nil
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA:
		return lzma.NewReader2(reader)
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ:
		return xz.NewReader(reader)
	case agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR:
		return reader, nil
	default:
		return nil, fmt.Errorf("Unrecognized archive type %q when trying to decompress tar", archiveType)
	}
}

func checkForConflicts(tr *tar.Reader, dst string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		filen, err := util.NormPath(filepath.Join(dst, header.Name))
		if err != nil {
			return err
		}
		stat, err := os.Stat(filen)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeDir && stat.IsDir() {
			// it's ok if directories already exist
			continue
		}
		return fmt.Errorf("file exists: %s", filen)
	}
	return nil
}

func extractTar(tarName string, dst string, archiveType agentendpointpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType) error {
	file, err := os.Open(tarName)
	if err != nil {
		return err
	}
	defer file.Close()

	decompressed, err := decompress(file, archiveType)
	if err != nil {
		return err
	}
	tr := tar.NewReader(decompressed)

	if err := checkForConflicts(tr, dst); err != nil {
		return err
	}

	file.Seek(0, 0)
	decompressed, err = decompress(file, archiveType)
	if err != nil {
		return err
	}
	tr = tar.NewReader(decompressed)

	for {
		var err error
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		filen, err := util.NormPath(filepath.Join(dst, header.Name))
		if err != nil {
			return err
		}
		filedir := filepath.Dir(filen)

		if err := os.MkdirAll(filedir, 0700); err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filen, os.FileMode(header.Mode)); err != nil {
				return err
			}
			// Setting to correct permissions in case the directory has already been created
			if err := os.Chmod(filen, os.FileMode(header.Mode)); err != nil {
				return err
			}
			if err := chown(filen, header.Uid, header.Gid); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			dst, err := os.OpenFile(filen, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(dst, tr); err != nil {
				dst.Close()
				return err
			}
			if err := dst.Close(); err != nil {
				return err
			}
		case tar.TypeLink:
			if err := os.Link(header.Linkname, filen); err != nil {
				return err
			}
			continue
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, filen); err != nil {
				return err
			}
			continue
		case tar.TypeChar:
			if err := mkCharDevice(filen, uint32(header.Devmajor), uint32(header.Devminor)); err != nil {
				return err
			}
			if runtime.GOOS != "windows" {
				if err := os.Chmod(filen, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeBlock:
			if err := mkBlockDevice(filen, uint32(header.Devmajor), uint32(header.Devminor)); err != nil {
				return err
			}
			if runtime.GOOS != "windows" {
				if err := os.Chmod(filen, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeFifo:
			if err := mkFifo(filen, uint32(header.Mode)); err != nil {
				return err
			}
		default:
			logger.Infof("unknown file type for tar entry %s\n", filen)
			continue
		}
		if err := chown(filen, header.Uid, header.Gid); err != nil {
			return err
		}
		if err := os.Chtimes(filen, header.AccessTime, header.ModTime); err != nil {
			return err
		}
	}

	return nil
}

func chown(file string, uid, gid int) error {
	// os.Chown unsupported on windows
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chown(file, uid, gid)
}
