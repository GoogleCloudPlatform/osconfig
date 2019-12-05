package test_classes

import (
	"fmt"

	customError "github.com/GoogleCloudPlatform/osconfig/e2etester/error"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
)

type ZypperTest struct {
}

func (z ZypperTest) TestZypperCommands() error {

	ec := new(customError.ErrorCollector)
	// list installed packages
	pkgs, err := packages.ZypperInstalledPatches()
	if err != nil || len(pkgs) == 0 {
		ec.Collect(fmt.Errorf("Error listing patches: %+v\n", err))
	}

	// install a package
	err = packages.InstallZypperPackages([]string{"xeyes"})
	if err != nil {
		ec.Collect(fmt.Errorf("error installing package: +%v", err))
	}

	// remove the same package that we just installed
	err = packages.RemoveZypperPackages([]string{"xeyes"})
	if err != nil {
		ec.Collect(fmt.Errorf("Error removing package: %+v", err))
	}

	count, errs := ec.Error()
	fmt.Printf("errors generated: %+v\n", errs)
	if count == 0 {
		return nil
	}
	return errs
}
