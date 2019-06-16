package recipes

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

type RecipeDB struct{}

func newRecipeDB() RecipeDB {
	return RecipeDB{}
}

type Recipe struct {
	version []int
}

func convertVersion(version string) ([]int, error) {
	if version == "" {
		return []int{0}, nil
	}
	var ret []int
	for idx, element := range strings.Split(version, ".") {
		if idx > 3 {
			return nil, fmt.Errorf("Invalid version string")
		}
		val, err := strconv.ParseUint(element, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("Invalid version string")
		}
		ret = append(ret, int(val))
	}
	return ret, nil
}

func (r *Recipe) SetVersion(version string) error {
	var err error
	r.version, err = convertVersion(version)
	return err
}

// Greater returns a boolean indicating whether the provided version argument
// is greater than the recipe's version.
func (r *Recipe) Greater(version string) bool {
	if version == "" {
		return false
	}
	cVersion, err := convertVersion(version)
	if err != nil {
		return false
	}
	if len(r.version) > len(cVersion) {
		topad := len(r.version) - len(cVersion)
		for i := 0; i < topad; i++ {
			cVersion = append(cVersion, 0)
		}
	} else {
		topad := len(cVersion) - len(r.version)
		for i := 0; i < topad; i++ {
			r.version = append(r.version, 0)
		}
	}
	for i := 0; i < len(r.version); i++ {
		if r.version[i] != cVersion[i] {
			return cVersion[i] > r.version[i]
		}
	}
	return false
}

func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	return Recipe{}, false
}

// anything implementing isSoftwareRecipe_Step_Step() can be used as type isSoftware
func RunStep(step *osconfigpb.SoftwareRecipe_Step, artifacts map[string]string) error {
	switch v := step.Step.(type) {
	case *osconfigpb.SoftwareRecipe_Step_FileCopy:
		return StepFileCopy(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction:
		return StepArchiveExtraction(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_MsiInstallation:
		return StepMsiInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_DpkgInstallation:
		return StepDpkgInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_RpmInstallation:
		return StepRpmInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_FileExec:
		return StepFileExec(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ScriptRun:
		return StepScriptRun(v, artifacts)
	default:
		return fmt.Errorf("I don't know about step type %T!\n", v)
	}
}

func InstallRecipe(ctx context.Context, recipe osconfigpb.SoftwareRecipe) error {
	steps := recipe.InstallSteps
	recipeDB := newRecipeDB()
	installedRecipe, ok := recipeDB.GetRecipe(recipe.Name)
	if ok {
		if (!installedRecipe.Greater(recipe.Version)) &&
			(recipe.DesiredState == osconfigpb.DesiredState_UPDATED) {
			steps = recipe.UpdateSteps
		} else {
			// log skipping recipe
			return nil
		}
	}
	artifacts, err := FetchArtifacts(recipe.Artifacts)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if err := RunStep(step, artifacts); err != nil {
			return err
		}
	}
	return nil
}

func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy,
	artifacts map[string]string) error {
	fmt.Println("StepFileCopy")
	return nil
}
func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction,
	artifacts map[string]string) error {
	fmt.Println("StepArchiveExtraction")
	return nil
}
func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation,
	artifacts map[string]string) error {
	fmt.Println("StepMsiInstallation")
	return nil
}
func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation,
	artifacts map[string]string) error {
	fmt.Println("StepDpkgInstallation")
	return nil
}
func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation,
	artifacts map[string]string) error {
	fmt.Println("StepRpmInstallation")
	return nil
}
func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec,
	artifacts map[string]string) error {
	var path string
	realstep := step.FileExec
	switch v := realstep.LocationType.(type) {
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_LocalPath:
		path = v.LocalPath
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId:
		var ok bool
		path, ok = artifacts[v.ArtifactId]
		if !ok {
			return fmt.Errorf("Artifact ID %q not found in artifact map", v.ArtifactId)
		}
	default:
		return fmt.Errorf("Can't figure out the location type for this artifact")
	}
	err := exec.Command(path, realstep.Args...).Run()
	if err != nil {
		return err
	}
	return nil
}
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun,
	artifacts map[string]string) error {
	fmt.Println("StepScriptRun")
	return nil
}

func FetchArtifacts(recipeArtifacts []*osconfigpb.SoftwareRecipe_Artifact) (map[string]string, error) {
	artifacts := make(map[string]string)
	for _, artifact := range recipeArtifacts {
		artifacts[artifact.Id] = artifact.Uri
	}
	return artifacts, nil
}
