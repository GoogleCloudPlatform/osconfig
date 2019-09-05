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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/common"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"

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
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP:
		fallthrough
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP:
		fallthrough
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA:
		fallthrough
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ:
		fallthrough
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR:
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

func decompress(reader io.Reader, archiveType osconfigpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType) (io.ReadCloser, error) {
	switch archiveType {
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP:
		return gzip.NewReader(reader)
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP:
		return ioutil.NopCloser(bzip2.NewReader(reader)), nil
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA:
		decompressed, err := lzma.NewReader2(reader)
		return ioutil.NopCloser(decompressed), err
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ:
		decompressed, err := xz.NewReader(reader)
		return ioutil.NopCloser(decompressed), err
	case osconfigpb.SoftwareRecipe_Step_ExtractArchive_TAR:
		return ioutil.NopCloser(reader), nil
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
		err = os.MkdirAll(filedir, 0755)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(filen, os.FileMode(header.Mode)); err != nil {
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
	decompressed.Close()
	if err != nil {
		return err
	}

	file.Seek(0, 0)
	decompressed, err = decompress(file, archiveType)
	if err != nil {
		return err
	}
	tr = tar.NewReader(decompressed)

	err = createFiles(tr, dst)
	decompressed.Close()
	return err
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
	switch step.ScriptRun.Interpreter {
	case osconfigpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED:
		return scriptRunDirect(step, runEnvs, stepDir)
	case osconfigpb.SoftwareRecipe_Step_RunScript_SHELL:
		if runtime.GOOS == "windows" {
			return scriptRunCmd(step, runEnvs, stepDir)
		}
		return scriptRunSh(step, runEnvs, stepDir)
	case osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL:
		if runtime.GOOS == "windows" {
			return scriptRunPowershell(step, runEnvs, stepDir)
		}
		return fmt.Errorf("interpreter %q cannot be used on non-Windows system", step.ScriptRun.Interpreter)
	default:
		return fmt.Errorf("invalid interpreter %q", step.ScriptRun.Interpreter)
	}
}

func scriptRunSh(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, runEnvs []string, stepDir string) error {
	scriptPath := filepath.Join(stepDir, "script")

	if err := writeScript(scriptPath, step.ScriptRun.Script); err != nil {
		return nil
	}

	var qargs []string
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	command := scriptPath + " " + strings.Join(qargs, " ")
	args := []string{"-c", command}
	return executeCommand("/bin/sh", stepDir, runEnvs, args...)
}

func scriptRunDirect(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, runEnvs []string, stepDir string) error {
	scriptPath := filepath.Join(stepDir, "script")

	if err := writeScript(scriptPath, step.ScriptRun.Script); err != nil {
		return err
	}

	return executeCommand(scriptPath, stepDir, runEnvs, step.ScriptRun.Args...)
}

func scriptRunPowershell(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, runEnvs []string, stepDir string) error {
	scriptPath := filepath.Join(stepDir, "script.ps1")

	if err := writeScript(scriptPath, step.ScriptRun.Script); err != nil {
		return err
	}

	args := append([]string{"-File", scriptPath}, step.ScriptRun.Args...)
	return executeCommand("PowerShell", stepDir, runEnvs, args...)
}

func scriptRunCmd(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, runEnvs []string, stepDir string) error {
	scriptPath := filepath.Join(stepDir, "script.bat")

	if err := writeScript(scriptPath, step.ScriptRun.Script); err != nil {
		return err
	}

	var qargs []string
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	command := scriptPath + " " + strings.Join(qargs, " ")
	args := []string{"/c", command}
	return executeCommand("cmd", stepDir, runEnvs, args...)
}

func writeScript(path, script string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	f.WriteString(script)
	f.Close()
	if err := os.Chmod(path, 0755); err != nil {
		return err
	}
	return nil
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
