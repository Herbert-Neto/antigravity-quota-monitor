//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
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

	idmToggle      = 1001
	idmPin         = 1002
	idmRefresh     = 1003
	idmExit        = 1004
	idmAutoStart   = 1005
	idmCheckUpdate = 1006

	btnNone = iota
	btnPin
	btnRefresh
	btnHide
	btnClose
	btnUpdate
)

type appState struct {
	mu              sync.Mutex
	hwnd            uintptr
	hTrayIcon       uintptr
	dpi             int32
	data            *UsageData
	loading         bool
	lastErr         string
	lastUpdated     time.Time
	pinned          bool
	userMoved       bool
	hoverBtn        int
	tracking        bool
	updateInfo      *UpdateInfo
	checkingUpdate  bool
	statusNote      string
	statusNoteUntil time.Time

	btnPinRect     rect
	btnRefreshRect rect
	btnHideRect    rect
	btnCloseRect   rect
	btnUpdateRect  rect
	headerRect     rect
}

var state = &appState{
	dpi:    96,
	pinned: true,
}

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version", "-v":
			fmt.Printf("%s %s\n", appTitle, CurrentVersion())
			os.Exit(0)
		case "--test-cli":
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
	triggerUpdateCheck(false)
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
		updateTicker := time.NewTicker(6 * time.Hour)
		defer fetchTicker.Stop()
		defer countdownTicker.Stop()
		defer updateTicker.Stop()

		for {
			select {
			case <-fetchTicker.C:
				triggerFetch()
			case <-countdownTicker.C:
				procPostMessageW.Call(hwnd, wmAppRefresh, 0, 0)
			case <-updateTicker.C:
				triggerUpdateCheck(false)
			}
		}
	}()
}

func setStatusNote(note string, d time.Duration) {
	state.mu.Lock()
	state.statusNote = note
	if note == "" || d <= 0 {
		state.statusNoteUntil = time.Time{}
	} else {
		state.statusNoteUntil = time.Now().Add(d)
	}
	h := state.hwnd
	state.mu.Unlock()

	if h != 0 {
		procPostMessageW.Call(h, wmAppRefresh, 0, 0)
	}
}

func triggerUpdateCheck(manual bool) {
	state.mu.Lock()
	if state.checkingUpdate {
		state.mu.Unlock()
		return
	}
	state.checkingUpdate = true
	if manual {
		state.statusNote = "Verificando atualizações no GitHub..."
		state.statusNoteUntil = time.Now().Add(15 * time.Second)
	}
	hwnd := state.hwnd
	state.mu.Unlock()

	if hwnd != 0 && manual {
		procPostMessageW.Call(hwnd, wmAppRefresh, 0, 0)
	}

	go func() {
		info, err := CheckForUpdates(context.Background())

		state.mu.Lock()
		state.checkingUpdate = false
		if err != nil {
			if manual {
				state.statusNote = "Erro ao verificar atualização"
				state.statusNoteUntil = time.Now().Add(6 * time.Second)
			}
		} else {
			state.updateInfo = info
			if info.HasUpdate {
				state.statusNote = fmt.Sprintf("Nova versão disponível: %s", info.LatestVersion)
				state.statusNoteUntil = time.Now().Add(10 * time.Second)
			} else if manual {
				state.statusNote = fmt.Sprintf("Você já está na versão mais recente (%s)", CurrentVersion())
				state.statusNoteUntil = time.Now().Add(6 * time.Second)
			}
		}
		h := state.hwnd
		state.mu.Unlock()

		if h != 0 {
			procPostMessageW.Call(h, wmAppRefresh, 0, 0)
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
		case idmAutoStart:
			enabled, err := ToggleAutoStart()
			if err != nil {
				setStatusNote("Erro no início automático: "+err.Error(), 6*time.Second)
			} else if enabled {
				setStatusNote("Início com o Windows ativado", 5*time.Second)
			} else {
				setStatusNote("Início com o Windows desativado", 5*time.Second)
			}
			procInvalidateRect.Call(hwnd, 0, 0)
		case idmRefresh:
			triggerFetch()
		case idmCheckUpdate:
			state.mu.Lock()
			info := state.updateInfo
			state.mu.Unlock()
			if info != nil && info.HasUpdate && info.ReleaseURL != "" {
				if err := OpenBrowserURL(info.ReleaseURL); err != nil {
					setStatusNote("Erro ao abrir navegador", 6*time.Second)
				}
			} else {
				triggerUpdateCheck(true)
			}
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
	case btnUpdate:
		state.mu.Lock()
		info := state.updateInfo
		state.mu.Unlock()
		if info != nil && info.HasUpdate && info.ReleaseURL != "" {
			if err := OpenBrowserURL(info.ReleaseURL); err != nil {
				setStatusNote("Erro ao abrir navegador", 6*time.Second)
			}
		}
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
	case ptInRect(pt, state.btnUpdateRect):
		return btnUpdate
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
