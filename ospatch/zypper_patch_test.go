package ospatch

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	utiltest "github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestRunFilter(t *testing.T) {
	patches, pkgUpdates, pkgToPatchesMap := prepareTestCase()
	type input struct {
		patches           []*packages.ZypperPatch
		pkgUpdates        []*packages.PkgInfo
		pkgToPatchesMap   map[string][]string
		exclusiveIncludes []string
		excludes          []*Exclude
		withUpdate        bool
	}
	type expect struct {
		patches    []string
		pkgUpdates []string
		err        error
	}

	var patch3String = "patch-3"

	tests := []struct {
		name   string
		input  input
		expect expect
	}{
		{name: "runfilterwithexclusivepatches",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{"patch-3"}, excludes: []*Exclude{}, withUpdate: false},
			expect: expect{patches: []string{"patch-3"}, pkgUpdates: []string{}, err: nil},
		},
		{name: "runFilterwithUpdatewithexcludes",
			// withupdate, exclude a patch that has
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []*Exclude{CreateStringExclude(&patch3String)}, withUpdate: true},
			expect: expect{patches: []string{"patch-1", "patch-2"}, pkgUpdates: []string{"pkg6"}, err: nil},
		},
		{name: "runFilterwithoutUpdatewithexcludes",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []*Exclude{CreateStringExclude(&patch3String)}, withUpdate: false},
			expect: expect{patches: []string{"patch-1", "patch-2"}, pkgUpdates: []string{}, err: nil},
		},
		{name: "runFilterwithUpdatewithoutexcludes",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []*Exclude{}, withUpdate: true},
			expect: expect{patches: []string{"patch-1", "patch-2", "patch-3"}, pkgUpdates: []string{"pkg6"}, err: nil},
		},
		{name: "runFilterwithoutUpdatewithoutexcludes",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []*Exclude{}, withUpdate: false},
			expect: expect{patches: []string{"patch-1", "patch-2", "patch-3"}, pkgUpdates: []string{}, err: nil},
		},
	}

	for _, tc := range tests {
		fPatches, fpkgs, err := runFilter(tc.input.patches, tc.input.exclusiveIncludes, tc.input.excludes, tc.input.pkgUpdates, tc.input.pkgToPatchesMap, tc.input.withUpdate)
		if err != nil {
			t.Errorf("[%s] unexpected error: got(%+v)", tc.name, err)
			continue
		}
		if len(fPatches) != len(tc.expect.patches) {
			t.Errorf("[%s] unexpected number of patches: expected(%d), got(%d)", tc.name, len(tc.expect.patches), len(fPatches))
		}
		for _, p := range fPatches {
			if !isIn(p.Name, tc.expect.patches) {
				t.Errorf("[%s] unexpected patch name: (%s)! is not in %+v", tc.name, p.Name, tc.expect.patches)
			}
		}
		if len(fpkgs) != len(tc.expect.pkgUpdates) {
			t.Errorf("[%s] unexpected number of packages: expected(%d), got(%d)", tc.name, len(tc.expect.pkgUpdates), len(fpkgs))
		}
		for _, p := range fpkgs {
			if !isIn(p.Name, tc.expect.pkgUpdates) {
				t.Errorf("[%s] unexpected package name: (%s)! is not in %+v", tc.name, p.Name, tc.expect.pkgUpdates)
			}
		}
	}
}

func TestRunZypperPatch(t *testing.T) {
	const zypperBin = "/usr/bin/zypper"
	someErr := errors.New("some error")
	patch2 := "patch-2"

	listPatchesBaseArgs := []string{"--gpg-auto-import-keys", "-q", "list-patches"}
	listPatchesAllArgs := append(listPatchesBaseArgs, "--all")
	listUpdatesArgs := []string{"--gpg-auto-import-keys", "-q", "list-updates"}
	installArgs := []string{"--gpg-auto-import-keys", "--non-interactive", "install", "--auto-agree-with-licenses"}

	onePatchOutput := []byte(`SLE-Module | patch-1 | security | important | --- | needed | Security patch`)
	twoPatchesOutput := []byte("SLE-Module | patch-1 | security | important | --- | needed | Security patch\n" +
		"SLE-Module | patch-2 | recommended | moderate | --- | needed | Recommended patch")
	oneUpdateOutput := []byte(`v | SLES12-SP3-Updates  | pkg1 | 1.0.0 | 2.0.0 | x86_64`)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })
	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	tests := []struct {
		name      string
		opts      []ZypperPatchOption
		setupMock func(ctx context.Context, mock *utilmocks.MockCommandRunner)
		wantErr   error
	}{
		{
			name: "When listing available patches fails, RunZypperPatch should surface the wrapped command error and not attempt to install anything.",
			opts: nil,
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return(nil, nil, someErr).Times(1)
			},
			wantErr: wrapRunErr(zypperBin, listPatchesAllArgs, someErr),
		},
		{
			name: "When zypper reports no needed patches and --with-update is not set, RunZypperPatch should return without running any install commands.",
			opts: nil,
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return([]byte(""), nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "In dry-run mode, RunZypperPatch should list available patches but skip the install step even when patches are needed.",
			opts: []ZypperPatchOption{ZypperUpdateDryrun(true)},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return(onePatchOutput, nil, nil).Times(1)
				// No install call expected.
			},
			wantErr: nil,
		},
		{
			name: "With one needed patch reported by zypper, RunZypperPatch should invoke zypper install with the patch:<name> argument.",
			opts: nil,
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				listCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return(onePatchOutput, nil, nil).Times(1)
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, append(installArgs, "patch:patch-1")...))).
					After(listCall).Return(nil, nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "When the install step fails, RunZypperPatch should return the wrapped command error from the install invocation.",
			opts: nil,
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				listCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return(onePatchOutput, nil, nil).Times(1)
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, append(installArgs, "patch:patch-1")...))).
					After(listCall).Return(nil, nil, someErr).Times(1)
			},
			wantErr: wrapRunErr(zypperBin, append(installArgs, "patch:patch-1"), someErr),
		},
		{
			name: "With --with-update enabled and no patches to install, a failure of `zypper list-updates` should be surfaced as the wrapped command error.",
			opts: []ZypperPatchOption{ZypperUpdateWithUpdate(true)},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				// Empty patches list so ZypperPackagesInPatch short-circuits.
				listCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return([]byte(""), nil, nil).Times(1)
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listUpdatesArgs...))).
					After(listCall).Return(nil, nil, someErr).Times(1)
			},
			wantErr: wrapRunErr(zypperBin, listUpdatesArgs, someErr),
		},
		{
			name: "With --with-update enabled and no needed patches, non-patch packages reported by `zypper list-updates` should be installed using the package:<name> form.",
			opts: []ZypperPatchOption{ZypperUpdateWithUpdate(true)},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				// Empty patches list so ZypperPackagesInPatch short-circuits (returns empty map, nil).
				listPatchesCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return([]byte(""), nil, nil).Times(1)
				listUpdatesCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listUpdatesArgs...))).
					After(listPatchesCall).Return(oneUpdateOutput, nil, nil).Times(1)
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, append(installArgs, "package:pkg1")...))).
					After(listUpdatesCall).Return(nil, nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "When a category filter is provided, list-patches should be invoked with --category=<value> and without --all.",
			opts: []ZypperPatchOption{ZypperPatchCategories([]string{"security"})},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				// With a category filter, --all is NOT appended.
				args := append(listPatchesBaseArgs, "--category=security")
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, args...))).
					Return([]byte(""), nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "When a severity filter is provided, list-patches should be invoked with --severity=<value> and without --all.",
			opts: []ZypperPatchOption{ZypperPatchSeverities([]string{"critical"})},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				// With a severity filter, --all is NOT appended.
				args := append(listPatchesBaseArgs, "--severity=critical")
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, args...))).
					Return([]byte(""), nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "When --with-optional is enabled, list-patches should be invoked with --with-optional alongside --all.",
			opts: []ZypperPatchOption{ZypperUpdateWithOptional(true)},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				args := append(listPatchesBaseArgs, "--with-optional", "--all")
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, args...))).
					Return([]byte(""), nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "With an exclusive-patches list, only the patches named in that list should be passed to the install invocation, even if zypper reports additional needed patches.",
			opts: []ZypperPatchOption{ZypperUpdateWithExclusivePatches([]string{"patch-1"})},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				listCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return(twoPatchesOutput, nil, nil).Times(1)
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, append(installArgs, "patch:patch-1")...))).
					After(listCall).Return(nil, nil, nil).Times(1)
			},
			wantErr: nil,
		},
		{
			name: "With an excludes list, the excluded patches should not be passed to the install invocation.",
			opts: []ZypperPatchOption{ZypperUpdateWithExcludes([]*Exclude{CreateStringExclude(&patch2)})},
			setupMock: func(ctx context.Context, mock *utilmocks.MockCommandRunner) {
				listCall := mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, listPatchesAllArgs...))).
					Return(twoPatchesOutput, nil, nil).Times(1)
				mock.EXPECT().
					Run(ctx, utilmocks.EqCmd(exec.Command(zypperBin, append(installArgs, "patch:patch-1")...))).
					After(listCall).Return(nil, nil, nil).Times(1)
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			tc.setupMock(ctx, mockCommandRunner)

			err := RunZypperPatch(ctx, tc.opts...)
			utiltest.AssertErrorMatch(t, err, tc.wantErr)
		})
	}
}

func isIn(needle string, haystack []string) bool {
	for _, hay := range haystack {
		if strings.Compare(hay, needle) == 0 {
			return true
		}
	}
	return false
}

func prepareTestCase() ([]*packages.ZypperPatch, []*packages.PkgInfo, map[string][]string) {
	var patches []*packages.ZypperPatch
	var pkgUpdates []*packages.PkgInfo
	var pkgToPatchesMap map[string][]string
	patches = append(patches, &packages.ZypperPatch{
		Name:     "patch-1",
		Category: "recommended",
		Severity: "important",
		Summary:  "patch-1",
	})
	patches = append(patches, &packages.ZypperPatch{
		Name:     "patch-2",
		Category: "security",
		Severity: "critical",
		Summary:  "patch-2",
	})
	patches = append(patches, &packages.ZypperPatch{
		Name:     "patch-3",
		Category: "optional",
		Severity: "low",
		Summary:  "patch-3",
	})

	pkgUpdates = append(pkgUpdates, &packages.PkgInfo{
		Name:    "pkg1",
		Arch:    "noarch",
		Version: "1.1.1",
	})
	pkgUpdates = append(pkgUpdates, &packages.PkgInfo{
		Name:    "pkg2",
		Arch:    "noarch",
		Version: "1.1.1",
	})
	pkgUpdates = append(pkgUpdates, &packages.PkgInfo{
		Name:    "pkg3",
		Arch:    "noarch",
		Version: "1.1.1",
	})
	pkgUpdates = append(pkgUpdates, &packages.PkgInfo{
		Name:    "pkg4",
		Arch:    "noarch",
		Version: "1.1.1",
	})
	pkgUpdates = append(pkgUpdates, &packages.PkgInfo{
		Name:    "pkg5",
		Arch:    "noarch",
		Version: "1.1.1",
	})
	// individual package update that is not a part
	// of a patch. this package only shows up
	// if user specifies --with-update
	pkgUpdates = append(pkgUpdates, &packages.PkgInfo{
		Name:    "pkg6",
		Arch:    "noarch",
		Version: "1.1.1",
	})

	pkgToPatchesMap = make(map[string][]string)
	pkgToPatchesMap["pkg1"] = []string{"patch-1"}
	pkgToPatchesMap["pkg2"] = []string{"patch-1"}
	pkgToPatchesMap["pkg3"] = []string{"patch-2"}
	pkgToPatchesMap["pkg4"] = []string{"patch-2"}
	pkgToPatchesMap["pkg5"] = []string{"patch-3"}

	return patches, pkgUpdates, pkgToPatchesMap
}

// wrapRunErr mirrors the formatting used by packages.run to wrap the
// underlying command error, so tests can express the exact expected error.
func wrapRunErr(cmd string, args []string, err error) error {
	return fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", cmd, args, err, []byte(""), []byte(""))
}
