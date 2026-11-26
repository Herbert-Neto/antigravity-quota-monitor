//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

func showTrayMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	state.mu.Lock()
	pinned := state.pinned
	updateInfo := state.updateInfo
	checkingUpdate := state.checkingUpdate
	state.mu.Unlock()

	pinFlags := uint32(mfString)
	if pinned {
		pinFlags |= mfChecked
	} else {
		pinFlags |= mfUnchecked
	}

	autoFlags := uint32(mfString)
	if IsAutoStartEnabled() {
		autoFlags |= mfChecked
	} else {
		autoFlags |= mfUnchecked
	}

	startMinFlags := uint32(mfString)
	if IsStartMinimizedEnabled() {
		startMinFlags |= mfChecked
	} else {
		startMinFlags |= mfUnchecked
	}

	updateLabel := fmt.Sprintf("Verificar atualizações (%s)", CurrentVersion())
	if updateInfo != nil && updateInfo.HasUpdate {
		updateLabel = fmt.Sprintf("Nova versão disponível (%s) — Baixar", updateInfo.LatestVersion)
	} else if checkingUpdate {
		updateLabel = "Verificando atualizações..."
	}

	appendMenuItem(hMenu, mfString, idmToggle, "Mostrar / Ocultar")
	appendMenuItem(hMenu, pinFlags, idmPin, "Modo Fixado")
	appendMenuItem(hMenu, autoFlags, idmAutoStart, "Iniciar com o Windows")
	appendMenuItem(hMenu, startMinFlags, idmStartMinimized, "Iniciar minimizado")
	appendMenuItem(hMenu, mfString, idmRefresh, "Atualizar Agora")
	appendMenuItem(hMenu, mfString, idmCheckUpdate, updateLabel)
	appendMenuItem(hMenu, mfSeparator, 0, "")
	appendMenuItem(hMenu, mfString, idmExit, "Sair")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(hwnd)
	procTrackPopupMenu.Call(
		hMenu,
		uintptr(tpmLeftAlign|tpmBottomAlign|tpmRightButton),
		uintptr(pt.X), uintptr(pt.Y),
		0, hwnd, 0,
	)
	procPostMessageW.Call(hwnd, wmNull, 0, 0)
}

func appendMenuItem(hMenu uintptr, flags uint32, id uintptr, label string) {
	var ptr uintptr
	if label != "" {
		p, _ := syscall.UTF16PtrFromString(label)
		ptr = uintptr(unsafe.Pointer(p))
	}
	procAppendMenuW.Call(hMenu, uintptr(flags), id, ptr)
}

func addTrayIcon(hwnd uintptr) {
	hIcon := createStatusIcon(rgb(0x3B, 0x82, 0xF6))
	state.mu.Lock()
	state.hTrayIcon = hIcon
	state.mu.Unlock()

	nid := makeNotifyIconData(hwnd, hIcon, "AGY Quota Monitor — Carregando...")
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
}

func updateTrayIcon(hwnd uintptr) {
	state.mu.Lock()
	data := state.data
	loading := state.loading
	lastErr := state.lastErr
	oldIcon := state.hTrayIcon
	state.mu.Unlock()

	col := statusColorForData(data, loading)
	newIcon := createStatusIcon(col)
	tip := buildTrayTooltip(data, loading, lastErr)

	nid := makeNotifyIconData(hwnd, newIcon, tip)
	ret, _, _ := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		procDestroyIcon.Call(newIcon)
		return
	}

	state.mu.Lock()
	state.hTrayIcon = newIcon
	state.mu.Unlock()
	if oldIcon != 0 {
		procDestroyIcon.Call(oldIcon)
	}
}

func removeTrayIcon(hwnd uintptr) {
	state.mu.Lock()
	hIcon := state.hTrayIcon
	state.hTrayIcon = 0
	state.mu.Unlock()

	nid := notifyIconDataW{
		CbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		Hwnd:   hwnd,
		UID:    trayIconID,
	}
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	if hIcon != 0 {
		procDestroyIcon.Call(hIcon)
	}
}

func makeNotifyIconData(hwnd, hIcon uintptr, tip string) notifyIconDataW {
	nid := notifyIconDataW{
		CbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		Hwnd:             hwnd,
		UID:              trayIconID,
		UFlags:           nifMsg | nifIcon | nifTip,
		UCallbackMessage: wmTrayCallback,
		HIcon:            hIcon,
	}
	copyUTF16(nid.SzTip[:], tip)
	return nid
}

func buildTrayTooltip(data *UsageData, loading bool, lastErr string) string {
	if data == nil || len(data.Groups) == 0 {
		if loading {
			return "AGY Quota: Atualizando..."
		}
		if lastErr != "" {
			return "AGY Quota: Erro ao consultar"
		}
		return "AGY Quota Monitor"
	}

	var parts []string
	for _, g := range data.Groups {
		label := g.Name
		switch {
		case strings.Contains(g.Name, "Gemini Models"):
			label = "Gem"
		case strings.Contains(g.Name, "Claude"):
			label = "3P"
		case strings.Contains(g.Name, "Argon"):
			label = "Argon"
		}
		lowest := 1.0
		for _, b := range g.Buckets {
			if b.RemainingFraction < lowest {
				lowest = b.RemainingFraction
			}
		}
		parts = append(parts, fmt.Sprintf("%s: %.0f%%", label, clamp01(lowest)*100.0))
	}
	return "AGY Quota | " + strings.Join(parts, " | ")
}

// createStatusIcon draws an anti-aliased 32x32 ARGB badge icon in memory.
func createStatusIcon(colorRef uint32) uintptr {
	const size = 32
	r := uint8(colorRef & 0xFF)
	g := uint8((colorRef >> 8) & 0xFF)
	b := uint8((colorRef >> 16) & 0xFF)

	bmi := bitmapInfoHeader{
		BiSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		BiWidth:       size,
		BiHeight:      -size,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: biRGB,
	}

	var bitsPtr unsafe.Pointer
	hdcScreen, _, _ := procCreateCompatibleDC.Call(0)
	defer procDeleteDC.Call(hdcScreen)

	hColor, _, _ := procCreateDIBSection.Call(
		hdcScreen,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColor,
		uintptr(unsafe.Pointer(&bitsPtr)),
		0, 0,
	)
	if hColor == 0 || bitsPtr == nil {
		return 0
	}
	defer procDeleteObject.Call(hColor)

	pixels := unsafe.Slice((*uint32)(bitsPtr), size*size)
	cx, cy := 15.5, 15.5
	outerR := 13.5
	innerR := 9.8

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Hypot(dx, dy)

			var pr, pg, pb, pa uint8
			switch {
			case dist <= innerR:
				pr, pg, pb, pa = r, g, b, 255
			case dist <= outerR:
				// Dark crisp border ring
				alpha := clamp01(outerR - dist + 0.5)
				pa = uint8(alpha * 255)
				pr = uint8(float64(0x18) * alpha)
				pg = uint8(float64(0x1C) * alpha)
				pb = uint8(float64(0x26) * alpha)
			}
			// BGRA premultiplied little-endian uint32
			pixels[y*size+x] = uint32(pb) | uint32(pg)<<8 | uint32(pr)<<16 | uint32(pa)<<24
		}
	}

	hMask, _, _ := procCreateBitmap.Call(size, size, 1, 1, 0)
	if hMask == 0 {
		return 0
	}
	defer procDeleteObject.Call(hMask)

	ii := iconInfo{
		FIcon:    1,
		HbmMask:  hMask,
		HbmColor: hColor,
	}
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	return hIcon
}
