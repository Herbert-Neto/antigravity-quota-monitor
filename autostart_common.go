package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	autoStartRegistryKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartAppName         = "AGYQuotaMonitor"
	linuxDesktopFileName     = "agy-quota.desktop"
	macLaunchAgentLabel      = "com.herbertneto.agy-quota"
	macLaunchAgentFileName   = "com.herbertneto.agy-quota.plist"
)

func resolveExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("nao foi possivel localizar o executavel: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	absPath, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("caminho do executavel invalido: %w", err)
	}
	if isTemporaryGoRunBinary(absPath) {
		return "", errors.New("inicio automatico indisponivel via 'go run'; compile o binario primeiro")
	}
	return absPath, nil
}

func isTemporaryGoRunBinary(execPath string) bool {
	cleaned := strings.ToLower(filepath.ToSlash(filepath.Clean(execPath)))
	if strings.Contains(cleaned, "/go-build") {
		return true
	}
	tmpDir := os.TempDir()
	if tmpDir != "" {
		normTmp := strings.ToLower(filepath.ToSlash(filepath.Clean(tmpDir)))
		if !strings.HasSuffix(normTmp, "/") {
			normTmp += "/"
		}
		if strings.HasPrefix(cleaned, normTmp) {
			return true
		}
	}
	return false
}

func buildDesktopEntry(execPath string) string {
	escapedExec := strings.ReplaceAll(execPath, `"`, `\"`)
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Version=1.0
Name=AGY Quota Monitor
Comment=Monitor de cotas do Google Antigravity CLI (agy)
Exec="%s"
Terminal=false
Categories=Utility;Monitor;
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, escapedExec)
}

func buildLaunchAgentPlist(execPath string) string {
	var escapedPath bytes.Buffer
	_ = xml.EscapeText(&escapedPath, []byte(execPath))

	var escapedLabel bytes.Buffer
	_ = xml.EscapeText(&escapedLabel, []byte(macLaunchAgentLabel))

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`, escapedLabel.String(), escapedPath.String())
}
