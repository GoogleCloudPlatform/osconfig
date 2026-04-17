package utiltest

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kr/pretty"
)

// BytesFromFile returns file as bytes; propagates err (e.g. file does not exist) as test failure reason
func BytesFromFile(t *testing.T, filepath string) []byte {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("readFile(%q) err: %v", filepath, err)
	}
	return bytes
}

const draftSnapshotFileSuffix = ".draft"

// testReporter is a subset of *testing.T,
// defines minimum interface for reporting test failures and logging.
type testReporter interface {
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...any)
	Helper()
}

func makeSnapshotDraftFilepath(snapshotFilepath string) string {
	return snapshotFilepath + draftSnapshotFileSuffix
}

func writeSnapshotDraft(t testReporter, filepath string, snapshot string) {
	t.Helper()
	draftFilepath := makeSnapshotDraftFilepath(filepath)
	if err := os.WriteFile(draftFilepath, []byte(snapshot), 0644); err != nil {
		t.Error(err)
		return
	}
	t.Logf("Remove %q suffix from %q actual data snapshot to make test pass.", draftSnapshotFileSuffix, draftFilepath)
}

func removeSnapshotDraft(filepath string) {
	draftFilepath := makeSnapshotDraftFilepath(filepath)
	os.Remove(draftFilepath)
}

// MatchSnapshot compares the actual data against a stored snapshot file.
//
// If the snapshot file doesn't exist, it creates a new draft file
// (with a .draft suffix) containing the actual data and marks test failed.
//
// If the snapshot file exists but its content differs from the actual data,
// it updates the draft file with the actual data, reports test failure and
// instructs on how to update the snapshot.
//
// If the snapshot file exists and matches the actual data, it ensures
// any existing draft file is removed and the test passes for this check.
func MatchSnapshot(t testReporter, actual any, snapshotFilepath string) {
	t.Helper()

	nextSnapshot := pretty.Sprint(actual)

	prevSnapshotBytes, err := os.ReadFile(snapshotFilepath)
	if errors.Is(err, os.ErrNotExist) {
		writeSnapshotDraft(t, snapshotFilepath, nextSnapshot)
		t.Errorf("Snapshot file %q does not exist", snapshotFilepath)
		return
	} else if err != nil {
		t.Error(err)
		return
	}

	if diff := cmp.Diff(string(prevSnapshotBytes), nextSnapshot); diff != "" {
		writeSnapshotDraft(t, snapshotFilepath, nextSnapshot)
		t.Errorf("Snapshot file %q is different from actual data:\n%s", snapshotFilepath, diff)
	} else {
		removeSnapshotDraft(snapshotFilepath)
	}
}

// AssertEquals checks if got and want are deeply equal. If not, it fails the test.
func AssertEquals(t *testing.T, got interface{}, want interface{}) {
	t.Helper()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("got != want (-want +got):\n%s", diff)
	}
}

// AssertErrorMatch verifies that the gotErr matches the wantErr type and message.
func AssertErrorMatch(t *testing.T, gotErr, wantErr error) {
	t.Helper()
	if gotErr == nil && wantErr == nil {
		return
	}
	if gotErr == nil || wantErr == nil {
		t.Errorf("Errors mismatch, want %v, got %v", wantErr, gotErr)
		return
	}
	if reflect.TypeOf(gotErr) != reflect.TypeOf(wantErr) || gotErr.Error() != wantErr.Error() {
		t.Errorf("Unexpected error, want %v, got %v", wantErr, gotErr)
	}
}

// AssertErrorContains verifies that the gotErr contains the wantErr message.
func AssertErrorContains(t *testing.T, gotErr, wantErr error) {
	t.Helper()
	if gotErr == nil && wantErr == nil {
		return
	}
	if gotErr == nil || wantErr == nil {
		t.Errorf("Errors mismatch, want %v, got %v", wantErr, gotErr)
		return
	}
	if !strings.Contains(gotErr.Error(), wantErr.Error()) {
		t.Errorf("Unexpected error, want error containing %q, got %q", wantErr.Error(), gotErr.Error())
	}
}

// AssertFilePath verifies that the file path base matches the expected path base.
func AssertFilePath(t *testing.T, gotPath string, wantPath string) {
	t.Helper()
	if wantPath == "" {
		if gotPath != "" {
			t.Errorf("unexpected path: got %q, want empty", gotPath)
		}
		return
	}
	if diff := cmp.Diff(wantPath, filepath.Base(gotPath)); diff != "" {
		t.Errorf("unexpected path (-want +got):\n%s", diff)
	}
}

// AssertFileContents verifies that the file at filePath matches the expected contents.
func AssertFileContents(t *testing.T, filePath string, wantContents string) {
	t.Helper()
	if filePath == "" {
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %q: %v", filePath, err)
	}
	if diff := cmp.Diff(wantContents, string(data)); diff != "" {
		t.Errorf("File contents mismatch (-want +got):\n%s", diff)
	}
}

// OverrideVariable overrides the value of a variable and returns a function to restore it.
func OverrideVariable[T any](ptr *T, val T) func() {
	original := *ptr
	*ptr = val
	return func() {
		*ptr = original
	}
}
