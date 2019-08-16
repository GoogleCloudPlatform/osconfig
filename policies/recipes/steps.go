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
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
	"golang.org/x/sys/unix"
)

// StepFileCopy builds the command for a FileCopy step
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) error {
	dest, err := normPath(step.FileCopy.Destination)
	if err != nil {
		return err
	}

	permissions, err := parsePermissions(step.FileCopy.Permissions)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		// file exists
		if !step.FileCopy.Overwrite {
			return fmt.Errorf("file already exists at path %q and Overwrite = false", step.FileCopy.Destination)
		}
		os.Chmod(dest, permissions)
	}

	src, ok := artifacts[step.FileCopy.ArtifactId]
	if !ok {
		return fmt.Errorf("could not find location for artifact %q", step.FileCopy.ArtifactId)
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
		return 755, nil
	}

	i, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(i), nil
}

// StepArchiveExtraction builds the command for a ArchiveExtraction step
func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction, artifacts map[string]string, stepDir string) error {
	filename, ok := artifacts[step.ArchiveExtraction.GetArtifactId()]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", step.ArchiveExtraction.GetArtifactId())
	}
	switch step.ArchiveExtraction.GetType() {
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_ZIP:
		return extractZip(filename, step.ArchiveExtraction.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR:
		return extractTar(filename, step.ArchiveExtraction.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP:
		compressed, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer compressed.Close()
		reader, err := gzip.NewReader(compressed)
		if err != nil {
			return err
		}
		defer reader.Close()
		decompressed, err := ioutil.TempFile(stepDir, "archive-*.tar")
		if err != nil {
			return err
		}
		_, err = io.Copy(decompressed, reader)
		decompressed.Close()
		if err != nil {
			return err
		}
		return extractTar(decompressed.Name(), step.ArchiveExtraction.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP:
		compressed, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer compressed.Close()
		reader := bzip2.NewReader(compressed)
		if err != nil {
			return err
		}
		decompressed, err := ioutil.TempFile(stepDir, "archive-*.tar")
		if err != nil {
			return err
		}
		_, err = io.Copy(decompressed, reader)
		decompressed.Close()
		if err != nil {
			return err
		}
		return extractTar(decompressed.Name(), step.ArchiveExtraction.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA:
		compressed, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer compressed.Close()
		reader, err := lzma.NewReader2(compressed)
		if err != nil {
			return err
		}
		decompressed, err := ioutil.TempFile(stepDir, "archive-*.tar")
		if err != nil {
			return err
		}
		_, err = io.Copy(decompressed, reader)
		decompressed.Close()
		if err != nil {
			return err
		}
		return extractTar(decompressed.Name(), step.ArchiveExtraction.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ:
		compressed, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer compressed.Close()
		reader, err := xz.NewReader(compressed)
		if err != nil {
			return err
		}
		decompressed, err := ioutil.TempFile(stepDir, "archive-*.tar")
		if err != nil {
			return err
		}
		_, err = io.Copy(decompressed, reader)
		decompressed.Close()
		if err != nil {
			return err
		}
		return extractTar(decompressed.Name(), step.ArchiveExtraction.Destination)
	default:
		return fmt.Errorf("Unrecognized archive type %q", step.ArchiveExtraction.GetType())
	}
}

func zipIsDir(name string) bool {
	return strings.HasSuffix(name, string(os.PathSeparator))
}

func extractZip(zipPath string, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	// Check for conflicts
	for _, f := range zr.File {
		filen, err := normPath(filepath.Join(dst, f.FileHeader.Name))
		if err != nil {
			return err
		}
		fmt.Printf("checking if file %s exists\n", filen)
		stat, err := os.Stat(filen)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if zipIsDir(filen) && stat.IsDir() {
			// it's ok if directories already exist
			continue
		}
		return fmt.Errorf("file exists: %s", filen)
	}

	// Create dirs
	for _, f := range zr.File {
		filen, err := normPath(filepath.Join(dst, f.FileHeader.Name))
		if err != nil {
			return err
		}

		if !zipIsDir(filen) {
			continue
		}
		_, err = os.Stat(filen)
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		err = os.MkdirAll(filen, 0755)
		if err != nil {
			return err
		}
	}

	// Create files.
	for _, f := range zr.File {
		filen, err := normPath(filepath.Join(dst, f.Name))
		if err != nil {
			return err
		}
		if zipIsDir(filen) {
			continue
		}
		filedir := filepath.Dir(filen)
		_, err = os.Stat(filedir)
		if os.IsNotExist(err) {
			err = os.MkdirAll(filedir, 0755)
		}
		if err != nil {
			return err
		}
		fmt.Printf("os.Create %s\n", filen)
		reader, err := f.Open()
		if err != nil {
			return err
		}

		dst, err := os.OpenFile(filen, os.O_RDWR|os.O_CREATE, 0755)
		if err == nil {
			_, err = io.Copy(dst, reader)
		}
		reader.Close()

		if err != nil {
			return err
		}
		err = os.Chtimes(filen, time.Now(), f.Modified)
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTar(tarName string, dst string) error {
	file, err := os.Open(tarName)
	if err != nil {
		return err
	}
	tr := tar.NewReader(file)

	// Check for conflicts
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		filen, err := normPath(filepath.Join(dst, header.Name))
		if err != nil {
			return err
		}
		fmt.Printf("checking if file %s exists\n", filen)
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

	file.Seek(0, 0)
	tr = tar.NewReader(file)

	// Create dirs
	for {
		var err error
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeDir {
			continue
		}
		filen, err := normPath(filepath.Join(dst, header.Name))
		if err != nil {
			return err
		}
		_, err = os.Stat(filen)
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		err = os.MkdirAll(filen, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		err = os.Chown(filen, header.Uid, header.Gid)
		if err != nil {
			return err
		}
	}

	file.Seek(0, 0)
	tr = tar.NewReader(file)

	// Create files.
	for {
		var err error
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		filen, err := normPath(filepath.Join(dst, header.Name))
		if err != nil {
			return err
		}
		filedir := filepath.Dir(filen)
		_, err = os.Stat(filedir)
		if err != nil {
			err = os.MkdirAll(filedir, 0755)
		}
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeRegA:
			fmt.Printf("os.Create %s (owner %s/%d group %s/%d)\n", filen, header.Uname, header.Uid, header.Gname, header.Gid)
			dst, err := os.Create(filen)
			if err == nil {
				_, err = io.Copy(dst, tr)
			}
		case tar.TypeLink:
			err = os.Link(header.Linkname, filen)
			continue
		case tar.TypeSymlink:
			err = os.Symlink(header.Linkname, filen)
			continue
		case tar.TypeChar:
			err = unix.Mknod(filen, uint32(unix.S_IFCHR), int(unix.Mkdev(uint32(header.Devmajor), uint32(header.Devminor))))
		case tar.TypeBlock:
			err = unix.Mknod(filen, uint32(unix.S_IFBLK), int(unix.Mkdev(uint32(header.Devmajor), uint32(header.Devminor))))
		case tar.TypeFifo:
			err = unix.Mkfifo(filen, uint32(header.Mode))
		default:
			fmt.Printf("unknown type for %s\n", filen)
			continue
		}
		if err != nil {
			return err
		}
		err = os.Chmod(filen, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		err = os.Chown(filen, header.Uid, header.Gid)
		if err != nil {
			return err
		}
		err = os.Chtimes(filen, header.AccessTime, header.ModTime)
		if err != nil {
			return err
		}
	}
	return nil
}

// StepMsiInstallation builds the command for a MsiInstallation step
func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation, artifacts map[string]string) error {
	fmt.Println("StepMsiInstallation")
	return nil
}

// StepDpkgInstallation builds the command for a DpkgInstallation step
func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation, artifacts map[string]string) error {
	fmt.Println("StepDpkgInstallation")
	return nil
}

// StepRpmInstallation builds the command for a FileCopy step
func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation, artifacts map[string]string) error {
	fmt.Println("StepRpmInstallation")
	return nil
}

// StepFileExec builds the command for a FileExec step
func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var path string
	switch v := step.FileExec.LocationType.(type) {
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_LocalPath:
		path = v.LocalPath
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId:
		var ok bool
		path, ok = artifacts[v.ArtifactId]
		if !ok {
			return fmt.Errorf("%q not found in artifact map", v.ArtifactId)
		}
	default:
		return fmt.Errorf("can't determine location type")
	}

	return executeCommand(path, stepDir, runEnvs, step.FileExec.Args...)
}

// StepScriptRun builds the command for a ScriptRun step
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string, runEnvs []string, stepDir string) error {
	scriptPath := filepath.Join(stepDir, "script")

	f, err := os.Create(scriptPath)
	if err != nil {
		return err
	}

	f.WriteString(step.ScriptRun.Script)
	f.Close()
	if err := os.Chmod("/tmp/scriptrun", 0755); err != nil {
		return err
	}

	var qargs []string
	//if step.ScriptRun.Interpreter == osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL {
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	command := scriptPath + " " + strings.Join(qargs, " ")
	args := []string{"-c", command}
	return executeCommand("/bin/sh", stepDir, runEnvs, args...)
}

func executeCommand(cmd string, workDir string, runEnvs []string, args ...string) error {
	cmdObj := exec.Command(cmd, args...)

	cmdObj.Dir = workDir
	cmdObj.Env = append(cmdObj.Env, runEnvs...)

	// TODO: log output from command.
	_, err := cmdObj.Output()
	if err != nil {
		return err
	}
	return nil
}
