package software_recipes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"

	"google.golang.org/api/option"
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

func createHandler(responseMap map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		fmt.Println(r.URL.String())
		resp, ok := responseMap[path]
		if !ok {
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("No mapping found for path %q", path)))
		}
		w.Write([]byte(resp))
	})
}

func TestGetHttpArtifact(t *testing.T) {
	s := httptest.NewTLSServer(createHandler(map[string]string{
		"/testartifact": "testartifact body",
	}))
	defer s.Close()
	fh := &FakeFileHandler{
		CreatedFiles: map[string]*StringWriteCloser{},
	}
	testHttpClient = s.Client()
	testFileHandler = fh
	directory := "/testdir"
	wantLocation := "/testdir/test"
	wantMap := map[string]string{"test": wantLocation}

	artifacts := []Artifact{Artifact{
		name:     "test",
		url:      s.URL + "/testartifact",
		checksum: "d53a628153c63429b3709e0a50a326efea2fc40b7c4afd70101c4b5bc16054ae",
	}}

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

func TestGetGCSArtifact(t *testing.T) {
	urls := []string{"gs://testbucket/testobject"}
	s := httptest.NewTLSServer(createHandler(map[string]string{
		"/testartifact": "testartifact body",
	}))
	defer s.Close()
	fh := &FakeFileHandler{
		CreatedFiles: map[string]*StringWriteCloser{},
	}
	var err error
	fmt.Println(s.URL)
	testStorageClient, err = storage.NewClient(context.Background(), option.WithEndpoint(s.URL), option.WithHTTPClient(s.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}

	testHttpClient = s.Client()
	testFileHandler = fh

	directory := "/testdir"
	wantLocation := "/testdir/test"
	wantMap := map[string]string{"test": wantLocation}
	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			artifacts := []Artifact{Artifact{
				name:     "test",
				url:      url,
				checksum: "d53a628153c63429b3709e0a50a326efea2fc40b7c4afd70101c4b5bc16054ae",
			}}
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
		})
	}
}
