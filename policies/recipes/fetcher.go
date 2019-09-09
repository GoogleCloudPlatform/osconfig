package recipes

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

type Fetcher interface {
	fetch(ctx context.Context) (io.ReadCloser, error)
}

type GCS_fetcher struct {
	client         *storage.Client
	Bucket, Object string
	generation     int64
}

type HTTP_fetcher struct {
	client *http.Client
	uri    string
}

func (fetcher *GCS_fetcher) fetch(ctx context.Context) (io.ReadCloser, error) {
	oh := fetcher.client.Bucket(fetcher.Bucket).Object(fetcher.Object)
	if fetcher.generation != 0 {
		oh = oh.Generation(fetcher.generation)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (fetcher *HTTP_fetcher) fetch(ctx context.Context) (io.ReadCloser, error) {
	resp, err := fetcher.client.Get(fetcher.uri)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http status %d when attempting to download artifact", resp.StatusCode)
	}

	return resp.Body, nil
}

func DownloadStream(ctx context.Context, r io.ReadCloser, checksum, localPath string) error {
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
