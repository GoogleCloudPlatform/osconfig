package cos

import (
	"fmt"
	"path"
	"regexp"

	"cos.googlesource.com/cos/tools.git/src/pkg/utils"

	"github.com/pkg/errors"
)

const (
	// GPUExtension is the name of GPU extension.
	GPUExtension = "gpu"
)

// ExtensionsDownloader is the struct downloading COS extensions from GCS bucket.
type ExtensionsDownloader interface {
	ListExtensions() ([]string, error)
	ListExtensionArtifacts(extension string) ([]string, error)
	DownloadExtensionArtifact(destDir, extension, artifact string) error
	GetExtensionArtifact(extension, artifact string) ([]byte, error)
}

// ListExtensions lists all supported extensions.
func (d *GCSDownloader) ListExtensions() ([]string, error) {
	var objects []string
	var err error
	gcsPath := path.Join(d.gcsDownloadPrefix, "extensions")
	if objects, err = utils.ListGCSBucket(d.gcsDownloadBucket, gcsPath); err != nil {
		return nil, errors.Wrap(err, "failed to list extensions")
	}
	var extensions []string
	re := regexp.MustCompile(`extensions/(\w+)$`)
	for _, object := range objects {
		if match := re.FindStringSubmatch(object); match != nil {
			extensions = append(extensions, match[1])
		}
	}
	return extensions, nil
}

// ListExtensionArtifacts lists all artifacts of a given extension.
func (d *GCSDownloader) ListExtensionArtifacts(extension string) ([]string, error) {
	var objects []string
	var err error
	gcsPath := path.Join(d.gcsDownloadPrefix, "extensions", extension)
	if objects, err = utils.ListGCSBucket(d.gcsDownloadBucket, gcsPath); err != nil {
		return nil, errors.Wrap(err, "failed to list extensions")
	}

	var artifacts []string
	re := regexp.MustCompile(fmt.Sprintf(`extensions/%s/(.+)$`, extension))
	for _, object := range objects {
		if match := re.FindStringSubmatch(object); match != nil {
			artifacts = append(artifacts, match[1])
		}
	}
	return artifacts, nil
}

// ListGPUExtensionArtifacts lists all artifacts of GPU extension.
func (d *GCSDownloader) ListGPUExtensionArtifacts() ([]string, error) {
	return d.ListExtensionArtifacts(GPUExtension)
}

// DownloadExtensionArtifact downloads an artifact of the given extension.
func (d *GCSDownloader) DownloadExtensionArtifact(destDir, extension, artifact string) error {
	artifactPath := path.Join("extensions", extension, artifact)
	return d.DownloadArtifact(destDir, artifactPath)
}

// GetExtensionArtifact reads the content of an artifact of the given extension.
func (d *GCSDownloader) GetExtensionArtifact(extension, artifact string) ([]byte, error) {
	artifactPath := path.Join("extensions", extension, artifact)
	return d.GetArtifact(artifactPath)
}
