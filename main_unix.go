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
	mu          sync.Mutex
	data        *UsageData
	loading     bool
	lastErr     string
	lastUpdated time.Time
	pinned      bool
}

var uState = &unixState{
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

func startUnixBackgroundLoop(ctx context.Context) {
	go func() {
		fetchTicker := time.NewTicker(3 * time.Minute)
		defer fetchTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-fetchTicker.C:
				triggerFetch()
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
	mux.HandleFunc("/api/quit", handleQuit(requestQuit))

	srv := &http.Server{Handler: mux}

	triggerFetch()
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
