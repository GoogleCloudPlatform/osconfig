package agentendpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/attributes"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/inventory"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	datepb "google.golang.org/genproto/googleapis/type/date"
)

const (
	inventoryURL = agentconfig.ReportURL + "/guestInventory"
)

// ReportInventory writes inventory to guest attributes and reports it to agent endpoint.
func (c *Client) ReportInventory(ctx context.Context) {
	state := c.inventoryProvider.Get(ctx)

	if agentconfig.GuestAttributesEnabled() && !agentconfig.DisableInventoryWrite() {
		clog.Infof(ctx, "Writing inventory to guest attributes")
		write(ctx, state, inventoryURL)
	}

	c.report(ctx, state)
}

func write(ctx context.Context, state *inventory.InstanceInventory, url string) {
	clog.Debugf(ctx, "Writing instance inventory to guest attributes.")

	e := reflect.ValueOf(state).Elem()
	t := e.Type()
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		u := fmt.Sprintf("%s/%s", url, t.Field(i).Name)
		switch f.Kind() {
		case reflect.String:
			clog.Debugf(ctx, "postAttribute %s: %+v", u, f)
			if err := attributes.PostAttribute(u, strings.NewReader(f.String())); err != nil {
				clog.Errorf(ctx, "postAttribute error: %v", err)
			}
		case reflect.Ptr:
			switch reflect.Indirect(f).Kind() {
			case reflect.Struct:
				clog.Debugf(ctx, "postAttributeCompressed %s", u)
				if err := attributes.PostAttributeCompressed(u, f.Interface()); err != nil {
					clog.Errorf(ctx, "postAttributeCompressed error: %v", err)
				}
			}
		}
	}
}

func (c *Client) report(ctx context.Context, state *inventory.InstanceInventory) {
	clog.Debugf(ctx, "Reporting instance inventory to agent endpoint.")
	inventory := formatInventory(ctx, state)
	vmInventory := formatVMInventory(ctx, state)

	reportFull := false
	var reportInventoryRes *agentendpointpb.ReportInventoryResponse
	var reportVMInventoryRes *agentendpointpb.ReportVmInventoryResponse
	var err error
	f := func() error {
		reportVMInventoryRes, err = c.reportVMInventory(ctx, vmInventory, reportFull)
		if shouldFallbackToLegacyAPI(err) {
			reportInventoryRes, err = c.reportInventory(ctx, inventory, reportFull)
		}

		if err != nil {
			return err
		}
		return nil
	}

	if err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "ReportInventory", f); err != nil {
		clog.Errorf(ctx, "Error reporting inventory checksum: %v", err)
		return
	}

	if shouldReportFullInventory(reportVMInventoryRes, reportInventoryRes) {
		reportFull = true
		if err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "ReportInventory", f); err != nil {
			clog.Errorf(ctx, "Error reporting full inventory: %v", err)
			return
		}
	}
}

func shouldFallbackToLegacyAPI(err error) bool {
	if st, ok := status.FromError(err); ok == true {
		return st.Code() == codes.FailedPrecondition
	}
	return false
}

func shouldReportFullInventory(reportVMInventoryRes *agentendpointpb.ReportVmInventoryResponse,
	reportInventoryRes *agentendpointpb.ReportInventoryResponse) bool {
	return reportVMInventoryRes.GetReportFullInventory() || reportInventoryRes.GetReportFullInventory()
}

func formatVMInventory(ctx context.Context, state *inventory.InstanceInventory) *agentendpointpb.VmInventory {
	osInfo := &agentendpointpb.VmInventory_OsInfo{
		HostName:             state.Hostname,
		LongName:             state.LongName,
		ShortName:            state.ShortName,
		Version:              state.Version,
		Architecture:         state.Architecture,
		KernelVersion:        state.KernelVersion,
		KernelRelease:        state.KernelRelease,
		OsconfigAgentVersion: state.OSConfigAgentVersion,
	}

	installedPackages := formatPkgsToInventoryItems(ctx, state.InstalledPackages)
	availablePackages := formatPkgsToInventoryItems(ctx, state.PackageUpdates)

	return &agentendpointpb.VmInventory{OsInfo: osInfo, InstalledPackages: installedPackages, AvailablePackages: availablePackages}
}

func formatPkgsToInventoryItems(ctx context.Context, pkgs *packages.Packages) []*agentendpointpb.VmInventory_InventoryItem {
	var softwarePackages []*agentendpointpb.VmInventory_InventoryItem
	if pkgs == nil {
		return softwarePackages
	}

	if pkgs.Yum != nil {
		softwarePackages = append(softwarePackages, yumToInventoryItem(pkgs.Yum)...)
	}
	if pkgs.Rpm != nil {
		softwarePackages = append(softwarePackages, rpmToInventoryItem(pkgs.Rpm)...)
	}
	if pkgs.Apt != nil {
		softwarePackages = append(softwarePackages, aptToInventoryItem(pkgs.Apt)...)
	}
	if pkgs.Deb != nil {
		softwarePackages = append(softwarePackages, debToInventoryItem(pkgs.Deb)...)
	}
	if pkgs.Zypper != nil {
		softwarePackages = append(softwarePackages, zypperToInventoryItem(pkgs.Zypper)...)
	}
	if pkgs.ZypperPatches != nil {
		softwarePackages = append(softwarePackages, zypperPatchToInventoryItem(pkgs.ZypperPatches)...)
	}
	if pkgs.COS != nil {
		softwarePackages = append(softwarePackages, cosToInventoryItem(pkgs.COS)...)
	}
	if pkgs.GooGet != nil {
		softwarePackages = append(softwarePackages, googetToInventoryItem(pkgs.GooGet)...)
	}
	if pkgs.WUA != nil {
		softwarePackages = append(softwarePackages, wuaToInventoryItem(pkgs.WUA)...)
	}
	if pkgs.QFE != nil {
		softwarePackages = append(softwarePackages, qfeToInventoryItem(pkgs.QFE)...)
	}
	if pkgs.WindowsApplication != nil {
		softwarePackages = append(softwarePackages, windowsApplicationToInventoryItem(pkgs.WindowsApplication)...)
	}
	return softwarePackages
}

func aptToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedApt := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedApt[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"SourceName":    structpb.NewStringValue(pkg.Source.Name),
				"SourceVersion": structpb.NewStringValue(pkg.Source.Version),
			}},
		}
	}
	return formattedApt
}

func debToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedDeb := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedDeb[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"SourceName":    structpb.NewStringValue(pkg.Source.Name),
				"SourceVersion": structpb.NewStringValue(pkg.Source.Version),
			}},
		}
	}
	return formattedDeb
}

func googetToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedGooGet := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedGooGet[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{}},
		}
	}
	return formattedGooGet
}

func yumToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedYum := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedYum[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"SourceRPM": structpb.NewStringValue(pkg.Source.Name),
			}},
		}
	}
	return formattedYum
}

func zypperToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedZypper := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedZypper[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"SourceRPM": structpb.NewStringValue(pkg.Source.Name),
			}},
		}
	}
	return formattedZypper
}

func rpmToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedRpm := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedRpm[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"SourceRPM": structpb.NewStringValue(pkg.Source.Name),
			}},
		}
	}
	return formattedRpm
}

func cosToInventoryItem(packages []*packages.PkgInfo) []*agentendpointpb.VmInventory_InventoryItem {
	formattedCos := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		formattedCos[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     pkg.Type,
			Version:  pkg.Version,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{}},
		}
	}
	return formattedCos
}

func zypperPatchToInventoryItem(packages []*packages.ZypperPatch) []*agentendpointpb.VmInventory_InventoryItem {
	zypperPatchFormattedPackages := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		zypperPatchFormattedPackages[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Name,
			Type:     "zypperPatch",
			Version:  "",
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"Category": structpb.NewStringValue(pkg.Category),
				"Severity": structpb.NewStringValue(pkg.Severity),
				"Summary":  structpb.NewStringValue(pkg.Summary),
			}},
		}
	}
	return zypperPatchFormattedPackages
}

func wuaToInventoryItem(packages []*packages.WUAPackage) []*agentendpointpb.VmInventory_InventoryItem {
	wuaFormattedPackages := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		categoriesList := formatToCategoriesList(pkg.CategoryIDs, pkg.Categories)
		kbArticleIdsList := formatToStructList(pkg.KBArticleIDs)
		moreInfoUrls := formatToStructList(pkg.MoreInfoURLs)
		categoryIds := formatToStructList(pkg.CategoryIDs)
		wuaFormattedPackages[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Title,
			Type:     "wuaPackage",
			Version:  pkg.UpdateID,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"Description":              structpb.NewStringValue(pkg.Description),
				"Categories":               structpb.NewListValue(categoriesList),
				"CategoryIds":              structpb.NewListValue(categoryIds),
				"KbArticleId":              structpb.NewListValue(kbArticleIdsList),
				"MoreInfoUrls":             structpb.NewListValue(moreInfoUrls),
				"RevisionNumber":           structpb.NewNumberValue(float64(pkg.RevisionNumber)),
				"LastDeploymentChangeTime": structpb.NewStringValue(pkg.LastDeploymentChangeTime.String()),
				"SupportUrl":               structpb.NewStringValue(pkg.SupportURL),
			}},
		}
	}
	return wuaFormattedPackages
}

func qfeToInventoryItem(packages []*packages.QFEPackage) []*agentendpointpb.VmInventory_InventoryItem {
	qfeFormattedPackages := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		qfeFormattedPackages[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.Caption,
			Type:     "qfePackage",
			Version:  pkg.HotFixID,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"Description": structpb.NewStringValue(pkg.Description),
				"InstalledOn": structpb.NewStringValue(pkg.InstalledOn),
			}},
		}
	}
	return qfeFormattedPackages
}

func windowsApplicationToInventoryItem(packages []*packages.WindowsApplication) []*agentendpointpb.VmInventory_InventoryItem {
	windowsApplicationFormattedPackages := make([]*agentendpointpb.VmInventory_InventoryItem, len(packages))
	for i, pkg := range packages {
		windowsApplicationFormattedPackages[i] = &agentendpointpb.VmInventory_InventoryItem{
			Name:     pkg.DisplayName,
			Type:     "windowsApplication",
			Version:  pkg.DisplayVersion,
			Purl:     pkg.Purl,
			Location: []string{},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"Publisher":   structpb.NewStringValue(pkg.Publisher),
				"InstallDate": structpb.NewStringValue(pkg.InstallDate.String()),
				"HelpLink":    structpb.NewStringValue(pkg.HelpLink),
			}},
		}
	}
	return windowsApplicationFormattedPackages
}

func formatToStructList(stringArray []string) *structpb.ListValue {
	var listAny []any
	for _, entry := range stringArray {
		listAny = append(listAny, entry)
	}
	structList, err := structpb.NewList(listAny)
	if err != nil {

	}
	return structList
}

func formatToCategoriesList(categoryIds []string, categoryNames []string) *structpb.ListValue {
	categoryList := &structpb.ListValue{}
	for i := range categoryIds {
		entry := map[string]any{
			"Id":   categoryIds[i],
			"Name": categoryNames[i],
		}

		v, err := structpb.NewValue(entry)
		if err != nil {
			continue
		}

		categoryList.Values = append(categoryList.Values, v)
	}
	return categoryList
}

func formatInventory(ctx context.Context, state *inventory.InstanceInventory) *agentendpointpb.Inventory {
	osInfo := &agentendpointpb.Inventory_OsInfo{
		Hostname:             state.Hostname,
		LongName:             state.LongName,
		ShortName:            state.ShortName,
		Version:              state.Version,
		Architecture:         state.Architecture,
		KernelVersion:        state.KernelVersion,
		KernelRelease:        state.KernelRelease,
		OsconfigAgentVersion: state.OSConfigAgentVersion,
	}
	installedPackages := formatPackages(ctx, state.InstalledPackages, state.ShortName)
	availablePackages := formatPackages(ctx, state.PackageUpdates, state.ShortName)

	return &agentendpointpb.Inventory{OsInfo: osInfo, InstalledPackages: installedPackages, AvailablePackages: availablePackages}
}

func formatPackages(ctx context.Context, pkgs *packages.Packages, shortName string) []*agentendpointpb.Inventory_SoftwarePackage {
	var softwarePackages []*agentendpointpb.Inventory_SoftwarePackage
	if pkgs == nil {
		return softwarePackages
	}
	if pkgs.Apt != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.Apt))
		for i, pkg := range pkgs.Apt {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatAptPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.Deb != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.Deb))
		for i, pkg := range pkgs.Deb {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatAptPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.GooGet != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.GooGet))
		for i, pkg := range pkgs.GooGet {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatGooGetPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.Yum != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.Yum))
		for i, pkg := range pkgs.Yum {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatYumPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.Zypper != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.Zypper))
		for i, pkg := range pkgs.Zypper {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatZypperPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.Rpm != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.Rpm))
		if packages.YumExists || !packages.ZypperExists {
			for i, pkg := range pkgs.Rpm {
				temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
					Details: formatYumPackage(pkg),
				}
			}
		} else {
			for i, pkg := range pkgs.Rpm {
				temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
					Details: formatZypperPackage(pkg),
				}
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.ZypperPatches != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.ZypperPatches))
		for i, pkg := range pkgs.ZypperPatches {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatZypperPatch(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.WUA != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.WUA))
		for i, pkg := range pkgs.WUA {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatWUAPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.QFE != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.QFE))
		for i, pkg := range pkgs.QFE {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatQFEPackage(ctx, pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.COS != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.COS))
		for i, pkg := range pkgs.COS {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatCOSPackage(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	if pkgs.WindowsApplication != nil {
		temp := make([]*agentendpointpb.Inventory_SoftwarePackage, len(pkgs.WindowsApplication))
		for i, pkg := range pkgs.WindowsApplication {
			temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
				Details: formatWindowsApplication(pkg),
			}
		}
		softwarePackages = append(softwarePackages, temp...)
	}
	// Ignore Pip and Gem packages.

	return softwarePackages
}

func formatAptPackage(pkg *packages.PkgInfo) *agentendpointpb.Inventory_SoftwarePackage_AptPackage {
	fPkg := &agentendpointpb.Inventory_SoftwarePackage_AptPackage{
		AptPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version,
		},
	}

	// for some of the APT packages source package might be available.
	if pkg.Source.Name != "" {
		fPkg.AptPackage.Source = &agentendpointpb.Inventory_VersionedPackage_Source{
			Name:    pkg.Source.Name,
			Version: pkg.Source.Version,
		}
	}

	return fPkg
}

func formatCOSPackage(pkg *packages.PkgInfo) *agentendpointpb.Inventory_SoftwarePackage_CosPackage {
	return &agentendpointpb.Inventory_SoftwarePackage_CosPackage{
		CosPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version,
		}}
}

func formatGooGetPackage(pkg *packages.PkgInfo) *agentendpointpb.Inventory_SoftwarePackage_GoogetPackage {
	return &agentendpointpb.Inventory_SoftwarePackage_GoogetPackage{
		GoogetPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version,
		}}
}

func formatYumPackage(pkg *packages.PkgInfo) *agentendpointpb.Inventory_SoftwarePackage_YumPackage {
	fPkg := &agentendpointpb.Inventory_SoftwarePackage_YumPackage{
		YumPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version,
		},
	}

	// for some of the YUM packages source package might be available.
	if pkg.Source.Name != "" {
		fPkg.YumPackage.Source = &agentendpointpb.Inventory_VersionedPackage_Source{
			Name:    pkg.Source.Name,
			Version: pkg.Source.Version,
		}
	}

	return fPkg
}

func formatZypperPackage(pkg *packages.PkgInfo) *agentendpointpb.Inventory_SoftwarePackage_ZypperPackage {
	return &agentendpointpb.Inventory_SoftwarePackage_ZypperPackage{
		ZypperPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version}}
}

func formatZypperPatch(pkg *packages.ZypperPatch) *agentendpointpb.Inventory_SoftwarePackage_ZypperPatch {
	return &agentendpointpb.Inventory_SoftwarePackage_ZypperPatch{
		ZypperPatch: &agentendpointpb.Inventory_ZypperPatch{
			PatchName: pkg.Name,
			Category:  pkg.Category,
			Severity:  pkg.Severity,
			Summary:   pkg.Summary,
		}}
}

func formatWUAPackage(pkg *packages.WUAPackage) *agentendpointpb.Inventory_SoftwarePackage_WuaPackage {
	var categories []*agentendpointpb.Inventory_WindowsUpdatePackage_WindowsUpdateCategory
	for idx, category := range pkg.Categories {
		categories = append(categories, &agentendpointpb.Inventory_WindowsUpdatePackage_WindowsUpdateCategory{
			Id:   pkg.CategoryIDs[idx],
			Name: category,
		})
	}

	return &agentendpointpb.Inventory_SoftwarePackage_WuaPackage{
		WuaPackage: &agentendpointpb.Inventory_WindowsUpdatePackage{
			Title:                    pkg.Title,
			Description:              pkg.Description,
			Categories:               categories,
			KbArticleIds:             pkg.KBArticleIDs,
			SupportUrl:               pkg.SupportURL,
			MoreInfoUrls:             pkg.MoreInfoURLs,
			UpdateId:                 pkg.UpdateID,
			RevisionNumber:           pkg.RevisionNumber,
			LastDeploymentChangeTime: timestamppb.New(pkg.LastDeploymentChangeTime),
		}}
}

func formatQFEPackage(ctx context.Context, pkg *packages.QFEPackage) *agentendpointpb.Inventory_SoftwarePackage_QfePackage {
	installedTime, err := time.Parse("1/2/2006", pkg.InstalledOn)
	if err != nil {
		clog.Warningf(ctx, "Error parsing QFE InstalledOn date: %v", err)
	}

	return &agentendpointpb.Inventory_SoftwarePackage_QfePackage{
		QfePackage: &agentendpointpb.Inventory_WindowsQuickFixEngineeringPackage{
			Caption:     pkg.Caption,
			Description: pkg.Description,
			HotFixId:    pkg.HotFixID,
			InstallTime: timestamppb.New(installedTime),
		}}
}

func formatWindowsApplication(pkg *packages.WindowsApplication) *agentendpointpb.Inventory_SoftwarePackage_WindowsApplication {

	d := datepb.Date{}
	// We have to check if date is zero.
	// Because zero value of time has Year, Month, Day equal to 1
	if !pkg.InstallDate.IsZero() {
		d = datepb.Date{
			Year:  int32(pkg.InstallDate.Year()),
			Month: int32(pkg.InstallDate.Month()),
			Day:   int32(pkg.InstallDate.Day()),
		}
	}
	return &agentendpointpb.Inventory_SoftwarePackage_WindowsApplication{
		WindowsApplication: &agentendpointpb.Inventory_WindowsApplication{
			DisplayName:    pkg.DisplayName,
			DisplayVersion: pkg.DisplayVersion,
			Publisher:      pkg.Publisher,
			InstallDate:    &d,
			HelpLink:       pkg.HelpLink,
		}}
}

func computeFingerprint(ctx context.Context, inventory *agentendpointpb.Inventory) (string, error) {
	fingerprint := sha256.New()
	b, err := proto.Marshal(inventory)
	if err != nil {
		return "", err
	}
	io.Copy(fingerprint, bytes.NewReader(b))

	return hex.EncodeToString(fingerprint.Sum(nil)), nil
}

func computeStableFingerprint(ctx context.Context, inventory *agentendpointpb.Inventory) (string, error) {
	fingerprint := sha256.New()
	b, err := proto.Marshal(inventory.GetOsInfo())
	if err != nil {
		return "", err
	}
	io.Copy(fingerprint, bytes.NewReader(b))

	installedPackages := inventory.GetInstalledPackages()
	availablePackages := inventory.GetAvailablePackages()

	entries := make([]string, 0, len(installedPackages)+len(availablePackages))

	for _, pkg := range installedPackages {
		entries = append(entries, fingerprintForPackage(pkg))
	}

	for _, pkg := range availablePackages {
		entries = append(entries, fingerprintForPackage(pkg))
	}

	sort.Strings(entries)

	for _, entry := range entries {
		if _, err := io.WriteString(fingerprint, entry); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(fingerprint.Sum(nil)), nil
}

func computeStableFingerprintVMInventory(ctx context.Context, inventory *agentendpointpb.VmInventory) (string, error) {
	fingerprint := sha256.New()
	b, err := proto.Marshal(inventory.GetOsInfo())
	if err != nil {
		return "", err
	}
	io.Copy(fingerprint, bytes.NewReader(b))

	installedPackages := inventory.GetInstalledPackages()
	availablePackages := inventory.GetAvailablePackages()

	entries := make([]string, 0, len(installedPackages)+len(availablePackages))

	for _, pkg := range installedPackages {
		entries = append(entries, fingerprintForInventoryItem(pkg))
	}

	for _, pkg := range availablePackages {
		entries = append(entries, fingerprintForInventoryItem(pkg))
	}

	sort.Strings(entries)

	for _, entry := range entries {
		if _, err := io.WriteString(fingerprint, entry); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(fingerprint.Sum(nil)), nil
}

func fingerprintForInventoryItem(pkg *agentendpointpb.VmInventory_InventoryItem) string {
	return pkg.String()
}

func fingerprintForPackage(pkg *agentendpointpb.Inventory_SoftwarePackage) string {
	//Inventory_WindowsUpdatePackage struct contains repeated fields
	//we should rely on fields that are stable and sufficient enough to uniquely identify the package.
	if wua := pkg.GetWuaPackage(); wua != nil {
		return fmt.Sprintf("%s-%s-%d", wua.GetTitle(), wua.GetUpdateId(), wua.GetRevisionNumber())
	}

	// For all packages other then wua we can just rely on proto String() method.
	return pkg.String()
}
