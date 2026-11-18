package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	appTitle       = "AGY Quota Monitor"
	windowClass    = "AGYQuotaWidgetClass"
	trayIconID     = 1
	wmAppRefresh   = 0x8001 // WM_APP + 1
	wmTrayCallback = 0x8002 // WM_APP + 2

	idmToggle  = 1001
	idmPin     = 1002
	idmRefresh = 1003
	idmExit    = 1004

	btnNone = iota
	btnPin
	btnRefresh
	btnHide
	btnClose
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
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

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

	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procDwmSetWindowAttrib  = dwmapi.NewProc("DwmSetWindowAttribute")
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

type appState struct {
	mu          sync.Mutex
	hwnd        uintptr
	hTrayIcon   uintptr
	dpi         int32
	data        *UsageData
	loading     bool
	lastErr     string
	lastUpdated time.Time
	pinned      bool
	userMoved   bool
	hoverBtn    int
	tracking    bool

	btnPinRect     rect
	btnRefreshRect rect
	btnHideRect    rect
	btnCloseRect   rect
	headerRect     rect
}

var state = &appState{
	dpi:    96,
	pinned: true,
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--test-cli" {
			os.Exit(runCLI())
		}
	}
	runGUI()
}

func runCLI() int {
	data, err := FetchQuota(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao consultar cotas: %v\n", err)
		return 1
	}

	now := time.Now()
	fmt.Printf("=== %s ===\n", appTitle)
	for _, g := range data.Groups {
		fmt.Printf("\n[%s] (%s)\n", g.Name, ShortGroupSubtitle(g))
		for _, b := range g.Buckets {
			pct := b.RemainingFraction * 100.0
			fmt.Printf("  - %-22s %5.1f%%  |  %s\n",
				FriendlyBucketName(b)+":", pct, FormatCountdown(b.ResetTime, now))
		}
	}
	return 0
}

func runGUI() {
	runtime.LockOSThread()
	enableDPIAwareness()

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	classPtr, _ := syscall.UTF16PtrFromString(windowClass)
	titlePtr, _ := syscall.UTF16PtrFromString(appTitle)
	cursorArrow, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		HCursor:       cursorArrow,
		LpszClassName: classPtr,
	}
	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		panic(fmt.Sprintf("RegisterClassExW falhou: %v", err))
	}

	initialW := int32(416)
	initialH := int32(250)
	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(wsExTopmost|wsExToolWindow),
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(wsPopup|wsVisible),
		0, 0, uintptr(initialW), uintptr(initialH),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		panic(fmt.Sprintf("CreateWindowExW falhou: %v", err))
	}

	state.mu.Lock()
	state.hwnd = hwnd
	state.dpi = queryWindowDPI(hwnd)
	state.mu.Unlock()

	applyRoundedCorners(hwnd)
	resizeAndAnchorBottomRight(hwnd, calcDesiredHeight())
	addTrayIcon(hwnd)

	// Initial background fetch + periodic refreshers
	triggerFetch()
	startBackgroundLoops(hwnd)

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func startBackgroundLoops(hwnd uintptr) {
	go func() {
		fetchTicker := time.NewTicker(3 * time.Minute)
		countdownTicker := time.NewTicker(15 * time.Second)
		defer fetchTicker.Stop()
		defer countdownTicker.Stop()

		for {
			select {
			case <-fetchTicker.C:
				triggerFetch()
			case <-countdownTicker.C:
				procPostMessageW.Call(hwnd, wmAppRefresh, 0, 0)
			}
		}
	}()
}

func triggerFetch() {
	state.mu.Lock()
	if state.loading {
		state.mu.Unlock()
		return
	}
	state.loading = true
	hwnd := state.hwnd
	state.mu.Unlock()

	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmAppRefresh, 0, 0)
	}

	go func() {
		data, err := FetchQuota(context.Background())

		state.mu.Lock()
		state.loading = false
		if err != nil {
			state.lastErr = err.Error()
		} else {
			state.data = data
			state.lastErr = ""
			state.lastUpdated = time.Now()
		}
		h := state.hwnd
		state.mu.Unlock()

		if h != 0 {
			procPostMessageW.Call(h, wmAppRefresh, 0, 0)
		}
	}()
}

func wndProc(hwnd uintptr, uMsg uint32, wParam, lParam uintptr) uintptr {
	switch uMsg {
	case wmEraseBkgnd:
		return 1

	case wmPaint:
		paintWindow(hwnd)
		return 0

	case wmAppRefresh:
		resizeAndAnchorBottomRight(hwnd, calcDesiredHeight())
		updateTrayIcon(hwnd)
		procInvalidateRect.Call(hwnd, 0, 0)
		return 0

	case wmTrayCallback:
		switch uint32(lParam) {
		case wmLButtonUp, wmLButtonDblClk:
			toggleWindowVisibility(hwnd)
		case wmRButtonUp:
			showTrayMenu(hwnd)
		}
		return 0

	case wmCommand:
		switch loword(wParam) {
		case idmToggle:
			toggleWindowVisibility(hwnd)
		case idmPin:
			state.mu.Lock()
			state.pinned = !state.pinned
			state.mu.Unlock()
			procInvalidateRect.Call(hwnd, 0, 0)
		case idmRefresh:
			triggerFetch()
		case idmExit:
			procDestroyWindow.Call(hwnd)
		}
		return 0

	case wmActivate:
		if loword(wParam) == waInactive {
			state.mu.Lock()
			pinned := state.pinned
			state.mu.Unlock()
			if !pinned {
				procShowWindow.Call(hwnd, swHide)
			}
		}
		return 0

	case wmMouseMove:
		pt := pointFromLParam(lParam)
		handleMouseMove(hwnd, pt)
		return 0

	case wmMouseLeave:
		state.mu.Lock()
		state.tracking = false
		changed := state.hoverBtn != btnNone
		state.hoverBtn = btnNone
		state.mu.Unlock()
		if changed {
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0

	case wmSetCursor:
		if loword(lParam) == htClient {
			state.mu.Lock()
			hover := state.hoverBtn
			state.mu.Unlock()
			if hover != btnNone {
				hand, _, _ := procLoadCursorW.Call(0, uintptr(idcHand))
				procSetCursor.Call(hand)
				return 1
			}
		}

	case wmLButtonDown:
		pt := pointFromLParam(lParam)
		handleLButtonDown(hwnd, pt)
		return 0

	case wmClose:
		procShowWindow.Call(hwnd, swHide)
		return 0

	case wmDestroy:
		removeTrayIcon(hwnd)
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(uMsg), wParam, lParam)
	return ret
}

func handleMouseMove(hwnd uintptr, pt point) {
	state.mu.Lock()
	if !state.tracking {
		tme := trackMouseEventStruct{
			CbSize:    uint32(unsafe.Sizeof(trackMouseEventStruct{})),
			DwFlags:   tmeLeave,
			HwndTrack: hwnd,
		}
		procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		state.tracking = true
	}
	newHover := hitTestButtonsLocked(pt)
	changed := newHover != state.hoverBtn
	state.hoverBtn = newHover
	state.mu.Unlock()

	if changed {
		procInvalidateRect.Call(hwnd, 0, 0)
	}
}

func handleLButtonDown(hwnd uintptr, pt point) {
	state.mu.Lock()
	btn := hitTestButtonsLocked(pt)
	inHeader := ptInRect(pt, state.headerRect)
	state.mu.Unlock()

	switch btn {
	case btnPin:
		state.mu.Lock()
		state.pinned = !state.pinned
		state.mu.Unlock()
		procInvalidateRect.Call(hwnd, 0, 0)
	case btnRefresh:
		triggerFetch()
	case btnHide:
		procShowWindow.Call(hwnd, swHide)
	case btnClose:
		procDestroyWindow.Call(hwnd)
	default:
		if inHeader {
			state.mu.Lock()
			state.userMoved = true
			state.mu.Unlock()
			procReleaseCapture.Call()
			procSendMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
		}
	}
}

func hitTestButtonsLocked(pt point) int {
	switch {
	case ptInRect(pt, state.btnCloseRect):
		return btnClose
	case ptInRect(pt, state.btnHideRect):
		return btnHide
	case ptInRect(pt, state.btnRefreshRect):
		return btnRefresh
	case ptInRect(pt, state.btnPinRect):
		return btnPin
	default:
		return btnNone
	}
}

func toggleWindowVisibility(hwnd uintptr) {
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis != 0 {
		procShowWindow.Call(hwnd, swHide)
		return
	}
	resizeAndAnchorBottomRight(hwnd, calcDesiredHeight())
	procShowWindow.Call(hwnd, swShow)
	procSetForegroundWindow.Call(hwnd)
	procInvalidateRect.Call(hwnd, 0, 0)
}

func showTrayMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	state.mu.Lock()
	pinned := state.pinned
	state.mu.Unlock()

	pinFlags := uint32(mfString)
	if pinned {
		pinFlags |= mfChecked
	} else {
		pinFlags |= mfUnchecked
	}

	appendMenuItem(hMenu, mfString, idmToggle, "Mostrar / Ocultar")
	appendMenuItem(hMenu, pinFlags, idmPin, "Modo Fixado")
	appendMenuItem(hMenu, mfString, idmRefresh, "Atualizar Agora")
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

func calcDesiredHeight() int32 {
	state.mu.Lock()
	data := state.data
	dpi := state.dpi
	state.mu.Unlock()

	const (
		headerH    = 44
		footerH    = 32
		outerPad   = 12
		groupGap   = 10
		groupPad   = 12
		groupHdrH  = 34
		bucketH    = 44
		emptyBodyH = 110
	)

	logicalH := headerH + outerPad + footerH
	if data == nil || len(data.Groups) == 0 {
		logicalH += emptyBodyH + outerPad
	} else {
		for i, g := range data.Groups {
			if i > 0 {
				logicalH += groupGap
			}
			logicalH += groupPad*2 + groupHdrH + len(g.Buckets)*bucketH
		}
		logicalH += outerPad
	}
	return scaleDPI(int32(logicalH), dpi)
}

func resizeAndAnchorBottomRight(hwnd uintptr, targetH int32) {
	state.mu.Lock()
	dpi := state.dpi
	userMoved := state.userMoved
	state.mu.Unlock()

	targetW := scaleDPI(416, dpi)
	margin := scaleDPI(12, dpi)

	var cur rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&cur)))
	curW := cur.Right - cur.Left
	curH := cur.Bottom - cur.Top

	if userMoved {
		if curW == targetW && curH == targetH {
			return
		}
		procSetWindowPos.Call(
			hwnd, 0,
			uintptr(cur.Left), uintptr(cur.Top),
			uintptr(targetW), uintptr(targetH),
			uintptr(swpNoZOrder|swpNoActivate),
		)
		return
	}

	var wa rect
	procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0)
	x := wa.Right - targetW - margin
	y := wa.Bottom - targetH - margin
	if x < wa.Left {
		x = wa.Left
	}
	if y < wa.Top {
		y = wa.Top
	}

	if cur.Left == x && cur.Top == y && curW == targetW && curH == targetH {
		return
	}

	procSetWindowPos.Call(
		hwnd, 0,
		uintptr(x), uintptr(y),
		uintptr(targetW), uintptr(targetH),
		uintptr(swpNoZOrder|swpNoActivate),
	)
}

func paintWindow(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	if w <= 0 || h <= 0 {
		return
	}

	memDC, _, _ := procCreateCompatibleDC.Call(hdc)
	memBmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	oldBmp, _, _ := procSelectObject.Call(memDC, memBmp)

	renderCard(memDC, w, h)

	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, srcCopy)
	procSelectObject.Call(memDC, oldBmp)
	procDeleteObject.Call(memBmp)
	procDeleteDC.Call(memDC)
}

func renderCard(hdc uintptr, w, h int32) {
	state.mu.Lock()
	dpi := state.dpi
	data := state.data
	loading := state.loading
	lastErr := state.lastErr
	lastUpdated := state.lastUpdated
	pinned := state.pinned
	hoverBtn := state.hoverBtn
	state.mu.Unlock()

	s := func(v int32) int32 { return scaleDPI(v, dpi) }

	colBg := rgb(0x14, 0x17, 0x1F)
	colHeader := rgb(0x1B, 0x20, 0x2B)
	colCard := rgb(0x1E, 0x24, 0x30)
	colBorder := rgb(0x2E, 0x36, 0x46)
	colTrack := rgb(0x2A, 0x31, 0x40)
	colText := rgb(0xF3, 0xF6, 0xFC)
	colSub := rgb(0x9B, 0xA6, 0xB8)
	colMuted := rgb(0x6F, 0x7B, 0x8F)
	colBtn := rgb(0x26, 0x2D, 0x3B)
	colBtnHover := rgb(0x33, 0x3C, 0x4F)
	colCloseHover := rgb(0xC0, 0x39, 0x2B)
	colBlue := rgb(0x3B, 0x82, 0xF6)

	fontTitle := createUIFont(s(16), fwSemibold, "Segoe UI Semibold")
	fontGroup := createUIFont(s(15), fwSemibold, "Segoe UI Semibold")
	fontBody := createUIFont(s(13), fwNormal, "Segoe UI")
	fontBold := createUIFont(s(13), fwSemibold, "Segoe UI Semibold")
	fontSmall := createUIFont(s(12), fwNormal, "Segoe UI")
	defer func() {
		procDeleteObject.Call(fontTitle)
		procDeleteObject.Call(fontGroup)
		procDeleteObject.Call(fontBody)
		procDeleteObject.Call(fontBold)
		procDeleteObject.Call(fontSmall)
	}()

	procSetBkMode.Call(hdc, transparentBk)

	// Window background + subtle border
	fillSolidRect(hdc, rect{0, 0, w, h}, colBg)
	headerH := s(44)
	fillSolidRect(hdc, rect{0, 0, w, headerH}, colHeader)

	// Bottom separator under header
	fillSolidRect(hdc, rect{0, headerH - 1, w, headerH}, colBorder)

	// Header buttons (right-to-left)
	btnGap := s(6)
	btnH := s(24)
	btnY := (headerH - btnH) / 2
	rightCursor := w - s(10)

	closeW := s(26)
	closeRect := rect{rightCursor - closeW, btnY, rightCursor, btnY + btnH}
	rightCursor = closeRect.Left - btnGap

	hideW := s(26)
	hideRect := rect{rightCursor - hideW, btnY, rightCursor, btnY + btnH}
	rightCursor = hideRect.Left - btnGap

	refreshW := s(70)
	refreshRect := rect{rightCursor - refreshW, btnY, rightCursor, btnY + btnH}
	rightCursor = refreshRect.Left - btnGap

	pinW := s(68)
	pinRect := rect{rightCursor - pinW, btnY, rightCursor, btnY + btnH}

	// Title + status indicator dot
	dotColor := statusColorForData(data, loading)
	dotSize := s(9)
	dotX := s(14)
	dotY := (headerH - dotSize) / 2
	drawRoundRect(hdc, rect{dotX, dotY, dotX + dotSize, dotY + dotSize}, dotSize, dotColor, dotColor)

	titleRect := rect{dotX + dotSize + s(8), 0, pinRect.Left - s(8), headerH}
	drawText(hdc, appTitle, titleRect, fontTitle, colText, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)

	// Draw buttons
	pinText := "Flutuante"
	pinBg := colBtn
	if hoverBtn == btnPin {
		pinBg = colBtnHover
	}
	if pinned {
		pinText = "Fixado"
		pinBg = colBlue
		if hoverBtn == btnPin {
			pinBg = rgb(0x25, 0x63, 0xEB)
		}
	}
	drawButton(hdc, pinRect, pinText, fontSmall, pinBg, colText, s(6))

	refreshText := "Atualizar"
	if loading {
		refreshText = "..."
	}
	refreshBg := colBtn
	if hoverBtn == btnRefresh {
		refreshBg = colBtnHover
	}
	drawButton(hdc, refreshRect, refreshText, fontSmall, refreshBg, colText, s(6))

	hideBg := colBtn
	if hoverBtn == btnHide {
		hideBg = colBtnHover
	}
	drawButton(hdc, hideRect, "_", fontBold, hideBg, colText, s(6))

	closeBg := colBtn
	if hoverBtn == btnClose {
		closeBg = colCloseHover
	}
	drawButton(hdc, closeRect, "X", fontSmall, closeBg, colText, s(6))

	state.mu.Lock()
	state.btnPinRect = pinRect
	state.btnRefreshRect = refreshRect
	state.btnHideRect = hideRect
	state.btnCloseRect = closeRect
	state.headerRect = rect{0, 0, w, headerH}
	state.mu.Unlock()

	// Body area
	outerPad := s(12)
	footerH := s(32)
	bodyY := headerH + outerPad
	cardW := w - outerPad*2
	now := time.Now()

	if data == nil || len(data.Groups) == 0 {
		msgRect := rect{outerPad, bodyY, w - outerPad, h - footerH - outerPad}
		drawRoundRect(hdc, msgRect, s(10), colCard, colBorder)
		msgText := "Carregando cotas do agy..."
		msgColor := colSub
		if !loading && lastErr != "" {
			msgText = "Erro: " + lastErr
			msgColor = rgb(0xE7, 0x4C, 0x3C)
		}
		drawText(hdc, msgText, msgRect, fontBody, msgColor, dtCenter|dtVCenter|dtSingleLine|dtEndEllipsis)
	} else {
		groupGap := s(10)
		groupPad := s(12)
		groupHdrH := s(34)
		bucketH := s(44)

		curY := bodyY
		for i, g := range data.Groups {
			if i > 0 {
				curY += groupGap
			}
			cardH := groupPad*2 + groupHdrH + int32(len(g.Buckets))*bucketH
			cardRect := rect{outerPad, curY, outerPad + cardW, curY + cardH}
			drawRoundRect(hdc, cardRect, s(10), colCard, colBorder)

			innerL := cardRect.Left + groupPad
			innerR := cardRect.Right - groupPad
			y := cardRect.Top + groupPad

			drawText(hdc, g.Name, rect{innerL, y, innerR, y + s(18)}, fontGroup, colText, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)
			drawText(hdc, ShortGroupSubtitle(g), rect{innerL, y + s(18), innerR, y + s(32)}, fontSmall, colSub, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)
			y += groupHdrH

			for _, b := range g.Buckets {
				pct := clamp01(b.RemainingFraction) * 100.0
				pctText := fmt.Sprintf("%.1f%%", pct)
				barColor := fractionColor(b.RemainingFraction)

				drawText(hdc, FriendlyBucketName(b), rect{innerL, y, innerR - s(58), y + s(16)}, fontBody, colText, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)
				drawText(hdc, pctText, rect{innerR - s(56), y, innerR, y + s(16)}, fontBold, barColor, dtRight|dtVCenter|dtSingleLine)

				barTop := y + s(19)
				barBot := barTop + s(7)
				trackRect := rect{innerL, barTop, innerR, barBot}
				drawRoundRect(hdc, trackRect, s(6), colTrack, colTrack)

				fillW := int32(math.Round(float64(innerR-innerL) * clamp01(b.RemainingFraction)))
				if fillW > 0 {
					if fillW < s(6) {
						fillW = s(6)
					}
					fillRect := rect{innerL, barTop, innerL + fillW, barBot}
					drawRoundRect(hdc, fillRect, s(6), barColor, barColor)
				}

				resetText := FormatCountdown(b.ResetTime, now)
				drawText(hdc, resetText, rect{innerL, barBot + s(2), innerR, barBot + s(17)}, fontSmall, colMuted, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)

				y += bucketH
			}
			curY += cardH
		}
	}

	// Footer status line
	footerTop := h - footerH
	fillSolidRect(hdc, rect{0, footerTop, w, footerTop + 1}, colBorder)
	footerText := "Aguardando primeira leitura..."
	footerColor := colMuted
	switch {
	case loading:
		footerText = "Atualizando cotas no agy..."
		footerColor = colBlue
	case lastErr != "" && data != nil:
		footerText = "Aviso na última atualização (exibindo cache)"
		footerColor = rgb(0xF3, 0x9C, 0x12)
	case lastErr != "":
		footerText = "Falha ao consultar agy"
		footerColor = rgb(0xE7, 0x4C, 0x3C)
	case !lastUpdated.IsZero():
		footerText = fmt.Sprintf("Atualizado às %s  •  Auto a cada 3m", lastUpdated.Format("15:04:05"))
	}
	drawText(hdc, footerText, rect{s(14), footerTop, w - s(14), h}, fontSmall, footerColor, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)

	// Outer 1px border frame
	drawBorderRect(hdc, rect{0, 0, w, h}, colBorder)
}

func drawButton(hdc uintptr, r rect, label string, hFont uintptr, bg, fg uint32, radius int32) {
	drawRoundRect(hdc, r, radius, bg, bg)
	drawText(hdc, label, r, hFont, fg, dtCenter|dtVCenter|dtSingleLine)
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

func applyRoundedCorners(hwnd uintptr) {
	pref := uint32(dwmwcpRound)
	procDwmSetWindowAttrib.Call(hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
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

func statusColorForData(data *UsageData, loading bool) uint32 {
	if data == nil {
		if loading {
			return rgb(0x3B, 0x82, 0xF6)
		}
		return rgb(0x6F, 0x7B, 0x8F)
	}
	return fractionColor(LowestFraction(data))
}

func fractionColor(f float64) uint32 {
	switch {
	case f < 0:
		return rgb(0x6F, 0x7B, 0x8F)
	case f >= 0.50:
		return rgb(0x2E, 0xCC, 0x71) // Green
	case f >= 0.20:
		return rgb(0xF3, 0x9C, 0x12) // Yellow/Orange
	default:
		return rgb(0xE7, 0x4C, 0x3C) // Red
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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
