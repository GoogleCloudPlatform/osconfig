package cos

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	log "github.com/golang/glog"
	"github.com/pkg/errors"

	"cos.googlesource.com/cos/tools.git/src/pkg/utils"
)

const (
	cosToolsGCS      = "cos-tools"
	chromiumOSSDKGCS = "chromiumos-sdk"
	kernelInfo       = "kernel_info"
	kernelSrcArchive = "kernel-src.tar.gz"
	kernelHeaders    = "kernel-headers.tgz"
	toolchainURL     = "toolchain_url"
	toolchainArchive = "toolchain.tar.xz"
	toolchainEnv     = "toolchain_env"
	crosKernelRepo   = "https://chromium.googlesource.com/chromiumos/third_party/kernel"
)

// ArtifactsDownloader defines the interface to download COS artifacts.
type ArtifactsDownloader interface {
	DownloadKernelSrc(destDir string) error
	DownloadToolchainEnv(destDir string) error
	DownloadToolchain(destDir string) error
	DownloadKernelHeaders(destDir string) error
	DownloadArtifact(destDir, artifact string) error
	GetArtifact(artifact string) ([]byte, error)
}

// GCSDownloader is the struct downloading COS artifacts from GCS bucket.
type GCSDownloader struct {
	envReader         *EnvReader
	gcsDownloadBucket string
	gcsDownloadPrefix string
}

// NewGCSDownloader creates a GCSDownloader instance.
func NewGCSDownloader(e *EnvReader, bucket, prefix string) *GCSDownloader {
	// Use cos-tools as the default GCS bucket.
	if bucket == "" {
		bucket = cosToolsGCS
	}
	// Use build number as the default GCS download prefix.
	if prefix == "" {
		prefix = e.BuildNumber()
	}
	return &GCSDownloader{e, bucket, prefix}
}

// DownloadKernelSrc downloads COS kernel sources to destination directory.
func (d *GCSDownloader) DownloadKernelSrc(destDir string) error {
	return d.DownloadArtifact(destDir, kernelSrcArchive)
}

// DownloadToolchainEnv downloads toolchain compilation environment variables to destination directory.
func (d *GCSDownloader) DownloadToolchainEnv(destDir string) error {
	return d.DownloadArtifact(destDir, toolchainEnv)
}

// DownloadToolchain downloads toolchain package to destination directory.
func (d *GCSDownloader) DownloadToolchain(destDir string) error {
	downloadURL, err := d.getToolchainURL()
	if err != nil {
		return errors.Wrap(err, "failed to download toolchain")
	}
	outputPath := filepath.Join(destDir, toolchainArchive)
	if err := utils.DownloadContentFromURL(downloadURL, outputPath, toolchainArchive); err != nil {
		return errors.Wrap(err, "failed to download toolchain")
	}
	return nil
}

// DownloadKernelHeaders downloads COS kernel headers to destination directory.
func (d *GCSDownloader) DownloadKernelHeaders(destDir string) error {
	return d.DownloadArtifact(destDir, kernelHeaders)
}

// GetArtifact gets an artifact from GCS buckets and returns its content.
func (d *GCSDownloader) GetArtifact(artifactPath string) ([]byte, error) {
	tmpDir, err := ioutil.TempDir("", "tmp")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tmpDir)

	if err = d.DownloadArtifact(tmpDir, artifactPath); err != nil {
		return nil, errors.Wrapf(err, "failed to download artifact %s", artifactPath)
	}

	content, err := ioutil.ReadFile(filepath.Join(tmpDir, filepath.Base(artifactPath)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file %s", filepath.Join(tmpDir, artifactPath))
	}

	return content, nil
}

// DownloadArtifact downloads an artifact from the GCS prefix configured in GCSDownloader.
func (d *GCSDownloader) DownloadArtifact(destDir, artifactPath string) error {
	gcsPath := path.Join(d.gcsDownloadPrefix, artifactPath)
	if err := utils.DownloadFromGCS(destDir, d.gcsDownloadBucket, gcsPath); err != nil {
		return errors.Errorf("failed to download %s from gs://%s/%s", artifactPath, d.gcsDownloadBucket, gcsPath)
	}
	return nil
}

func (d *GCSDownloader) getToolchainURL() (string, error) {
	// First, check if the toolchain path is available locally
	tcPath := d.envReader.ToolchainPath()
	if tcPath != "" {
		log.V(2).Info("Found toolchain path file locally")
		return fmt.Sprintf("https://storage.googleapis.com/%s/%s", chromiumOSSDKGCS, tcPath), nil
	}

	// Next, check if the toolchain path is available in GCS.
	tmpDir, err := ioutil.TempDir("", "temp")
	if err != nil {
		return "", errors.Wrap(err, "failed to create tmp dir")
	}
	defer os.RemoveAll(tmpDir)
	if err := d.DownloadArtifact(tmpDir, toolchainURL); err != nil {
		return "", err
	}
	toolchainURLContent, err := ioutil.ReadFile(filepath.Join(tmpDir, toolchainURL))
	if err != nil {
		return "", errors.Wrap(err, "failed to read toolchain URL file")
	}
	return string(toolchainURLContent), nil
}
