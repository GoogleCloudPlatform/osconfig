package software_recipes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
	s := httptest.NewServer(&mappedHandler{})
	testHttpClient = s.Client()
	testFileHandler = &FakeFileHandler{}

	artifacts := []Artifact{Artifact{
		name:     "test",
		url:      s.URL + "/testartifact",
		checksum: "definitely invalid",
	}}

	files, err := FetchArtifacts(context.Background(), artifacts, "/test")
	if err != nil {
		t.Fatalf("FetchArtifacts(ctx, %v, \"/test\") returned unexpected error %q", artifacts, err)
	}
	if len(files) != 1 {
		t.Fatalf("FetchArtifacts(ctx, %v, \"/test\") = %v, want {}", artifacts, files)
	}
}
