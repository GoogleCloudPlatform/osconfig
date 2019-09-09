package external

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/osconfig/common"
)

// GCSFetcher fetches data from GCS bucket
type GCSFetcher struct {
	Client         *storage.Client
	Bucket, Object string
	Generation     int64
}

// HTTPFetcher fetches data from remote location
// using http client
type HTTPFetcher struct {
	Client *http.Client
	URI    string
}

// Fetch fetches data from GCS bucket
func (fetcher *GCSFetcher) Fetch(ctx context.Context) (io.ReadCloser, error) {
	oh := fetcher.Client.Bucket(fetcher.Bucket).Object(fetcher.Object)
	if fetcher.Generation != 0 {
		oh = oh.Generation(fetcher.Generation)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// Fetch fetches data from remote location
func (fetcher *HTTPFetcher) Fetch(ctx context.Context) (io.ReadCloser, error) {
	resp, err := fetcher.Client.Get(fetcher.URI)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http status %d when attempting to download artifact", resp.StatusCode)
	}

	return resp.Body, nil
}

// DownloadStream fetches data from an input stream
func DownloadStream(r io.ReadCloser, checksum, localPath string) error {
	localPath, err := common.NormPath(localPath)
	if err != nil {
		return err
	}
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(file, hasher), r); err != nil {
		return err
	}
	computed := hex.EncodeToString(hasher.Sum(nil))
	if checksum != "" && !strings.EqualFold(checksum, computed) {
		return fmt.Errorf("got %q for checksum, expected %q", computed, checksum)
	}
	return nil
}
