//go:build windows

package main

import "testing"

func TestIsWindowRectFullscreen(t *testing.T) {
	mon := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	work := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}

	tests := []struct {
		name  string
		win   rect
		mon   rect
		work  rect
		style uint32
		want  bool
	}{
		{
			name:  "borderless fullscreen",
			win:   rect{0, 0, 1920, 1080},
			mon:   mon,
			work:  work,
			style: 0x80000000, // WS_POPUP
			want:  true,
		},
		{
			name:  "exclusive or oversized fullscreen",
			win:   rect{-1, -1, 1921, 1081},
			mon:   mon,
			work:  work,
			style: 0,
			want:  true,
		},
		{
			name:  "maximized window with visible taskbar",
			win:   rect{-8, -8, 1928, 1048},
			mon:   mon,
			work:  work,
			style: wsCaption | wsMaximize,
			want:  false,
		},
		{
			name:  "maximized window with auto-hide taskbar",
			win:   rect{0, 0, 1920, 1080},
			mon:   mon,
			work:  mon,
			style: wsCaption | wsMaximize,
			want:  false,
		},
		{
			name:  "normal windowed app",
			win:   rect{100, 100, 1280, 800},
			mon:   mon,
			work:  work,
			style: wsCaption,
			want:  false,
		},
		{
			name:  "invalid monitor rect",
			win:   rect{0, 0, 1920, 1080},
			mon:   rect{0, 0, 0, 0},
			work:  work,
			style: 0,
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWindowRectFullscreen(tc.win, tc.mon, tc.work, tc.style); got != tc.want {
				t.Fatalf("isWindowRectFullscreen(...) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsIgnoredFullscreenClass(t *testing.T) {
	ignored := []string{"Progman", "WorkerW", "Shell_TrayWnd", "Shell_SecondaryTrayWnd"}
	for _, cls := range ignored {
		if !isIgnoredFullscreenClass(cls) {
			t.Fatalf("expected class %q to be ignored", cls)
		}
	}

	notIgnored := []string{"UnrealWindow", "SDL_app", "Chrome_WidgetWin_1", ""}
	for _, cls := range notIgnored {
		if isIgnoredFullscreenClass(cls) {
			t.Fatalf("expected class %q not to be ignored", cls)
		}
	}
}
