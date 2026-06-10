package utiltest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/kr/pretty"
)

// AssertFormatMatch verifies that the got matches the wantFormat regular expression.
func AssertFormatMatch(t *testing.T, got string, wantFormat string) {
	t.Helper()
	matched, err := regexp.MatchString(wantFormat, got)
	if err != nil {
		t.Fatalf("regexp.MatchString(%q, %q) err: %v", wantFormat, got, err)
	}
	if !matched {
		t.Errorf("Format mismatch, want %q, got %q", wantFormat, got)
	}
}

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

// ExpectedCommand defines a reusable expected command call
type ExpectedCommand struct {
	Cmd    *exec.Cmd
	Envs   []string
	Stdout []byte
	Stderr []byte
	Err    error
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
	assertErrorMatch(t, gotErr, wantErr, false, false)
}

// AssertErrorMatchAndFail verifies that the gotErr matches the wantErr type and message,
// and fails the test immediately if they don't match.
func AssertErrorMatchAndFail(t *testing.T, gotErr, wantErr error) {
	t.Helper()
	assertErrorMatch(t, gotErr, wantErr, true, false)
}

// AssertErrorMatchAndSkip verifies that the gotErr matches the wantErr type and message,
// and skips the further test step immediately if they match.
func AssertErrorMatchAndSkip(t *testing.T, gotErr, wantErr error) {
	t.Helper()
	assertErrorMatch(t, gotErr, wantErr, false, true)
}

func assertErrorMatch(t *testing.T, gotErr, wantErr error, failNow bool, skipNow bool) {
	t.Helper()
	if gotErr == nil && wantErr == nil {
		return
	}
	if gotErr == nil || wantErr == nil || reflect.TypeOf(gotErr) != reflect.TypeOf(wantErr) || gotErr.Error() != wantErr.Error() {
		t.Errorf("Errors mismatch, want %v, got %v", wantErr, gotErr)
		if failNow {
			t.FailNow()
		}
	}
	if skipNow {
		t.SkipNow()
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
func OverrideVariable[T any](t *testing.T, ptr *T, val T) {
	original := *ptr
	*ptr = val
	t.Cleanup(func() { *ptr = original })
}

// SetExpectedCommands sets expected result for provided mock commands
func SetExpectedCommands(ctx context.Context, mockCommandRunner *utilmocks.MockCommandRunner, expectedCommands []ExpectedCommand) {
	if len(expectedCommands) == 0 {
		return
	}

	var prev *gomock.Call
	for _, expectedCommand := range expectedCommands {
		if expectedCommand.Envs != nil {
			expectedCommand.Cmd.Env = append(os.Environ(), expectedCommand.Envs...)
		}
		call := mockCommandRunner.EXPECT().
			Run(ctx, utilmocks.EqCmd(expectedCommand.Cmd)).
			Return(expectedCommand.Stdout, expectedCommand.Stderr, expectedCommand.Err).
			Times(1)
		if prev != nil {
			call.After(prev)
		}
		prev = call
	}
}

// WriteToTempFileMust writes content to a temporary file. If content is nil, it only returns the path where the file would be located.
// It fails the test if any error occurs.
func WriteToTempFileMust(t *testing.T, filename string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), filename)
	if content != nil {
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to write to temp file %q: %v", path, err)
		}
	}
	return path
}
