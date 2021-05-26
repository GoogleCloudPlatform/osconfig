package ospatch

import (
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
)

func TestRunFilter(t *testing.T) {
	patches, pkgUpdates, pkgToPatchesMap := prepareTestCase()
	type input struct {
		patches           []*packages.ZypperPatch
		pkgUpdates        []*packages.PkgInfo
		pkgToPatchesMap   map[string][]string
		exclusiveIncludes []string
		excludes          []string
		withUpdate        bool
	}
	type expect struct {
		patches    []string
		pkgUpdates []string
		err        error
	}

	tests := []struct {
		name   string
		input  input
		expect expect
	}{
		{name: "runfilterwithexclusivepatches",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{"patch-3"}, excludes: []string{}, withUpdate: false},
			expect: expect{patches: []string{"patch-3"}, pkgUpdates: []string{}, err: nil},
		},
		{name: "runFilterwithUpdatewithexcludes",
			// withupdate, exclude a patch that has
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []string{"patch-3"}, withUpdate: true},
			expect: expect{patches: []string{"patch-1", "patch-2"}, pkgUpdates: []string{"pkg6"}, err: nil},
		},
		{name: "runFilterwithoutUpdatewithexcludes",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []string{"patch-3"}, withUpdate: false},
			expect: expect{patches: []string{"patch-1", "patch-2"}, pkgUpdates: []string{}, err: nil},
		},
		{name: "runFilterwithUpdatewithoutexcludes",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []string{}, withUpdate: true},
			expect: expect{patches: []string{"patch-1", "patch-2", "patch-3"}, pkgUpdates: []string{"pkg6"}, err: nil},
		},
		{name: "runFilterwithoutUpdatewithoutexcludes",
			input:  input{patches: patches, pkgUpdates: pkgUpdates, pkgToPatchesMap: pkgToPatchesMap, exclusiveIncludes: []string{}, excludes: []string{}, withUpdate: false},
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
