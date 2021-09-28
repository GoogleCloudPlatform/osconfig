//  Copyright 2020 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package packages

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	msiInstallArgs = []string{"ACTION=INSTALL", "REBOOT=ReallySuppress"}

	msi                        = windows.NewLazySystemDLL("msi.dll")
	procMsiOpenPackageExW      = msi.NewProc("MsiOpenPackageExW")
	procMsiGetProductPropertyW = msi.NewProc("MsiGetProductPropertyW")
	procMsiQueryProductStateW  = msi.NewProc("MsiQueryProductStateW")
	procMsiCloseHandle         = msi.NewProc("MsiCloseHandle")
	procMsiInstallProductW     = msi.NewProc("MsiInstallProductW")
	procMsiSetInternalUI       = msi.NewProc("MsiSetInternalUI")

	once sync.Once
)

func init() {
	MSIExists = true
}

func setUIMode() {
	/*
		INSTALLUILEVEL MsiSetInternalUI(
		  INSTALLUILEVEL dwUILevel,
		  HWND           *phWnd
		);
	*/
	const INSTALLUILEVEL_NONE = 2
	once.Do(func() {
		procMsiSetInternalUI.Call(
			uintptr(INSTALLUILEVEL_NONE),
			0,
		)
	})
}

// https://docs.microsoft.com/en-us/windows/win32/api/msi/nf-msi-msiopenpackageexw
func msiOpenPackageExW(szPackagePath string, dwOptions uint32) (uintptr, error) {
	/*
		UINT MsiOpenPackageExW(
		  LPCWSTR   szPackagePath,
		  DWORD     dwOptions,
		  MSIHANDLE *hProduct
		);
	*/
	var handle int32
	pHandle := uintptr(unsafe.Pointer(&handle))

	szPackagePathPtr, err := syscall.UTF16PtrFromString(szPackagePath)
	if err != nil {
		return 0, fmt.Errorf("error encoding szPackagePath to UTF16: %v", err)
	}

	ret, _, _ := procMsiOpenPackageExW.Call(
		uintptr(unsafe.Pointer(szPackagePathPtr)),
		uintptr(dwOptions),
		pHandle,
	)
	if ret != 0 {
		return 0, fmt.Errorf("MsiOpenPackageExW error: %s", syscall.Errno(ret))
	}

	return uintptr(handle), nil
}

// https://docs.microsoft.com/en-us/windows/win32/api/msi/nf-msi-msiclosehandle
func msiCloseHandle(handle uintptr) {
	/*
		UINT MsiCloseHandle(
		  MSIHANDLE hAny
		);
	*/

	procMsiCloseHandle.Call(handle)
}

// https://docs.microsoft.com/en-us/windows/win32/api/msi/nf-msi-msigetproductpropertyw
func msiGetProductPropertyW(handle uintptr, szProperty string) (string, error) {
	/*
		UINT MsiGetProductPropertyW(
		  MSIHANDLE hProduct,
		  LPCSTR    szProperty,
		  LPSTR     lpValueBuf,
		  LPDWORD   pcchValueBuf
		);
	*/

	szPropertyPtr, err := syscall.UTF16PtrFromString(szProperty)
	if err != nil {
		return "", fmt.Errorf("error encoding szProperty to UTF16: %v", err)
	}

	size := uint32(128)
	lpValueBuf := make([]uint16, size)

	ret, _, _ := procMsiGetProductPropertyW.Call(
		handle,
		uintptr(unsafe.Pointer(szPropertyPtr)),
		uintptr(unsafe.Pointer(&lpValueBuf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret != 0 {
		return "", fmt.Errorf("MsiGetProductPropertyW error: %s", syscall.Errno(ret))
	}
	return syscall.UTF16ToString(lpValueBuf), nil
}

type msiInstallState int32

const (
	INSTALLSTATE_UNKNOWN    = msiInstallState(-1)
	INSTALLSTATE_ADVERTISED = msiInstallState(1)
	INSTALLSTATE_ABSENT     = msiInstallState(2)
	INSTALLSTATE_DEFAULT    = msiInstallState(5)
)

//  https://docs.microsoft.com/en-us/windows/win32/api/msi/nf-msi-msiqueryproductstatew
func msiMsiQueryProductStateW(szProduct string) (msiInstallState, error) {
	/*
		INSTALLSTATE MsiQueryProductStateW(
		  LPCWSTR szProduct
		);
	*/

	szProductPtr, err := syscall.UTF16PtrFromString(szProduct)
	if err != nil {
		return -1, fmt.Errorf("error encoding szProduct to UTF16: %v", err)
	}

	ret, _, _ := procMsiQueryProductStateW.Call(uintptr(unsafe.Pointer(szProductPtr)))
	return msiInstallState(ret), nil
}

// https://docs.microsoft.com/en-us/windows/win32/api/msi/nf-msi-msiinstallproductw
func msiInstallProductW(szPackagePath string, szCommandLine []string) error {
	/*
		UINT MsiInstallProductW(
		  LPCWSTR szPackagePath,
		  LPCWSTR szCommandLine
		);
	*/

	szPackagePathPtr, err := syscall.UTF16PtrFromString(szPackagePath)
	if err != nil {
		return fmt.Errorf("error encoding szPackagePath to UTF16: %v", err)
	}

	szCommandLinePtr, err := syscall.UTF16PtrFromString(strings.Join(szCommandLine, " "))
	if err != nil {
		return fmt.Errorf("error encoding szCommandLine to UTF16: %v", err)
	}

	ret, _, _ := procMsiInstallProductW.Call(
		uintptr(unsafe.Pointer(szPackagePathPtr)),
		uintptr(unsafe.Pointer(szCommandLinePtr)),
	)
	if ret != 0 {
		return fmt.Errorf("MsiInstallProductW error: %s", syscall.Errno(ret))
	}
	return nil
}

// MSIInfo returns the ProductName and ProductCode for an MSI.
func MSIInfo(path string) (string, string, error) {
	setUIMode()

	if err := coInitializeEx(); err != nil {
		return "", "", err
	}
	defer ole.CoUninitialize()

	const MSIOPENPACKAGEFLAGS_IGNOREMACHINESTATE = 1
	handle, err := msiOpenPackageExW(path, MSIOPENPACKAGEFLAGS_IGNOREMACHINESTATE)
	if err != nil {
		return "", "", fmt.Errorf("error opening MSI package %q: %v", path, err)
	}
	defer msiCloseHandle(handle)

	productCode, err := msiGetProductPropertyW(handle, "ProductCode")
	if err != nil {
		return "", "", fmt.Errorf("error getting ProductCode property: %v", err)
	}

	productName, err := msiGetProductPropertyW(handle, "ProductName")
	if err != nil {
		return "", "", fmt.Errorf("error getting ProductName property: %v", err)
	}

	return productName, productCode, nil
}

// MSIInstalled returns if the msi ProductCode is installed.
func MSIInstalled(productCode string) (bool, error) {
	setUIMode()

	if err := coInitializeEx(); err != nil {
		return false, err
	}
	defer ole.CoUninitialize()

	state, err := msiMsiQueryProductStateW(productCode)
	if err != nil {
		return false, err
	}

	return state == INSTALLSTATE_DEFAULT, nil
}

// InstallMSIPackage installs an msi package.
func InstallMSIPackage(ctx context.Context, path string, args []string) error {
	setUIMode()

	args = append(msiInstallArgs, args...)
	clog.Infof(ctx, "Installing msi package %q with command line %q.", path, args)
	if err := msiInstallProductW(path, args); err != nil {
		return fmt.Errorf("error installing MSI package %q: %v", path, err)
	}

	return nil
}
