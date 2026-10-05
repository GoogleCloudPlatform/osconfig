// Package utils provides utility functions.
package utils

import (
	"archive/tar"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"

	log "github.com/golang/glog"
)

var (
	downloadRetries = 3
	lockFile        = "/root/tmp/cos_gpu_installer_lock"
)

type serviceAccountToken struct {
	Token     string `json:"access_token"`
	Expire    int    `json:"expires_in"`
	TokenType string `json:"token_type"`
}

type listStorageObjectsResponse struct {
	Kind  string `json:"kind"`
	Items []struct {
		Kind                    string `json:"kind"`
		ID                      string `json:"id"`
		SelfLink                string `json:"selfLink"`
		MediaLink               string `json:"mediaLink"`
		Name                    string `json:"name"`
		Bucket                  string `json:"bucket"`
		Generation              string `json:"generation"`
		Metageneration          string `json:"metageneration"`
		ContentType             string `json:"contentType"`
		StorageClass            string `json:"storageClass"`
		Size                    string `json:"size"`
		Md5Hash                 string `json:"md5Hash"`
		Crc32c                  string `json:"crc32c"`
		Etag                    string `json:"etag"`
		TimeCreated             string `json:"timeCreated"`
		Updated                 string `json:"updated"`
		TimeStorageClassUpdated string `json:"timeStorageClassUpdated"`
	} `json:"items"`
}

// Flock exclusively locks a special file on the host to make sure only one calling process is running at any time.
func Flock() {
	// TODO(mikewu): generalize Flock to make it useful for other use cases.
	f, err := os.OpenFile(lockFile, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Exitf("Failed to open lock file: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		log.Exitf("File %s is locked. Other process might be running.", lockFile)
	}
}

// DownloadContentFromURL downloads file from a given URL.
func DownloadContentFromURL(url, outputPath, infoStr string) error {
	url = strings.TrimSpace(url)
	log.Infof("Downloading %s from %s", infoStr, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to download %s from %s", infoStr, url)
	}
	// TODO(mikewu): Consider using GCS GO package.
	if strings.HasPrefix(url, "https://storage.googleapis.com") {
		// TODO(mikewu): Consider using sgauth (https://github.com/google/oauth2l/tree/master/sgauth).
		token, err := GetDefaultVMToken()
		if err != nil {
			return errors.Wrap(err, "failed to get VM token")
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %s", outputPath)
	}
	defer outputFile.Close()

	client := &http.Client{}

	var response *http.Response
	retries := downloadRetries
	for retries > 0 {
		response, err = client.Do(req)
		if err != nil {
			log.Errorf("Failed to download %s: %v", infoStr, err)
			retries--
			time.Sleep(time.Second)
			log.Info("Retry...")
		} else {
			break
		}
	}
	if response == nil {
		return errors.Wrapf(err, "failed to download %s", infoStr)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.Errorf("failed to download %s, status: %s", infoStr, response.Status)
	}
	if _, err := io.Copy(outputFile, response.Body); err != nil {
		return errors.Wrapf(err, "failed to download %s", infoStr)
	}

	log.Infof("Successfully downloaded %s from %s", infoStr, url)
	return nil
}

// DownloadFromGCS downloads an object from the given GCS path.
func DownloadFromGCS(destDir, gcsBucket, gcsPath string) error {
	downloadURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", gcsBucket, gcsPath)
	filename := filepath.Base(gcsPath)
	outputPath := filepath.Join(destDir, filename)
	return DownloadContentFromURL(downloadURL, outputPath, filename)
}

// ListGCSBucket lists the objects whose names begin with the given prefix in the given GCS bucekt.
func ListGCSBucket(bucket, prefix string) ([]string, error) {
	log.Infof("Listing objects from GCS bucekt %s with prefix %s", bucket, prefix)

	url := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o?prefix=%s", bucket, prefix)
	dir, err := ioutil.TempDir("", "bucketlist")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tempdir")
	}
	defer os.RemoveAll(dir)
	tmpfile := filepath.Join(dir, "bucketlist")
	if err := DownloadContentFromURL(url, tmpfile, "bucketlist"); err != nil {
		return nil, errors.Wrapf(err, "failed to downoad url %s", url)
	}

	content, err := ioutil.ReadFile(tmpfile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %s", tmpfile)
	}
	var jsonContent listStorageObjectsResponse
	if err := json.Unmarshal(content, &jsonContent); err != nil {
		return nil, errors.Wrapf(err, "failed to parse json string %s", string(content))
	}

	var objects []string
	for _, item := range jsonContent.Items {
		objects = append(objects, item.Name)
	}
	return objects, nil
}

// GetDefaultVMToken returns the default GCE service account of the COS VM the program is running on.
func GetDefaultVMToken() (string, error) {
	tokenStr, err := GetGCEMetadata("service-accounts/default/token")
	if err != nil {
		return "", errors.Wrap(err, "failed to get default VM token")
	}
	token, err := parseVMToken(tokenStr)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse VM token")
	}
	return token.Token, nil
}

// GetGCEMetadata queries GCE metadata server to get the value of a given metadata key.
func GetGCEMetadata(metadataPath string) (string, error) {
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/"+metadataPath, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to get GCE metadata")
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to get GCE metadata")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to get GCE metadata")
	}
	return string(body), nil
}

// IsDirEmpty returns whether a given directory is empty.
func IsDirEmpty(dirName string) (bool, error) {
	dir, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer dir.Close()
	_, err = dir.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// LoadEnvFromFile reads an env file from fs into memory as a map.
func LoadEnvFromFile(prefix, filePath string) (map[string]string, error) {
	path := filepath.Join(prefix, filePath)
	envs := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %s", path)
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	// TODO(mikewu): Consider using https://golang.org/pkg/bufio/#Scanner.
	for {
		line, err := rd.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, errors.Wrapf(err, "failed to read file %s", path)
		}
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			parts := strings.SplitN(trimmedLine, "=", 2)
			if len(parts) != 2 {
				return nil, errors.Wrapf(err, "Unrecognized format: %s", trimmedLine)
			}
			envs[parts[0]] = strings.Trim(parts[1], `"'`)
		}
		if err == io.EOF {
			break
		}
	}
	return envs, nil
}

// CreateTarFile creates a tar archive file given a map of {filename: content}.
func CreateTarFile(tarFilename string, files map[string][]byte) error {
	tarFile, err := os.Create(tarFilename)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	for name, body := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// RunCommandAndLogOutput runs the given command and logs the stdout and stderr in parallel.
func RunCommandAndLogOutput(cmd *exec.Cmd, expectError bool) error {
	errLogger := log.Error
	if expectError {
		errLogger = log.V(1).Info
	}

	cmd.Stdout = &loggingWriter{logger: log.Info}
	cmd.Stderr = &loggingWriter{logger: errLogger}

	err := cmd.Run()
	if _, ok := err.(*exec.ExitError); ok && expectError {
		log.Warningf("command %s didn't complete successfully: %v", cmd.Path, err)
		return nil
	}
	return err
}

// CopyFile copies a file from src to dest.
func CopyFile(src, dest string) error {
	srcfile, err := os.Open(src)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s", src)
	}
	defer srcfile.Close()
	destfile, err := os.Create(dest)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %s", dest)
	}
	defer destfile.Close()
	if _, err := io.Copy(destfile, srcfile); err != nil {
		return errors.Wrapf(err, "failed to copy file from %s to %s", dest, src)
	}
	if err := srcfile.Close(); err != nil {
		return errors.Wrapf(err, "failed to close source file %s", src)
	}
	if err := destfile.Close(); err != nil {
		return errors.Wrapf(err, "failed to close destination file %s", dest)
	}
	return nil
}

// MoveFile moves a file from src to dest.
// Avoid to use os.Rename as the src and dst may on different filesystems,
// e.g. (container temp fs -> host mounted volume).
func MoveFile(src, dest string) error {
	if err := CopyFile(src, dest); err != nil {
		return errors.Wrapf(err, "failed to move file from %s to %s", src, dest)
	}
	if err := os.Remove(src); err != nil {
		return errors.Wrapf(err, "failed to remove file %s", src)
	}
	return nil
}

func parseVMToken(tokenStr string) (*serviceAccountToken, error) {
	var token serviceAccountToken
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		return nil, err
	}
	return &token, nil
}

type loggingWriter struct {
	logger func(...interface{})
	buf    []byte
}

func (l *loggingWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			l.logger(string(l.buf[:]))
			l.buf = nil
			continue
		}
		l.buf = append(l.buf, b)
	}
	return len(p), nil
}
