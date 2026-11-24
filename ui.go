//go:build windows

package main

import (
	"fmt"
	"math"
	"time"
	"unsafe"
)

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
	pendingFetch := state.pendingFetch
	lastErr := state.lastErr
	lastUpdated := state.lastUpdated
	pinned := state.pinned
	hoverBtn := state.hoverBtn
	updateInfo := state.updateInfo
	statusNote := state.statusNote
	statusNoteUntil := state.statusNoteUntil
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

	footerRight := w - s(14)
	var updateRect rect
	if updateInfo != nil && updateInfo.HasUpdate {
		updateRect = rect{w - s(106), footerTop + s(4), w - s(12), h - s(4)}
		footerRight = updateRect.Left - s(8)
		updateBg := colBlue
		if hoverBtn == btnUpdate {
			updateBg = rgb(0x25, 0x63, 0xEB)
		}
		drawButton(hdc, updateRect, fmt.Sprintf("Baixar %s", updateInfo.LatestVersion), fontSmall, updateBg, colText, s(5))
	}
	state.mu.Lock()
	state.btnUpdateRect = updateRect
	state.mu.Unlock()

	footerText := "Aguardando primeira leitura..."
	footerColor := colMuted
	switch {
	case statusNote != "" && now.Before(statusNoteUntil):
		footerText = statusNote
		footerColor = colBlue
	case loading:
		footerText = "Atualizando cotas no agy..."
		footerColor = colBlue
	case lastErr != "" && data != nil:
		footerText = "Aviso na última atualização (exibindo cache)"
		footerColor = rgb(0xF3, 0x9C, 0x12)
	case lastErr != "":
		footerText = "Falha ao consultar agy"
		footerColor = rgb(0xE7, 0x4C, 0x3C)
	case !lastUpdated.IsZero() && pendingFetch:
		footerText = fmt.Sprintf("Atualizado às %s  •  Pausado (tela cheia)", lastUpdated.Format("15:04:05"))
	case !lastUpdated.IsZero():
		footerText = fmt.Sprintf("Atualizado às %s  •  Auto a cada 3m", lastUpdated.Format("15:04:05"))
	}
	drawText(hdc, footerText, rect{s(14), footerTop, footerRight, h}, fontSmall, footerColor, dtLeft|dtVCenter|dtSingleLine|dtEndEllipsis)

	// Outer 1px border frame
	drawBorderRect(hdc, rect{0, 0, w, h}, colBorder)
}

func drawButton(hdc uintptr, r rect, label string, hFont uintptr, bg, fg uint32, radius int32) {
	drawRoundRect(hdc, r, radius, bg, bg)
	drawText(hdc, label, r, hFont, fg, dtCenter|dtVCenter|dtSingleLine)
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
