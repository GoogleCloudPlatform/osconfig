package software_recipes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetArtifacts_NoArtifacts(t *testing.T) {
	files, err := FetchArtifacts(context.Background(), []Artifact{}, "/test")
	if err != nil {
		t.Fatalf("FetchArtifacts(ctx, {}, \"/test\") returned unexpected error %q", err)
	}
	if len(files) != 0 {
		t.Fatalf("FetchArtifacts(ctx, {}, \"/test\") = %v, want {}", files)
	}
}

type mappedHandler struct {
	responseMap map[string]string
}

func (h *mappedHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	resp, ok := h.responseMap[path]
	if !ok {
		w.WriteHeader(404)
		w.Write([]byte(fmt.Sprintf("No mapping found for path %q", path)))
	}
	w.Write([]byte(resp))
}

func TestGetHttpArtifact(t *testing.T) {
	s := httptest.NewTLSServer(&mappedHandler{
		responseMap: map[string]string{
			"/testartifact": "testartifact body",
		},
	})
	fh := &FakeFileHandler{
		CreatedFiles: map[string]*StringWriteCloser{},
	}
	testHttpClient = s.Client()
	testFileHandler = fh

	artifacts := []Artifact{Artifact{
		name:     "test",
		url:      s.URL + "/testartifact",
		checksum: "d53a628153c63429b3709e0a50a326efea2fc40b7c4afd70101c4b5bc16054ae",
	}}
	directory := "/testdir"
	wantLocation := "/testdir/test"
	wantMap := map[string]string{"test": wantLocation}

	files, err := FetchArtifacts(context.Background(), artifacts, directory)
	if err != nil {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) returned unexpected error %q", artifacts, directory, err)
	}
	if !cmp.Equal(files, wantMap) {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) = %v, wanted map with one entry", artifacts, directory, files)
	}
	wc, ok := fh.CreatedFiles[wantLocation]
	if !ok {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) did not create expected file %s", artifacts, directory, wantLocation)
	}
	if wc.GetWrittenString() != "testartifact body" {
		t.Fatalf("FetchArtifacts(ctx, %v, %q) wrote %q to file, expected %q", artifacts, directory, wc.GetWrittenString(), "testartifact body")
	}
}
