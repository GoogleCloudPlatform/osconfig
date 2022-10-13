package agentendpoint

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/attributes"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/inventory"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
	datepb "google.golang.org/genproto/googleapis/type/date"
)

const (
	inventoryURL = agentconfig.ReportURL + "/guestInventory"
)

// ReportInventory writes inventory to guest attributes and reports it to agent endpoint.
func (c *Client) ReportInventory(ctx context.Context) {
	state := inventory.Get(ctx)
	if !agentconfig.DisableInventoryWrite() {
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

	reportFull := false
	var res *agentendpointpb.ReportInventoryResponse
	var err error
	f := func() error {
		res, err = c.reportInventory(ctx, inventory, reportFull)
		if err != nil {
			return err
		}
		return nil
	}

	if err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "ReportInventory", f); err != nil {
		clog.Errorf(ctx, "Error reporting inventory checksum: %v", err)
		return
	}

	if res.GetReportFullInventory() {
		reportFull = true
		if err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "ReportInventory", f); err != nil {
			clog.Errorf(ctx, "Error reporting full inventory: %v", err)
			return
		}
	}
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
		if packages.YumExists {
			for i, pkg := range pkgs.Rpm {
				temp[i] = &agentendpointpb.Inventory_SoftwarePackage{
					Details: formatYumPackage(pkg),
				}
			}
		} else if packages.ZypperExists {
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
	return &agentendpointpb.Inventory_SoftwarePackage_AptPackage{
		AptPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version,
		}}
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
	return &agentendpointpb.Inventory_SoftwarePackage_YumPackage{
		YumPackage: &agentendpointpb.Inventory_VersionedPackage{
			PackageName:  pkg.Name,
			Architecture: pkg.Arch,
			Version:      pkg.Version}}
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
