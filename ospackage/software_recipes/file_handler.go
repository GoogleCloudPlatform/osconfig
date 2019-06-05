package software_recipes

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// Wraps the file creation to allow a fake to be substituted for testing.
type OSFileHandler struct{}

func (fh *OSFileHandler) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

func (fh *OSFileHandler) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

type FileHandler interface {
	Create(string) (io.WriteCloser, error)
	Open(string) (io.ReadCloser, error)
}

type FakeFileHandler struct {
	CreatedFiles map[string]*StringWriteCloser
	FakeFiles    map[string]string
}

type StringReadCloser struct {
	*strings.Reader
	Closed bool
}

func (rc *StringReadCloser) Close() error {
	rc.Closed = true
	return nil
}

type StringWriteCloser struct {
	*bytes.Buffer
	Closed bool
}

func (wc *StringWriteCloser) Close() error {
	wc.Closed = true
	return nil
}

func (wc *StringWriteCloser) GetWrittenString() string {
	return wc.Buffer.String()
}

func (fh *FakeFileHandler) Create(path string) (io.WriteCloser, error) {
	wc := &StringWriteCloser{Buffer: &bytes.Buffer{}}
	fh.CreatedFiles[path] = wc
	return wc, nil
}

func (fh *FakeFileHandler) Open(path string) (io.ReadCloser, error) {
	contents, ok := fh.FakeFiles[path]
	if !ok {
		return nil, fmt.Errorf("Fake contents for file with path %q has not been initialized", path)
	}
	return &StringReadCloser{Reader: strings.NewReader(contents)}, nil
}
