package cos

import (
	"io/ioutil"
	"path/filepath"
	"syscall"

	"cos.googlesource.com/cos/tools.git/src/pkg/utils"

	"github.com/pkg/errors"
)

const (
	osReleasePath     = "/etc/os-release"
	toolchainPathFile = "/etc/toolchain-path"

	buildID        = "BUILD_ID"
	version        = "VERSION"
	kernelCommitID = "KERNEL_COMMIT_ID"
)

// EnvReader is to read system configurations of COS.
// TODO(mikewu): rename EnvReader to a better name.
type EnvReader struct {
	osRelease     map[string]string
	toolchainPath string
	uname         syscall.Utsname
}

// NewEnvReader returns an instance of EnvReader.
func NewEnvReader(hostRootPath string) (reader *EnvReader, err error) {
	reader = &EnvReader{}
	reader.osRelease, err = utils.LoadEnvFromFile(hostRootPath, osReleasePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read OsRelease file from %s", osReleasePath)
	}

	if toolchainPath, err := ioutil.ReadFile(filepath.Join(hostRootPath, toolchainPathFile)); err == nil {
		reader.toolchainPath = string(toolchainPath)
	}

	if err := syscall.Uname(&reader.uname); err != nil {
		return nil, errors.Wrap(err, "failed to get uname")
	}
	return reader, nil
}

// OsRelease returns configs of /etc/os-release as a map.
func (c *EnvReader) OsRelease() map[string]string { return c.osRelease }

// BuildNumber returns COS build number.
func (c *EnvReader) BuildNumber() string { return c.osRelease[buildID] }

// Milestone returns COS milestone.
func (c *EnvReader) Milestone() string { return c.osRelease[version] }

// KernelCommit returns commit hash of the COS kernel.
func (c *EnvReader) KernelCommit() string { return c.osRelease[kernelCommitID] }

// ToolchainPath returns the toolchain path of the COS version.
// It may return an empty string if the COS version doesn't support the feature.
func (c *EnvReader) ToolchainPath() string { return c.toolchainPath }

// KernelRelease return COS kernel release, i.e. `uname -r`
func (c *EnvReader) KernelRelease() string { return charsToString(c.uname.Release[:]) }

// charsToString converts a c-style byte array (null-terminated string) to string.
func charsToString(chars []int8) string {
	s := make([]byte, 0, len(chars))
	for _, ch := range chars {
		if ch == 0 {
			break
		}
		s = append(s, byte(ch))
	}
	return string(s)
}
