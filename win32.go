//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Win32 constants
const (
	wsPopup         = 0x80000000
	wsVisible       = 0x10000000
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	swHide          = 0
	swShow          = 5
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	waInactive      = 0
	htClient        = 1
	htCaption       = 2
	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmSetCursor     = 0x0020
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmMouseLeave    = 0x02A3
	wmNCLButtonDown = 0x00A1
	wmActivate      = 0x0006
	wmCommand       = 0x0111
	wmNull          = 0x0000

	spiGetWorkArea = 0x0030
	idcArrow       = 32512
	idcHand        = 32649
	nullPen        = 8

	psSolid          = 0
	transparentBk    = 1
	cleartypeQuality = 5
	fwNormal         = 400
	fwSemibold       = 600
	fwBold           = 700
	defaultCharset   = 1
	outDefaultPrecis = 0
	clipDefault      = 0
	defaultPitch     = 0

	dtLeft        = 0x00000000
	dtCenter      = 0x00000001
	dtRight       = 0x00000002
	dtVCenter     = 0x00000004
	dtSingleLine  = 0x00000020
	dtEndEllipsis = 0x00008000

	srcCopy     = 0x00CC0020
	tmeLeave    = 0x00000002
	biRGB       = 0
	dibRGBColor = 0

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002
	nifMsg    = 0x00000001
	nifIcon   = 0x00000002
	nifTip    = 0x00000004

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfChecked   = 0x00000008
	mfUnchecked = 0x00000000

	tpmLeftAlign   = 0x0000
	tpmBottomAlign = 0x0020
	tpmRightButton = 0x0002

	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2
	loadLibrarySearchSystem32   = 0x00000800
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procSendMessageW                  = user32.NewProc("SendMessageW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procIsWindowVisible               = user32.NewProc("IsWindowVisible")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procInvalidateRect                = user32.NewProc("InvalidateRect")
	procBeginPaint                    = user32.NewProc("BeginPaint")
	procEndPaint                      = user32.NewProc("EndPaint")
	procFillRect                      = user32.NewProc("FillRect")
	procDrawTextW                     = user32.NewProc("DrawTextW")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procSetCursor                     = user32.NewProc("SetCursor")
	procReleaseCapture                = user32.NewProc("ReleaseCapture")
	procTrackMouseEvent               = user32.NewProc("TrackMouseEvent")
	procSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")
	procCreatePopupMenu               = user32.NewProc("CreatePopupMenu")
	procAppendMenuW                   = user32.NewProc("AppendMenuW")
	procTrackPopupMenu                = user32.NewProc("TrackPopupMenu")
	procDestroyMenu                   = user32.NewProc("DestroyMenu")
	procCreateIconIndirect            = user32.NewProc("CreateIconIndirect")
	procDestroyIcon                   = user32.NewProc("DestroyIcon")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procFindWindowW                   = user32.NewProc("FindWindowW")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap           = gdi32.NewProc("CreateBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procCreateFontW            = gdi32.NewProc("CreateFontW")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procLoadLibraryExW   = kernel32.NewProc("LoadLibraryExW")
	procGetProcAddress   = kernel32.NewProc("GetProcAddress")
)

type point struct {
	X int32
	Y int32
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type paintStruct struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type trackMouseEventStruct struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   uintptr
	DwHoverTime uint32
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type notifyIconDataW struct {
	CbSize           uint32
	Hwnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         guid
	HBalloonIcon     uintptr
}

func fillSolidRect(hdc uintptr, r rect, color uint32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	procDeleteObject.Call(brush)
}

func drawBorderRect(hdc uintptr, r rect, borderColor uint32) {
	fillSolidRect(hdc, rect{r.Left, r.Top, r.Right, r.Top + 1}, borderColor)
	fillSolidRect(hdc, rect{r.Left, r.Bottom - 1, r.Right, r.Bottom}, borderColor)
	fillSolidRect(hdc, rect{r.Left, r.Top, r.Left + 1, r.Bottom}, borderColor)
	fillSolidRect(hdc, rect{r.Right - 1, r.Top, r.Right, r.Bottom}, borderColor)
}

func drawRoundRect(hdc uintptr, r rect, radius int32, fillColor, borderColor uint32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(fillColor))
	var pen uintptr
	if fillColor == borderColor {
		pen, _, _ = procGetStockObject.Call(nullPen)
	} else {
		pen, _, _ = procCreatePen.Call(psSolid, 1, uintptr(borderColor))
	}

	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)

	procRoundRect.Call(hdc,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom),
		uintptr(radius), uintptr(radius),
	)

	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(brush)
	if fillColor != borderColor {
		procDeleteObject.Call(pen)
	}
}

func drawText(hdc uintptr, text string, r rect, hFont uintptr, color uint32, flags uint32) {
	if text == "" {
		return
	}
	u16, _ := syscall.UTF16FromString(text)
	if len(u16) == 0 {
		return
	}
	oldFont, _, _ := procSelectObject.Call(hdc, hFont)
	procSetTextColor.Call(hdc, uintptr(color))
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u16[0])), ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(flags))
	procSelectObject.Call(hdc, oldFont)
}

func createUIFont(heightPx, weight int32, face string) uintptr {
	facePtr, _ := syscall.UTF16PtrFromString(face)
	hFont, _, _ := procCreateFontW.Call(
		uintptr(uint32(-heightPx)),
		0, 0, 0,
		uintptr(weight),
		0, 0, 0,
		defaultCharset,
		outDefaultPrecis,
		clipDefault,
		cleartypeQuality,
		defaultPitch,
		uintptr(unsafe.Pointer(facePtr)),
	)
	return hFont
}

func applyRoundedCorners(hwnd uintptr) {
	dllPtr, err := syscall.UTF16PtrFromString("dwmapi.dll")
	if err != nil {
		return
	}
	hMod, _, _ := procLoadLibraryExW.Call(uintptr(unsafe.Pointer(dllPtr)), 0, loadLibrarySearchSystem32)
	if hMod == 0 {
		return
	}
	procNamePtr, err := syscall.BytePtrFromString("DwmSetWindowAttribute")
	if err != nil {
		return
	}
	addr, _, _ := procGetProcAddress.Call(hMod, uintptr(unsafe.Pointer(procNamePtr)))
	if addr == 0 {
		return
	}
	pref := uint32(dwmwcpRound)
	syscall.SyscallN(addr, hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}

func enableDPIAwareness() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = (DPI_AWARENESS_CONTEXT)-4
	const perMonitorAwareV2 = ^uintptr(3)
	if err := procSetProcessDpiAwarenessContext.Find(); err == nil {
		ret, _, _ := procSetProcessDpiAwarenessContext.Call(perMonitorAwareV2)
		if ret != 0 {
			return
		}
	}
	procSetProcessDPIAware.Call()
}

func queryWindowDPI(hwnd uintptr) int32 {
	if err := procGetDpiForWindow.Find(); err == nil {
		ret, _, _ := procGetDpiForWindow.Call(hwnd)
		if ret >= 96 {
			return int32(ret)
		}
	}
	return 96
}

func scaleDPI(val, dpi int32) int32 {
	return (val*dpi + 48) / 96
}

func rgb(r, g, b uint8) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func loword(v uintptr) uint16 {
	return uint16(v & 0xFFFF)
}

func pointFromLParam(lp uintptr) point {
	x := int16(lp & 0xFFFF)
	y := int16((lp >> 16) & 0xFFFF)
	return point{X: int32(x), Y: int32(y)}
}

func ptInRect(p point, r rect) bool {
	return p.X >= r.Left && p.X < r.Right && p.Y >= r.Top && p.Y < r.Bottom
}

func copyUTF16(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	u16, _ := syscall.UTF16FromString(s)
	n := len(u16)
	if n > len(dst) {
		n = len(dst)
	}
	copy(dst[:n], u16[:n])
	dst[len(dst)-1] = 0
}
