//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func autoStartFilePath() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("diretorio home indisponivel")
		}
		return filepath.Join(home, "Library", "LaunchAgents", macLaunchAgentFileName), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return "", fmt.Errorf("diretorio de configuracao indisponivel")
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "autostart", linuxDesktopFileName), nil
}

// IsAutoStartEnabled reports whether the autostart descriptor exists for the current OS.
func IsAutoStartEnabled() bool {
	p, err := autoStartFilePath()
	if err != nil {
		return false
	}
	_, statErr := os.Stat(p)
	return statErr == nil
}

// SetAutoStart creates or removes the OS login autostart file (.desktop on Linux, LaunchAgent plist on macOS).
func SetAutoStart(enabled bool) error {
	p, err := autoStartFilePath()
	if err != nil {
		return err
	}

	if !enabled {
		if rmErr := os.Remove(p); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("erro ao remover arquivo de inicio automatico: %w", rmErr)
		}
		return nil
	}

	exePath, err := resolveExecutablePath()
	if err != nil {
		return err
	}

	if mkErr := os.MkdirAll(filepath.Dir(p), 0700); mkErr != nil {
		return fmt.Errorf("erro ao criar diretorio de autostart: %w", mkErr)
	}

	content := buildDesktopEntry(exePath)
	if runtime.GOOS == "darwin" {
		content = buildLaunchAgentPlist(exePath)
	}

	if wrErr := os.WriteFile(p, []byte(content), 0600); wrErr != nil {
		return fmt.Errorf("erro ao gravar arquivo de autostart: %w", wrErr)
	}
	return nil
}

// ToggleAutoStart flips the autostart state and returns the updated setting.
func ToggleAutoStart() (bool, error) {
	target := !IsAutoStartEnabled()
	if err := SetAutoStart(target); err != nil {
		return IsAutoStartEnabled(), err
	}
	return target, nil
}

// OpenBrowserURL opens an HTTP/HTTPS URL using the OS default browser handler.
func OpenBrowserURL(targetURL string) error {
	if !isAllowedBrowserURL(targetURL) {
		return fmt.Errorf("URL invalida para abertura no navegador")
	}
	bin := "xdg-open"
	if runtime.GOOS == "darwin" {
		bin = "open"
	}
	cmd := exec.Command(bin, targetURL)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("nao foi possivel abrir o navegador: %w", err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
