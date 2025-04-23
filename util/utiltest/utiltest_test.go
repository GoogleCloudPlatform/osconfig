package utiltest

import (
	"fmt"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util"
	"github.com/google/go-cmp/cmp"
)

type TestReporterSpy struct {
	calls []string
}

func (m *TestReporterSpy) Helper() {
}

func (m *TestReporterSpy) Logf(format string, args ...interface{}) {
	m.calls = append(m.calls, fmt.Sprintf("[Logf] "+format, args...))
}

func (m *TestReporterSpy) Errorf(format string, args ...interface{}) {
	m.calls = append(m.calls, fmt.Sprintf("[Errorf] "+format, args...))
}

func (m *TestReporterSpy) Error(args ...any) {
	m.calls = append(m.calls, "[Error] "+fmt.Sprintln(args...))
}

func Test_NoSnapshot_FailsAndWritesDraft(t *testing.T) {
	// Given snapshot does not exist on disk
	snapshotFilepath := "./does-not-exist"
	defer os.Remove(makeSnapshotDraftFilepath(snapshotFilepath))

	spyT := &TestReporterSpy{}

	// When matching a non-existent snapshot
	MatchSnapshot(spyT, struct{}{}, snapshotFilepath)
	expectedSnapshotContent := "struct {}{}"

	// Then a draft snapshot file is written for review
	bytes, err := os.ReadFile(makeSnapshotDraftFilepath(snapshotFilepath))
	if err != nil {
		t.Error(err)
		return
	}
	if diff := cmp.Diff(string(bytes), expectedSnapshotContent); diff != "" {
		t.Errorf("unexpected draft snapshot content, diff: %s", diff)
	}

	// And test is marked as failed
	if diff := cmp.Diff(spyT.calls, []string{
		"[Logf] Remove \".draft\" suffix from \"./does-not-exist.draft\" actual data snapshot to make test pass.",
		"[Errorf] Snapshot file \"./does-not-exist\" does not exist",
	}); diff != "" {
		t.Errorf("unexpected testing.T calls diff: %s", diff)
	}
}

func Test_ExistingEqualSnapshot_Passes(t *testing.T) {
	// Given snapshot is present on disk
	snapshotFilepath := "./existing-equal-snapshot"
	snapshotContent := "struct {}{}"
	os.WriteFile(snapshotFilepath, []byte(snapshotContent), 0644)
	defer os.Remove(snapshotFilepath)

	spyT := &TestReporterSpy{}

	// When matching an existing equal snapshot
	MatchSnapshot(spyT, struct{}{}, snapshotFilepath)

	// Then a present snapshot remains unchanged
	bytes, err := os.ReadFile(snapshotFilepath)
	if err != nil {
		t.Error(err)
		return
	}
	if diff := cmp.Diff(string(bytes), snapshotContent); diff != "" {
		t.Errorf("unexpected snapshot file content diff: %s", diff)
	}
	// And there is no draft snapshot created
	if util.Exists(makeSnapshotDraftFilepath(snapshotFilepath)) {
		t.Errorf("expected draft snapshot file does not exist")
	}

	// And test passes
	if len(spyT.calls) > 0 {
		t.Errorf("unexpected testing.T calls: %v", spyT.calls)
	}
}

func Test_ExistingNonEqualSnapshot_FailsAndWritesDraft(t *testing.T) {
	// Given snapshot is present on disk
	snapshotFilepath := "./existing-non-equal-snapshot"
	snapshotContent := "struct {}{}"
	os.WriteFile(snapshotFilepath, []byte(snapshotContent), 0644)
	defer os.Remove(snapshotFilepath)
	defer os.Remove(makeSnapshotDraftFilepath(snapshotFilepath))

	spyT := &TestReporterSpy{}

	// When matching an existing non-equal snapshot
	type NamedStruct struct{}
	MatchSnapshot(spyT, NamedStruct{}, snapshotFilepath)
	expectedSnapshotContent := "utiltest.NamedStruct{}"

	// Then a present snapshot remains unchanged
	bytes, err := os.ReadFile(snapshotFilepath)
	if err != nil {
		t.Error(err)
		return
	}
	if diff := cmp.Diff(string(bytes), snapshotContent); diff != "" {
		t.Errorf("unexpected snapshot file content diff: %s", diff)
	}

	// And draft snapshot is written for review
	draftBytes, err := os.ReadFile(makeSnapshotDraftFilepath(snapshotFilepath))
	if diff := cmp.Diff(string(draftBytes), expectedSnapshotContent); diff != "" {
		t.Errorf("unexpected draft snapshot content, diff: %s, err: %v", diff, err)
	}

	// And test is marked as failed
	if diff := cmp.Diff(spyT.calls, []string{
		"[Logf] Remove \".draft\" suffix from \"./existing-non-equal-snapshot.draft\" actual data snapshot to make test pass.",
		"[Errorf] Snapshot file \"./existing-non-equal-snapshot\" is different from actual data:\n" + cmp.Diff(snapshotContent, expectedSnapshotContent),
	}); diff != "" {
		t.Errorf("unexpected testing.T calls diff: %s", diff)
	}
}

func Test_ExistingEqualSnapshot_PassesAndRemovesOutdatedDraft(t *testing.T) {
	// Given snapshot is present on disk
	snapshotFilepath := "./existing-equal-snapshot"
	snapshotContent := "struct {}{}"
	os.WriteFile(snapshotFilepath, []byte(snapshotContent), 0644)
	defer os.Remove(snapshotFilepath)
	// And outdated draft snapshot present on disk
	draftSnapshotContent := "utiltest.NamedStruct{}"
	os.WriteFile(makeSnapshotDraftFilepath(snapshotFilepath), []byte(draftSnapshotContent), 0644)
	defer os.Remove(makeSnapshotDraftFilepath(snapshotFilepath))

	spyT := &TestReporterSpy{}

	// When matching an existing equal snapshot
	MatchSnapshot(spyT, struct{}{}, snapshotFilepath)

	// Then outdated draft snapshot is removed
	if util.Exists(makeSnapshotDraftFilepath(snapshotFilepath)) {
		t.Errorf("expected draft snapshot file does not exist")
	}

	// And test passes
	if len(spyT.calls) > 0 {
		t.Errorf("unexpected testing.T calls: %v", spyT.calls)
	}
}
