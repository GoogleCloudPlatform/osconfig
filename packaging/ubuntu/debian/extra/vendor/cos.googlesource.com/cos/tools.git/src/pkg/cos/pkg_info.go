package cos

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

const packageInfoDefaultJSONFile = "/etc/cos-package-info.json"

// Package represents a COS package.
type Package struct {
	Category      string
	Name          string
	Version       string
	EbuildVersion string
}

// PackageInfo contains information about the packages of a COS instance.
type PackageInfo struct {
	InstalledPackages []Package
	BuildTimePackages []Package
}

type packageJSON struct {
	Category      string `json:"category"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	EbuildVersion string `json:"ebuild_version"`
}

type packageInfoJSON struct {
	InstalledPackages []packageJSON `json:"installedPackages"`
	BuildTimePackages []packageJSON `json:"buildTimePackages"`
}

// PackageInfoExists returns whether COS package information exists.
func PackageInfoExists() bool {
	info, err := os.Stat(packageInfoDefaultJSONFile)
	return !os.IsNotExist(err) && !info.IsDir()
}

// GetPackageInfo loads the package information from this COS system and returns
// it.
func GetPackageInfo() (PackageInfo, error) {
	return GetPackageInfoFromFile(packageInfoDefaultJSONFile)
}

// GetPackageInfoFromFile loads the package information from the specified file
// and returns it.
func GetPackageInfoFromFile(filename string) (PackageInfo, error) {
	var packageInfo PackageInfo

	f, err := os.Open(filename)
	if err != nil {
		return packageInfo, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return packageInfo, err
	}

	var piJSON packageInfoJSON
	if err = json.Unmarshal(b, &piJSON); err != nil {
		return packageInfo, err
	}

	packageInfo.InstalledPackages = make([]Package, len(piJSON.InstalledPackages))
	for i := range piJSON.InstalledPackages {
		pJSON := &piJSON.InstalledPackages[i]
		p := &packageInfo.InstalledPackages[i]

		p.Category = pJSON.Category
		p.Name = pJSON.Name
		p.Version = pJSON.Version
		p.EbuildVersion = pJSON.EbuildVersion
	}

	packageInfo.BuildTimePackages = make([]Package, len(piJSON.BuildTimePackages))
	for i := range piJSON.BuildTimePackages {
		pJSON := &piJSON.BuildTimePackages[i]
		p := &packageInfo.BuildTimePackages[i]

		p.Category = pJSON.Category
		p.Name = pJSON.Name
		p.Version = pJSON.Version
		p.EbuildVersion = pJSON.EbuildVersion
	}

	return packageInfo, nil
}
