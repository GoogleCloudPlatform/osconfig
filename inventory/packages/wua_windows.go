/*
Copyright 2019 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package packages

import (
	"fmt"
	"sync"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var wuaSession sync.Mutex

// IUpdateSession is a an IUpdateSession.
type IUpdateSession struct {
	obj *ole.IUnknown
	ses *ole.IDispatch
}

func NewUpdateSession() (*IUpdateSession, error) {
	wuaSession.Lock()
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return nil, err
	}

	updateSessionObj, err := oleutil.CreateObject("Microsoft.Update.Session")
	if err != nil {
		return nil, fmt.Errorf(`oleutil.CreateObject("Microsoft.Update.Session"): %v`, err)
	}

	session, err := updateSessionObj.IDispatch(ole.IID_IDispatch)
	if err != nil {
		return nil, err
	}
	return &IUpdateSession{obj: updateSessionObj, ses: session}, nil
}

func (s *IUpdateSession) Close() {
	wuaSession.Unlock()
	ole.CoUninitialize()
	s.ses.Release()
	s.obj.Release()
}

// InstallWUAUpdate install a WIndows update.
func (s *IUpdateSession) InstallWUAUpdate(updt *IUpdate) error {
	title, err := updt.GetProperty("Title")
	if err != nil {
		return fmt.Errorf(`updt.GetProperty("Title"): %v`, err)
	}

	updateCollObj, err := oleutil.CreateObject("Microsoft.Update.UpdateColl")
	if err != nil {
		return fmt.Errorf(`oleutil.CreateObject("updateColl"): %v`, err)
	}
	defer updateCollObj.Release()

	updateColl, err := updateCollObj.IDispatch(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer updateColl.Release()
	updts := &IUpdateCollection{updateColl}

	eula, err := updt.GetProperty("EulaAccepted")
	if err != nil {
		return fmt.Errorf(`updt.GetProperty("EulaAccepted"): %v`, err)
	}
	// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-oaut/7b39eb24-9d39-498a-bcd8-75c38e5823d0
	if eula.Val == 0 {
		DebugLogger.Printf("%s - Accepting EULA", title.Value())
		if _, err := updt.CallMethod("AcceptEula"); err != nil {
			return fmt.Errorf(`updateColl.CallMethod("AcceptEula"): %v`, err)
		}
	} else {
		DebugLogger.Printf("%s - EulaAccepted: %v", title.Value(), eula.Value())
	}

	if _, err := updateColl.CallMethod("Add", updt); err != nil {
		return fmt.Errorf(`updateColl.CallMethod("Add", updt): %v`, err)
	}

	DebugLogger.Printf("Downloading update %s", title.Value())
	if err := s.DownloadWUAUpdateCollection(updts); err != nil {
		return fmt.Errorf("DownloadWUAUpdateCollection error: %v", err)
	}

	DebugLogger.Printf("Installing update %s", title.Value())
	if err := s.InstallWUAUpdateCollection(updts); err != nil {
		return fmt.Errorf("InstallWUAUpdateCollection error: %v", err)
	}

	return nil
}

type IUpdateCollection struct {
	*ole.IDispatch
}

type IUpdate struct {
	*ole.IDispatch
}

func (c *IUpdateCollection) Add(updt *IUpdate) error {
	if _, err := c.CallMethod("Add", updt); err != nil {
		return fmt.Errorf(`updateColl.CallMethod("Add", updt): %v`, err)
	}
	return nil
}

func (c *IUpdateCollection) RemoveAt(i int) error {
	if c == nil {
		return nil
	}
	_, err := c.CallMethod("RemoveAt", i)
	return err
}

func (c *IUpdateCollection) Count() (int32, error) {
	if c == nil {
		return 0, nil
	}

	countRaw, err := c.GetProperty("Count")
	if err != nil {
		return 0, err
	}
	count, _ := countRaw.Value().(int32)
	return count, nil
}

func (c *IUpdateCollection) Item(i int) (*IUpdate, error) {
	updtRaw, err := c.GetProperty("Item", i)
	if err != nil {
		return nil, err
	}
	return &IUpdate{updtRaw.ToIDispatch()}, nil
}

func getStringSlice(dis *ole.IDispatch) ([]string, error) {
	countRaw, err := dis.GetProperty("Count")
	if err != nil {
		return nil, err
	}
	count, _ := countRaw.Value().(int32)

	if count == 0 {
		return nil, nil
	}

	var ss []string
	for i := 0; i < int(count); i++ {
		item, err := dis.GetProperty("Item", i)
		if err != nil {
			return nil, err
		}

		ss = append(ss, item.ToString())
	}
	return ss, nil
}

func getCategories(cat *ole.IDispatch) ([]string, []string, error) {
	countRaw, err := cat.GetProperty("Count")
	if err != nil {
		return nil, nil, err
	}
	count, _ := countRaw.Value().(int32)

	if count == 0 {
		return nil, nil, nil
	}

	var cns, cids []string
	for i := 0; i < int(count); i++ {
		itemRaw, err := cat.GetProperty("Item", i)
		if err != nil {
			return nil, nil, err
		}
		item := itemRaw.ToIDispatch()
		defer item.Release()

		name, err := item.GetProperty("Name")
		if err != nil {
			return nil, nil, err
		}

		categoryID, err := item.GetProperty("CategoryID")
		if err != nil {
			return nil, nil, err
		}

		cns = append(cns, name.ToString())
		cids = append(cids, categoryID.ToString())
	}
	return cns, cids, nil
}

// WUAUpdates queries the Windows Update Agent API searcher with the provided query.
func WUAUpdates(query string) ([]WUAPackage, error) {
	session, err := NewUpdateSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	updts, err := session.GetWUAUpdateCollection(query)
	if err != nil {
		return nil, err
	}

	count, err := updts.GetProperty("Count")
	if err != nil {
		return nil, err
	}
	updtCnt, _ := count.Value().(int32)

	if updtCnt == 0 {
		return nil, nil
	}

	var packages []WUAPackage
	for i := 0; i < int(updtCnt); i++ {
		updtRaw, err := updts.GetProperty("Item", i)
		if err != nil {
			return nil, err
		}

		updt := updtRaw.ToIDispatch()
		defer updt.Release()

		title, err := updt.GetProperty("Title")
		if err != nil {
			return nil, err
		}

		description, err := updt.GetProperty("Description")
		if err != nil {
			return nil, err
		}

		kbArticleIDsRaw, err := updt.GetProperty("KBArticleIDs")
		if err != nil {
			return nil, err
		}
		kbArticleIDs, err := getStringSlice(kbArticleIDsRaw.ToIDispatch())
		if err != nil {
			return nil, err
		}

		categoriesRaw, err := updt.GetProperty("Categories")
		if err != nil {
			return nil, err
		}
		categories, categoryIDs, err := getCategories(categoriesRaw.ToIDispatch())
		if err != nil {
			return nil, err
		}

		supportURL, err := updt.GetProperty("SupportURL")
		if err != nil {
			return nil, err
		}

		lastDeploymentChangeTimeRaw, err := updt.GetProperty("LastDeploymentChangeTime")
		if err != nil {
			return nil, err
		}
		lastDeploymentChangeTime, err := ole.GetVariantDate(uint64(lastDeploymentChangeTimeRaw.Val))
		if err != nil {
			return nil, err
		}

		identityRaw, err := updt.GetProperty("Identity")
		if err != nil {
			return nil, err
		}
		identity := identityRaw.ToIDispatch()
		defer updt.Release()

		revisionNumber, err := identity.GetProperty("RevisionNumber")
		if err != nil {
			return nil, err
		}

		updateID, err := identity.GetProperty("UpdateID")
		if err != nil {
			return nil, err
		}

		pkg := WUAPackage{
			Title:                    title.ToString(),
			Description:              description.ToString(),
			SupportURL:               supportURL.ToString(),
			KBArticleIDs:             kbArticleIDs,
			UpdateID:                 updateID.ToString(),
			Categories:               categories,
			CategoryIDs:              categoryIDs,
			RevisionNumber:           int32(revisionNumber.Val),
			LastDeploymentChangeTime: lastDeploymentChangeTime,
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// DownloadWUAUpdateCollection downloads all updates in a IUpdateCollection
func (s *IUpdateSession) DownloadWUAUpdateCollection(updates *IUpdateCollection) error {
	// returns IUpdateDownloader
	// https://docs.microsoft.com/en-us/windows/desktop/api/wuapi/nn-wuapi-iupdatedownloader
	downloaderRaw, err := s.ses.CallMethod("CreateUpdateDownloader")
	if err != nil {
		return fmt.Errorf("error calling method CreateUpdateDownloader on IUpdateSession: %v", err)
	}
	downloader := downloaderRaw.ToIDispatch()
	defer downloaderRaw.Clear()

	if _, err := downloader.PutProperty("Updates", updates); err != nil {
		return fmt.Errorf("error calling PutProperty Updates on IUpdateDownloader: %v", err)
	}

	if _, err := downloader.CallMethod("Download"); err != nil {
		return fmt.Errorf("error calling method Download on IUpdateDownloader: %v", err)
	}
	return nil
}

// InstallWUAUpdateCollection installs all updates in a IUpdateCollection
func (s *IUpdateSession) InstallWUAUpdateCollection(updates *IUpdateCollection) error {
	// returns IUpdateInstallersession *ole.IDispatch,
	// https://docs.microsoft.com/en-us/windows/desktop/api/wuapi/nf-wuapi-iupdatesession-createupdateinstaller
	installerRaw, err := s.ses.CallMethod("CreateUpdateInstaller")
	if err != nil {
		return fmt.Errorf("error calling method CreateUpdateInstaller on IUpdateSession: %v", err)
	}
	installer := installerRaw.ToIDispatch()
	defer installerRaw.Clear()

	if _, err := installer.PutProperty("Updates", updates); err != nil {
		return fmt.Errorf("error calling PutProperty Updates on IUpdateInstaller: %v", err)
	}

	// TODO: Look into using the async methods and attempt to track/log progress.
	if _, err := installer.CallMethod("Install"); err != nil {
		return fmt.Errorf("error calling method Install on IUpdateInstaller: %v", err)
	}
	return nil
}

// GetWUAUpdateCollection queries the Windows Update Agent API searcher with the provided query
// and returns a IUpdateCollection.
func (s *IUpdateSession) GetWUAUpdateCollection(query string) (*IUpdateCollection, error) {
	// returns IUpdateSearcher
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa386515(v=vs.85).aspx
	searcherRaw, err := s.ses.CallMethod("CreateUpdateSearcher")
	if err != nil {
		return nil, err
	}
	searcher := searcherRaw.ToIDispatch()
	defer searcherRaw.Clear()

	// returns ISearchResult
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa386077(v=vs.85).aspx
	resultRaw, err := searcher.CallMethod("Search", query)
	if err != nil {
		return nil, fmt.Errorf("error calling method Search on IUpdateSearcher: %v", err)
	}
	result := resultRaw.ToIDispatch()
	defer resultRaw.Clear()

	// returns IUpdateCollection
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa386107(v=vs.85).aspx
	updtsRaw, err := result.GetProperty("Updates")
	if err != nil {
		return nil, fmt.Errorf("error calling GetProperty Updates on ISearchResult: %v", err)
	}

	return &IUpdateCollection{updtsRaw.ToIDispatch()}, nil
}
