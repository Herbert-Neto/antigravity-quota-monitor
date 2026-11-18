//go:build !windows

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const appTitle = "AGY Quota Monitor"

type unixState struct {
	mu              sync.Mutex
	data            *UsageData
	loading         bool
	lastErr         string
	lastUpdated     time.Time
	pinned          bool
	updateInfo      *UpdateInfo
	checkingUpdate  bool
	statusNote      string
	statusNoteUntil time.Time
}

var uState = &unixState{
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

func triggerFetch() {
	uState.mu.Lock()
	if uState.loading {
		uState.mu.Unlock()
		return
	}
	uState.loading = true
	uState.mu.Unlock()

	go func() {
		data, err := FetchQuota(context.Background())

		uState.mu.Lock()
		uState.loading = false
		if err != nil {
			uState.lastErr = err.Error()
		} else {
			uState.data = data
			uState.lastErr = ""
			uState.lastUpdated = time.Now()
		}
		uState.mu.Unlock()
	}()
}

func setUnixStatusNote(note string, d time.Duration) {
	uState.mu.Lock()
	uState.statusNote = note
	if note == "" || d <= 0 {
		uState.statusNoteUntil = time.Time{}
	} else {
		uState.statusNoteUntil = time.Now().Add(d)
	}
	uState.mu.Unlock()
}

func triggerUpdateCheck(manual bool) {
	uState.mu.Lock()
	if uState.checkingUpdate {
		uState.mu.Unlock()
		return
	}
	uState.checkingUpdate = true
	if manual {
		uState.statusNote = "Verificando atualizações no GitHub..."
		uState.statusNoteUntil = time.Now().Add(15 * time.Second)
	}
	uState.mu.Unlock()

	go func() {
		info, err := CheckForUpdates(context.Background())

		uState.mu.Lock()
		uState.checkingUpdate = false
		if err != nil {
			if manual {
				uState.statusNote = "Erro ao verificar atualização"
				uState.statusNoteUntil = time.Now().Add(6 * time.Second)
			}
		} else {
			uState.updateInfo = info
			if info.HasUpdate {
				uState.statusNote = fmt.Sprintf("Nova versão disponível: %s", info.LatestVersion)
				uState.statusNoteUntil = time.Now().Add(10 * time.Second)
			} else if manual {
				uState.statusNote = fmt.Sprintf("Você já está na versão mais recente (%s)", CurrentVersion())
				uState.statusNoteUntil = time.Now().Add(6 * time.Second)
			}
		}
		uState.mu.Unlock()
	}()
}

func startUnixBackgroundLoop(ctx context.Context) {
	go func() {
		fetchTicker := time.NewTicker(3 * time.Minute)
		updateTicker := time.NewTicker(6 * time.Hour)
		defer fetchTicker.Stop()
		defer updateTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-fetchTicker.C:
				triggerFetch()
			case <-updateTicker.C:
				triggerUpdateCheck(false)
			}
		}
	}()
}

func runGUI() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao iniciar servidor local: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quitCh := make(chan struct{}, 1)
	var quitOnce sync.Once
	requestQuit := func() {
		quitOnce.Do(func() {
			close(quitCh)
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/state", handleState)
	mux.HandleFunc("/api/refresh", handleRefresh)
	mux.HandleFunc("/api/pin", handlePin)
	mux.HandleFunc("/api/autostart", handleAutoStart)
	mux.HandleFunc("/api/update-check", handleUpdateCheck)
	mux.HandleFunc("/api/quit", handleQuit(requestQuit))

	srv := &http.Server{Handler: mux}

	triggerFetch()
	triggerUpdateCheck(false)
	startUnixBackgroundLoop(ctx)

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Erro no servidor HTTP local: %v\n", serveErr)
			requestQuit()
		}
	}()

	url := "http://" + ln.Addr().String()
	if err := openAppWindow(url); err != nil {
		fmt.Fprintf(os.Stderr, "Aviso: abra manualmente no navegador: %s (%v)\n", url, err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
	case <-quitCh:
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func openAppWindow(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		chromeCandidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
		}
		for _, bin := range chromeCandidates {
			if _, err := os.Stat(bin); err == nil {
				cmd = exec.Command(bin, "--app="+url, "--window-size=416,514")
				break
			}
		}
		if cmd == nil {
			cmd = exec.Command("open", url)
		}

	case "linux":
		linuxBrowsers := []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
			"brave-browser",
			"microsoft-edge",
		}
		for _, name := range linuxBrowsers {
			if bin, err := exec.LookPath(name); err == nil && bin != "" {
				cmd = exec.Command(bin, "--app="+url, "--window-size=416,514")
				break
			}
		}
		if cmd == nil {
			cmd = exec.Command("xdg-open", url)
		}

	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
