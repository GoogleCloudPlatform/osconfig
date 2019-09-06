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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/common"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

// StepFileCopy builds the command for a FileCopy step
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) error {
	dest, err := common.NormPath(step.FileCopy.Destination)
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
func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction, artifacts map[string]string) error {
	filename, ok := artifacts[step.ArchiveExtraction.GetArtifactId()]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", step.ArchiveExtraction.GetArtifactId())
	}
	switch step.ArchiveExtraction.GetType() {
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_ZIP:
		return extractZip(filename, step.ArchiveExtraction.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR:
		return extractTar(filename, step.ArchiveExtraction.Destination, step.ArchiveExtraction.GetType())
	default:
		return fmt.Errorf("Unrecognized archive type %q", step.ArchiveExtraction.GetType())
	}
}

func zipIsDir(name string) bool {
	return strings.HasSuffix(name, string(os.PathSeparator))
}

func normalizeSlashes(s string) string {
	if os.PathSeparator != '/' {
		return strings.Replace(s, "/", string(os.PathSeparator), -1)
	}
	return s
}

func extractZip(zipPath string, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	// Check for conflicts
	for _, f := range zr.File {
		filen, err := common.NormPath(filepath.Join(dst, normalizeSlashes(f.Name)))
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
		if zipIsDir(f.Name) && stat.IsDir() {
			// it's ok if directories already exist
			continue
		}
		return fmt.Errorf("file exists: %s", filen)
	}

	// Create files.
	for _, f := range zr.File {
		filen, err := common.NormPath(filepath.Join(dst, normalizeSlashes(f.Name)))
		if err != nil {
			return err
		}
		if zipIsDir(f.Name) {
			mode := f.Mode()
			if mode == 0 {
				mode = 0755
			}
			if err = os.MkdirAll(filen, mode); err != nil {
				return err
			}
			// Setting to correct permissions in case the directory has already been created
			if err = os.Chmod(filen, mode); err != nil {
				return err
			}
			continue
		}
		filedir := filepath.Dir(filen)
		if err = os.MkdirAll(filedir, 0755); err != nil {
			return err
		}
		fmt.Printf("os.Create %s\n", filen)
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
		_, err = io.Copy(dst, reader)
		dst.Close()
		reader.Close()
		if err != nil {
			return err
		}
		err = os.Chtimes(filen, time.Now(), f.ModTime())
		if err != nil {
			return err
		}
	}
	return nil
}

func decompress(reader io.Reader, archiveType osconfigpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType) (io.Reader, error) {
	switch archiveType {
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP:
		// *gzip.Reader is a io.ReadCloser but it isn't necesary to call Close() on it.
		return gzip.NewReader(reader)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP:
		return bzip2.NewReader(reader), nil
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA:
		return lzma.NewReader2(reader)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ:
		return xz.NewReader(reader)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR:
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
		filen, err := common.NormPath(filepath.Join(dst, header.Name))
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
	return nil
}

func createFiles(tr *tar.Reader, dst string) error {
	for {
		var err error
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		filen, err := common.NormPath(filepath.Join(dst, header.Name))
		if err != nil {
			return err
		}
		filedir := filepath.Dir(filen)
		err = os.MkdirAll(filedir, 0700)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(filen, os.FileMode(header.Mode)); err != nil {
				return err
			}
			// Setting to correct permissions in case the directory has already been created
			if err = os.Chmod(filen, os.FileMode(header.Mode)); err != nil {
				return err
			}
			if err = os.Chown(filen, header.Uid, header.Gid); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			fmt.Printf("os.Create %s (owner %s/%d group %s/%d)\n", filen, header.Uname, header.Uid, header.Gname, header.Gid)
			dst, err := os.OpenFile(filen, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(dst, tr)
			dst.Close()
			if err != nil {
				return err
			}
		case tar.TypeLink:
			if err = os.Link(header.Linkname, filen); err != nil {
				return err
			}
			continue
		case tar.TypeSymlink:
			if err = os.Symlink(header.Linkname, filen); err != nil {
				return err
			}
			continue
		case tar.TypeChar:
			if err = mkCharDevice(filen, uint32(header.Devmajor), uint32(header.Devminor)); err != nil {
				return err
			}
			if runtime.GOOS != "windows" {
				if err = os.Chmod(filen, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeBlock:
			if err = mkBlockDevice(filen, uint32(header.Devmajor), uint32(header.Devminor)); err != nil {
				return err
			}
			if runtime.GOOS != "windows" {
				if err = os.Chmod(filen, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeFifo:
			if err = mkFifo(filen, uint32(header.Mode)); err != nil {
				return err
			}
		default:
			fmt.Printf("unknown type for %s\n", filen)
			continue
		}
		if err = os.Chown(filen, header.Uid, header.Gid); err != nil {
			return err
		}
		if err = os.Chtimes(filen, header.AccessTime, header.ModTime); err != nil {
			return err
		}
	}
	return nil
}

func extractTar(tarName string, dst string, archiveType osconfigpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType) error {
	file, err := os.Open(tarName)
	if err != nil {
		return err
	}
	decompressed, err := decompress(file, archiveType)
	if err != nil {
		return err
	}
	tr := tar.NewReader(decompressed)

	err = checkForConflicts(tr, dst)
	if err != nil {
		return err
	}

	file.Seek(0, 0)
	decompressed, err = decompress(file, archiveType)
	if err != nil {
		return err
	}
	tr = tar.NewReader(decompressed)

	return createFiles(tr, dst)
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

	return executeCommand(path, step.FileExec.Args, stepDir, runEnvs, []int32{0})
}

// StepScriptRun runs a ScriptRun step.
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string, runEnvs []string, stepDir string) error {
	cmd := filepath.Join(stepDir, "recipe_script_source")
	if err := writeScript(cmd, step.ScriptRun.Script); err != nil {
		return err
	}

	var args []string
	switch step.ScriptRun.Interpreter {
	case osconfigpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED:
		if runtime.GOOS == "windows" {
			args = []string{"/c", cmd}
			cmd = "C:\\Windows\\System32\\cmd.exe"
		}
	case osconfigpb.SoftwareRecipe_Step_RunScript_SHELL:
		if runtime.GOOS == "windows" {
			args = []string{"/c", cmd}
			cmd = "C:\\Windows\\System32\\cmd.exe"
		}
		args = []string{"-c", cmd}
		cmd = "/bin/sh"
	case osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL:
		if runtime.GOOS != "windows" {
			return fmt.Errorf("interpreter %q can only used on Windows systems", step.ScriptRun.Interpreter)
		}
		args = []string{"-File", cmd}
		cmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe"
	default:
		return fmt.Errorf("unsupported interpreter %q", step.ScriptRun.Interpreter)
	}
	return executeCommand(cmd, args, stepDir, runEnvs, step.ScriptRun.AllowedExitCodes)
}

func writeScript(path, contents string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	f.WriteString(contents)
	err = f.Close()
	if err != nil {
		return err
	}
	err = os.Chmod(path, 0755)
	if err != nil {
		return err
	}
	return nil
}

func executeCommand(cmd string, args []string, workDir string, runEnvs []string, allowedExitCodes []int32) error {
	cmdObj := exec.Command(cmd, args...)
	cmdObj.Dir = workDir
	cmdObj.Env = append(cmdObj.Env, runEnvs...)

	// TODO: log output from command.
	_, err := cmdObj.Output()
	if err == nil {
		return nil
	}
	if v, ok := err.(*exec.ExitError); ok && len(allowedExitCodes) != 0 {
		result := int32(v.ExitCode())
		for _, code := range allowedExitCodes {
			if result == code {
				return nil
			}
		}
	}
	return err
}
