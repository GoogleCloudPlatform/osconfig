package recipes

import (
	"context"
	"fmt"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

const (
	UP_TO_DATE = "UP_TO_DATE"
)

type RecipeDB struct{}

func newRecipeDB() RecipeDB {
	return RecipeDB{}
}

type Recipe struct {
	Version string
}

func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	return Recipe{}, false
}

// anything implementing isSoftwareRecipe_Step_Step() can be used as type isSoftware
func RunStep(step *osconfigpb.SoftwareRecipe_Step, artifacts map[string]string) error {
	switch v := step.Step.(type) {
	case *osconfigpb.SoftwareRecipe_Step_FileCopy:
		StepFileCopy(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction:
		StepArchiveExtraction(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_MsiInstallation:
		StepMsiInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_DpkgInstallation:
		StepDpkgInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_RpmInstallation:
		StepRpmInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_FileExec:
		StepFileExec(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ScriptRun:
		StepScriptRun(v, artifacts)
	default:
		fmt.Printf("I don't know about type %T!\n", v)
	}

	return nil
}

func InstallRecipe(ctx context.Context, recipe osconfigpb.SoftwareRecipe) error {
	fmt.Printf("Trying to install recipe %s:%v\n", recipe.Name, recipe.Version)
	steps := recipe.InstallSteps
	recipeDB := newRecipeDB()
	installed, ok := recipeDB.GetRecipe(recipe.Name)
	if ok {
		if (recipe.Version > installed.Version) && (recipe.DesiredState == osconfigpb.DesiredState_UPDATED) {
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
	artifacts map[string]string) {
	fmt.Println("StepFileCopy")
}
func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction,
	artifacts map[string]string) {
	fmt.Println("StepArchiveExtraction")
}
func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation,
	artifacts map[string]string) {
	fmt.Println("StepMsiInstallation")
}
func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation,
	artifacts map[string]string) {
	fmt.Println("StepDpkgInstallation")
}
func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation,
	artifacts map[string]string) {
	fmt.Println("StepRpmInstallation")
}
func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec,
	artifacts map[string]string) {
	realstep := step.FileExec
	switch v := realstep.LocationType.(type) {
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_LocalPath:
		fmt.Printf("Running local executable \"%s\" with args %v\n", v.LocalPath, realstep.Args)
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId:
		fmt.Printf("Running ArtifactId with args %v\n", realstep.Args)
	default:
		fmt.Println("can't figure out the location type")
	}
}
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun,
	artifacts map[string]string) {
	fmt.Println("StepScriptRun")
}

func FetchArtifacts([]*osconfigpb.SoftwareRecipe_Artifact) (map[string]string, error) {
	return map[string]string{}, nil
}
