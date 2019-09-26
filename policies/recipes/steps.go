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

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

var extensionMap = map[osconfigpb.SoftwareRecipe_Step_RunScript_Interpreter]string{
	osconfigpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED: ".bat",
	osconfigpb.SoftwareRecipe_Step_RunScript_SHELL:                   ".bat",
	osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL:              ".ps1",
}

func stepCopyFile(step *osconfigpb.SoftwareRecipe_Step_CopyFile, artifacts map[string]string, runEnvs []string, stepDir string) error {
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

func stepExtractArchive(step *osconfigpb.SoftwareRecipe_Step_ExtractArchive, artifacts map[string]string, runEnvs []string, stepDir string) error {
	artifact := step.GetArtifactId()
	filename, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", artifact)
	}
	switch typ := step.GetType(); typ {
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_ZIP:
		return extractZip(filename, step.Destination)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ,
		osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR:
		return extractTar(filename, step.Destination, typ)
	default:
		return fmt.Errorf("Unrecognized archive type %q", typ)
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
		filen, err := util.NormPath(filepath.Join(dst, normalizeSlashes(f.Name)))
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
		filen, err := util.NormPath(filepath.Join(dst, normalizeSlashes(f.Name)))
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
		filen, err := util.NormPath(filepath.Join(dst, header.Name))
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
		filen, err := util.NormPath(filepath.Join(dst, header.Name))
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
			logger.Infof("unknown file type for tar entry %s\n", filen)
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

func stepInstallMsi(step *osconfigpb.SoftwareRecipe_Step_InstallMsi, artifacts map[string]string, runEnvs []string, stepDir string) error {
	artifact := step.GetArtifactId()
	path, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", artifact)
	}
	args := step.Flags
	if len(args) == 0 {
		args = []string{"/i", "/qn", "/norestart"}
	}
	args = append(args, path)

	exitCodes := step.AllowedExitCodes
	if len(exitCodes) == 0 {
		exitCodes = []int32{0, 1641, 3010}
	}
	return executeCommand("C:\\Windows\\System32\\msiexec.exe", args, stepDir, runEnvs, exitCodes)
}

func stepInstallDpkg(step *osconfigpb.SoftwareRecipe_Step_InstallDpkg, artifacts map[string]string, runEnvs []string, stepDir string) error {
	return fmt.Errorf("InstallDpkg not yet supported")
}

func stepInstallRpm(step *osconfigpb.SoftwareRecipe_Step_InstallRpm, artifacts map[string]string, runEnvs []string, stepDir string) error {
	return fmt.Errorf("InstallRpm not yet supported")
}

func stepExecFile(step *osconfigpb.SoftwareRecipe_Step_ExecFile, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var path string
	switch {
	case step.GetArtifactId() != "":
		var ok bool
		artifact := step.GetArtifactId()
		path, ok = artifacts[artifact]
		if !ok {
			return fmt.Errorf("%q not found in artifact map", artifact)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		newMode := info.Mode() | 0100
		if newMode != info.Mode() {
			os.Chmod(path, newMode)
		}
	case step.GetLocalPath() != "":
		path = step.GetLocalPath()
	default:
		return fmt.Errorf("can't determine location type")

	}

	return executeCommand(path, step.Args, stepDir, runEnvs, []int32{0})
}

func stepRunScript(step *osconfigpb.SoftwareRecipe_Step_RunScript, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var extension string
	if runtime.GOOS == "windows" {
		extension = extensionMap[step.Interpreter]
	}
	scriptPath := filepath.Join(stepDir, "recipe_script_source"+extension)
	if err := writeScript(scriptPath, step.Script); err != nil {
		return err
	}

	var cmd string
	var args []string
	switch step.Interpreter {
	case osconfigpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED:
		cmd = scriptPath
		args = step.Args
	case osconfigpb.SoftwareRecipe_Step_RunScript_SHELL:
		if runtime.GOOS == "windows" {
			cmd = scriptPath
			args = step.Args
		} else {
			args = append([]string{scriptPath}, step.Args...)
			cmd = "/bin/sh"
		}
	case osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL:
		if runtime.GOOS != "windows" {
			return fmt.Errorf("interpreter %q can only used on Windows systems", step.Interpreter)
		}
		args = append([]string{"-File", scriptPath}, step.Args...)
		cmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe"
	default:
		return fmt.Errorf("unsupported interpreter %q", step.Interpreter)
	}
	return executeCommand(cmd, args, stepDir, runEnvs, step.AllowedExitCodes)
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
