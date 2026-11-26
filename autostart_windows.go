//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser            = uintptr(0x80000001)
	keyQueryValue              = 0x0001
	keySetValue                = 0x0002
	regSZ                      = 1
	regDword                   = 4
	errorSuccess               = 0
	errorFileNotFound          = 2
	appSettingsRegistryKeyPath = `Software\AGYQuotaMonitor`
	startMinimizedValueName    = "StartMinimized"
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")

	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// IsAutoStartEnabled reports whether the Windows Run registry entry is configured.
func IsAutoStartEnabled() bool {
	keyPtr, _ := syscall.UTF16PtrFromString(autoStartRegistryKeyPath)
	valPtr, _ := syscall.UTF16PtrFromString(autoStartAppName)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(keyPtr)),
		0,
		keyQueryValue,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != errorSuccess {
		return false
	}
	defer procRegCloseKey.Call(hKey)

	var dataType uint32
	var dataLen uint32
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valPtr)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		0,
		uintptr(unsafe.Pointer(&dataLen)),
	)
	return ret == errorSuccess && dataLen > 2
}

// SetAutoStart enables or disables starting the app automatically when signing into Windows.
func SetAutoStart(enabled bool) error {
	keyPtr, _ := syscall.UTF16PtrFromString(autoStartRegistryKeyPath)
	valPtr, _ := syscall.UTF16PtrFromString(autoStartAppName)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(keyPtr)),
		0,
		keySetValue,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != errorSuccess {
		return fmt.Errorf("erro ao abrir registro do Windows (codigo %d)", ret)
	}
	defer procRegCloseKey.Call(hKey)

	if !enabled {
		ret, _, _ = procRegDeleteValueW.Call(hKey, uintptr(unsafe.Pointer(valPtr)))
		if ret != errorSuccess && ret != errorFileNotFound {
			return fmt.Errorf("erro ao remover entrada do registro (codigo %d)", ret)
		}
		return nil
	}

	exePath, err := resolveExecutablePath()
	if err != nil {
		return err
	}

	u16Val, err := syscall.UTF16FromString(`"` + exePath + `"`)
	if err != nil {
		return fmt.Errorf("erro ao codificar caminho: %w", err)
	}

	byteLen := uint32(len(u16Val) * 2)
	ret, _, _ = procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valPtr)),
		0,
		regSZ,
		uintptr(unsafe.Pointer(&u16Val[0])),
		uintptr(byteLen),
	)
	if ret != errorSuccess {
		return fmt.Errorf("erro ao gravar no registro (codigo %d)", ret)
	}
	return nil
}

// ToggleAutoStart flips the auto-start state and returns the updated setting.
func ToggleAutoStart() (bool, error) {
	target := !IsAutoStartEnabled()
	if err := SetAutoStart(target); err != nil {
		return IsAutoStartEnabled(), err
	}
	return target, nil
}

// IsStartMinimizedEnabled reports whether starting hidden in the tray is enabled.
func IsStartMinimizedEnabled() bool {
	keyPtr, _ := syscall.UTF16PtrFromString(appSettingsRegistryKeyPath)
	valPtr, _ := syscall.UTF16PtrFromString(startMinimizedValueName)

	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(keyPtr)),
		0,
		keyQueryValue,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != errorSuccess {
		return false
	}
	defer procRegCloseKey.Call(hKey)

	var dataType uint32
	var val uint32
	dataLen := uint32(unsafe.Sizeof(val))
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valPtr)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		uintptr(unsafe.Pointer(&val)),
		uintptr(unsafe.Pointer(&dataLen)),
	)
	return ret == errorSuccess && dataType == regDword && val != 0
}

// SetStartMinimized persists the "start minimized" preference in the Windows registry.
func SetStartMinimized(enabled bool) error {
	keyPtr, _ := syscall.UTF16PtrFromString(appSettingsRegistryKeyPath)
	valPtr, _ := syscall.UTF16PtrFromString(startMinimizedValueName)

	var hKey uintptr
	ret, _, _ := procRegCreateKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(keyPtr)),
		0,
		0,
		0,
		keySetValue,
		0,
		uintptr(unsafe.Pointer(&hKey)),
		0,
	)
	if ret != errorSuccess {
		return fmt.Errorf("erro ao abrir chave de configuracao (codigo %d)", ret)
	}
	defer procRegCloseKey.Call(hKey)

	var val uint32
	if enabled {
		val = 1
	}
	ret, _, _ = procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valPtr)),
		0,
		regDword,
		uintptr(unsafe.Pointer(&val)),
		unsafe.Sizeof(val),
	)
	if ret != errorSuccess {
		return fmt.Errorf("erro ao gravar preferencia no registro (codigo %d)", ret)
	}
	return nil
}

// ToggleStartMinimized flips the start-minimized preference and returns the updated setting.
func ToggleStartMinimized() (bool, error) {
	target := !IsStartMinimizedEnabled()
	if err := SetStartMinimized(target); err != nil {
		return IsStartMinimizedEnabled(), err
	}
	return target, nil
}

// OpenBrowserURL opens an HTTP/HTTPS URL using the default Windows browser.
func OpenBrowserURL(targetURL string) error {
	if !isAllowedBrowserURL(targetURL) {
		return fmt.Errorf("URL invalida para abertura no navegador")
	}
	verbPtr, _ := syscall.UTF16PtrFromString("open")
	urlPtr, _ := syscall.UTF16PtrFromString(targetURL)
	ret, _, err := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(urlPtr)),
		0,
		0,
		swShow,
	)
	if ret <= 32 {
		return fmt.Errorf("nao foi possivel abrir o navegador: %v", err)
	}
	return nil
}
