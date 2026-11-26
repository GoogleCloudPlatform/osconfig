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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
)

var extensionMap = map[agentendpointpb.SoftwareRecipe_Step_RunScript_Interpreter]string{
	agentendpointpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED: ".bat",
	agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL:                   ".bat",
	agentendpointpb.SoftwareRecipe_Step_RunScript_POWERSHELL:              ".ps1",
}

var chown = chownFunc

func stepCopyFile(step *agentendpointpb.SoftwareRecipe_Step_CopyFile, artifacts map[string]string, runEnvs []string, stepDir string) error {
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
		if err := os.Chmod(dest, permissions); err != nil {
			return err
		}
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

	_, err = util.AtomicWriteFileStream(reader, "", dest, permissions)

	return err
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

func stepExtractArchive(ctx context.Context, step *agentendpointpb.SoftwareRecipe_Step_ExtractArchive, artifacts map[string]string, runEnvs []string, stepDir string) error {
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
		return extractTar(ctx, filename, step.Destination, typ)
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

// symlinkInRoot creates the symlink name, a path relative to the root, pointing to target.
// The symlink is kept only if it points to an existing file inside of the root.
func symlinkInRoot(root *os.Root, target, name string) error {
	if err := root.Symlink(target, name); err != nil {
		return err
	}

	return ensureLinkResolvesInRoot(root, name)
}

// hardLinkInRoot creates the hard link name to target, both are paths relative to the root.
// The link is kept only if it points to an existing file inside of the root.
func hardLinkInRoot(root *os.Root, target, name string) error {
	if err := root.Link(target, name); err != nil {
		return err
	}

	// Target can be a symlink, then the link is one more symlink that is resolved from the new location.
	return ensureLinkResolvesInRoot(root, name)
}

// ensureLinkResolvesInRoot removes the link name, a path relative to the root,
// if it does not point to an existing file inside of the root.
// The link is resolved by the root the same way OS does it, one path element at a time,
// so neither "<symlink>/.." elements nor chains of symlinks can be used to bypass the check.
func ensureLinkResolvesInRoot(root *os.Root, name string) error {
	if _, err := root.Stat(name); err != nil {
		return errors.Join(err, root.Remove(name))
	}

	return nil
}

func ensureFilePathBelongsToDir(dirPath string, filePath string) error {
	dirAbs, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(dirAbs, fileAbs)
	if err != nil {
		return err
	}

	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %s, does not belongs to dir %s, rel %s", filePath, dirPath, rel)
	}

	return nil
}

func extractZip(zipPath string, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	// Check that we can extract zip
	for _, f := range zr.File {
		filen, err := util.NormPath(filepath.Join(dst, f.Name))
		if err != nil {
			return err
		}

		if err := ensureFilePathBelongsToDir(dst, filen); err != nil {
			return fmt.Errorf("unable to extract zip archive %s: %w", zipPath, err)
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
		return fmt.Errorf("unable to extract zip archive %s: file %s is already exists", zipPath, filen)
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

		if err := ensureFilePathBelongsToDir(dst, filen); err != nil {
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

// pathInRoot converts the path from the tar entry to the path relative to the extraction root.
// Leading path separator is dropped, the same way filepath.Join(dst, path) does it.
func pathInRoot(path string) string {
	return filepath.Join(".", path)
}

// openRootDir opens dst, creating it if it does not exist, as the root directory of the extraction.
// File operations performed through the returned root can not reach outside of dst,
// including the ones that go through symlinks.
func openRootDir(dst string) (*os.Root, error) {
	// Empty dst is the current working directory, as it is for the paths built with filepath.Join(dst, name).
	dst, err := filepath.Abs(dst)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dst, 0700); err != nil {
		return nil, err
	}

	return os.OpenRoot(dst)
}

func extractTar(ctx context.Context, tarName string, dst string, archiveType agentendpointpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType) error {
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
		return fmt.Errorf("unable to extract tar archive %s: %s", tarName, err)
	}

	file.Seek(0, 0)
	decompressed, err = decompress(file, archiveType)
	if err != nil {
		return err
	}
	tr = tar.NewReader(decompressed)

	root, err := openRootDir(dst)
	if err != nil {
		return err
	}
	defer root.Close()

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

		if err := ensureFilePathBelongsToDir(dst, filen); err != nil {
			return err
		}

		// Files are created through the root by the path relative to it, this way they can not get outside of dst.
		name := pathInRoot(header.Name)

		if err := root.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, os.FileMode(header.Mode).Perm()); err != nil {
				return err
			}
			// Setting to correct permissions in case the directory has already been created
			if err := root.Chmod(name, os.FileMode(header.Mode)); err != nil {
				return err
			}
			if err := chown(root, name, header.Uid, header.Gid); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			dst, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE, os.FileMode(header.Mode).Perm())
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
			// Linkname of the hard link is a path of another archive entry, so it is relative to the root as well.
			if err := hardLinkInRoot(root, pathInRoot(header.Linkname), name); err != nil {
				clog.Infof(ctx,
					"link %s resolved outside of destination %s, for the security reason it is not allowed: %v", header.Linkname, dst, err)
				continue
			}
			continue
		case tar.TypeSymlink:
			if err := symlinkInRoot(root, header.Linkname, name); err != nil {
				clog.Infof(ctx,
					"symlink %s resolved outside of destination %s, for the security reason it is not allowed: %v", header.Linkname, dst, err)
				continue
			}
			continue
		case tar.TypeChar:
			if err := mkCharDevice(root, name, uint32(header.Devmajor), uint32(header.Devminor)); err != nil {
				return err
			}
			if runtime.GOOS != "windows" {
				if err := root.Chmod(name, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeBlock:
			if err := mkBlockDevice(root, name, uint32(header.Devmajor), uint32(header.Devminor)); err != nil {
				return err
			}
			if runtime.GOOS != "windows" {
				if err := root.Chmod(name, os.FileMode(header.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeFifo:
			if err := mkFifo(root, name, uint32(header.Mode)); err != nil {
				return err
			}
		default:
			clog.Infof(ctx, "Unknown file type for tar entry %s\n", filen)
			continue
		}
		if err := chown(root, name, header.Uid, header.Gid); err != nil {
			return err
		}
		if err := root.Chtimes(name, header.AccessTime, header.ModTime); err != nil {
			return err
		}
	}

	return nil
}

func stepInstallMsi(ctx context.Context, step *agentendpointpb.SoftwareRecipe_Step_InstallMsi, artifacts map[string]string, runEnvs []string, stepDir string) error {
	if runtime.GOOS != "windows" {
		return errors.New("SoftwareRecipe_Step_InstallMsi only applicable on Windows")
	}
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
	return executeCommand(ctx, "C:\\Windows\\System32\\msiexec.exe", args, stepDir, runEnvs, exitCodes)
}

func stepInstallDpkg(ctx context.Context, step *agentendpointpb.SoftwareRecipe_Step_InstallDpkg, artifacts map[string]string) error {
	if !packages.DpkgExists {
		return fmt.Errorf("dpkg does not exist on system")
	}

	artifact := step.GetArtifactId()
	path, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", artifact)
	}

	return packages.DpkgInstall(ctx, path)
}

func stepInstallRpm(ctx context.Context, step *agentendpointpb.SoftwareRecipe_Step_InstallRpm, artifacts map[string]string) error {
	if !packages.RPMExists {
		return fmt.Errorf("rpm does not exist on system")
	}

	artifact := step.GetArtifactId()
	path, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", artifact)
	}

	return packages.RPMInstall(ctx, path)
}

func stepExecFile(ctx context.Context, step *agentendpointpb.SoftwareRecipe_Step_ExecFile, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var path string
	switch {
	case step.GetArtifactId() != "":
		var ok bool
		artifact := step.GetArtifactId()
		path, ok = artifacts[artifact]
		if !ok {
			return fmt.Errorf("%q not found in artifact map", artifact)
		}

		// By default artifacts are created with 0644
		if err := os.Chmod(path, 0755); err != nil {
			return fmt.Errorf("error setting execute permissions on artifact %s: %v", step.GetArtifactId(), err)
		}
	case step.GetLocalPath() != "":
		path = step.GetLocalPath()
	default:
		return fmt.Errorf("can't determine location type")

	}

	return executeCommand(ctx, path, step.Args, stepDir, runEnvs, []int32{0})
}

func stepRunScript(ctx context.Context, step *agentendpointpb.SoftwareRecipe_Step_RunScript, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var extension string
	if runtime.GOOS == "windows" {
		extension = extensionMap[step.Interpreter]
	}
	scriptPath := filepath.Join(stepDir, "recipe_script_source"+extension)
	if err := util.AtomicWrite(scriptPath, []byte(step.Script), 0755); err != nil {
		return err
	}

	var cmd string
	var args []string
	switch step.Interpreter {
	case agentendpointpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED:
		cmd = scriptPath
	case agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL:
		if runtime.GOOS == "windows" {
			cmd = scriptPath
		} else {
			args = append(args, scriptPath)
			cmd = "/bin/sh"
		}
	case agentendpointpb.SoftwareRecipe_Step_RunScript_POWERSHELL:
		if runtime.GOOS != "windows" {
			return fmt.Errorf("interpreter %q can only be used on Windows systems", step.Interpreter)
		}
		args = append(args, "-File", scriptPath)
		cmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe"
	default:
		return fmt.Errorf("unsupported interpreter %q", step.Interpreter)
	}
	return executeCommand(ctx, cmd, args, stepDir, runEnvs, step.AllowedExitCodes)
}

func executeCommand(ctx context.Context, cmd string, args []string, workDir string, runEnvs []string, allowedExitCodes []int32) error {
	cmdObj := exec.Command(cmd, args...)
	cmdObj.Dir = workDir
	defaultEnv, err := createDefaultEnvironment()
	if err != nil {
		return fmt.Errorf("error creating default environment: %v", err)
	}
	cmdObj.Env = append(cmdObj.Env, defaultEnv...)
	cmdObj.Env = append(cmdObj.Env, runEnvs...)

	o, err := cmdObj.CombinedOutput()
	clog.Infof(ctx, "Combined output for %q command:\n%s", cmd, o)
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

func chownFunc(root *os.Root, name string, uid, gid int) error {
	// os.Chown unsupported on windows
	if runtime.GOOS == "windows" {
		return nil
	}
	return root.Chown(name, uid, gid)
}
