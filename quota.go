//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const createNoWindow = 0x08000000

// UsageOutput represents the top-level JSON returned by `agy --print /usage --output-format json`.
type UsageOutput struct {
	Status  string      `json:"status"`
	Command CommandInfo `json:"command"`
}

// CommandInfo holds the executed slash command metadata and payload.
type CommandInfo struct {
	Name string    `json:"name"`
	Data UsageData `json:"data"`
}

// UsageData holds the quota description and model groups.
type UsageData struct {
	Description string       `json:"description"`
	Groups      []QuotaGroup `json:"groups"`
}

// QuotaGroup represents a logical model family sharing quota buckets.
type QuotaGroup struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Buckets     []QuotaBucket `json:"buckets"`
}

// QuotaBucket represents a single quota window (e.g., 5h or weekly).
type QuotaBucket struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Description       string  `json:"description,omitempty"`
	Window            string  `json:"window"`
	RemainingFraction float64 `json:"remaining_fraction"`
	ResetTime         string  `json:"reset_time"`
}

// FindAgyExecutable resolves the path to the agy CLI binary.
func FindAgyExecutable() string {
	if path, err := exec.LookPath("agy"); err == nil && path != "" {
		return path
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidate := filepath.Join(localAppData, "agy", "bin", "agy.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "agy"
}

// FetchQuota runs `agy --print /usage --output-format json` without opening a console window
// and parses the resulting quota groups.
func FetchQuota(ctx context.Context) (*UsageData, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	agyPath := FindAgyExecutable()
	cmd := exec.CommandContext(ctx, agyPath, "--print", "/usage", "--output-format", "json")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		return nil, fmt.Errorf("falha ao executar agy: %s", errText)
	}

	out, err := parseUsageJSON(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	if out.Status != "SUCCESS" {
		return nil, fmt.Errorf("agy retornou status %q", out.Status)
	}
	return &out.Command.Data, nil
}

func parseUsageJSON(raw []byte) (*UsageOutput, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("saida vazia do agy")
	}

	// Scan lines first in case any diagnostic line precedes the JSON object.
	for _, line := range bytes.Split(trimmed, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var out UsageOutput
		if err := json.Unmarshal(line, &out); err == nil && out.Command.Name != "" {
			return &out, nil
		}
	}

	// Fallback: try decoding from the first '{' in stdout.
	idx := bytes.IndexByte(trimmed, '{')
	if idx < 0 {
		return nil, errors.New("nenhum JSON valido encontrado na saida do agy")
	}
	var out UsageOutput
	if err := json.Unmarshal(trimmed[idx:], &out); err != nil {
		return nil, fmt.Errorf("erro ao interpretar JSON do agy: %w", err)
	}
	return &out, nil
}

// FormatCountdown converts an ISO-8601 UTC reset timestamp into a local Portuguese countdown string.
func FormatCountdown(resetTimeISO string, now time.Time) string {
	reset, err := time.Parse(time.RFC3339Nano, resetTimeISO)
	if err != nil {
		reset, err = time.Parse(time.RFC3339, resetTimeISO)
		if err != nil {
			return "Horário de reset indisponível"
		}
	}

	localReset := reset.Local()
	diff := reset.Sub(now)
	if diff <= 0 {
		return "Resetando agora"
	}

	totalMinutes := int(diff.Minutes())
	days := totalMinutes / (60 * 24)
	hours := (totalMinutes % (60 * 24)) / 60
	minutes := totalMinutes % 60

	if days > 0 {
		return fmt.Sprintf("Reseta em %dd %02dh (%s)", days, hours, localReset.Format("02/01 15:04"))
	}
	if hours > 0 {
		return fmt.Sprintf("Reseta em %dh %02dm (%s)", hours, minutes, localReset.Format("15:04"))
	}
	if minutes > 0 {
		return fmt.Sprintf("Reseta em %dm (%s)", minutes, localReset.Format("15:04"))
	}
	return fmt.Sprintf("Reseta em <1m (%s)", localReset.Format("15:04"))
}

// LowestFraction returns the minimum remaining_fraction across all buckets, or -1 if unavailable.
func LowestFraction(data *UsageData) float64 {
	if data == nil {
		return -1
	}
	lowest := -1.0
	for _, g := range data.Groups {
		for _, b := range g.Buckets {
			if lowest < 0 || b.RemainingFraction < lowest {
				lowest = b.RemainingFraction
			}
		}
	}
	return lowest
}

// FriendlyBucketName maps English bucket labels to concise Portuguese labels for the UI.
func FriendlyBucketName(b QuotaBucket) string {
	switch {
	case b.ID == "gemini-argon-wtu-weekly":
		return "Limite Semanal Argon"
	case b.Window == "5h":
		return "Limite de 5 Horas"
	case b.Window == "weekly":
		return "Limite Semanal"
	default:
		return b.Name
	}
}

// ShortGroupSubtitle extracts a concise subtitle from the group's description.
func ShortGroupSubtitle(g QuotaGroup) string {
	desc := strings.TrimSpace(g.Description)
	if strings.HasPrefix(desc, "Models within this group:") {
		return strings.TrimSpace(strings.TrimPrefix(desc, "Models within this group:"))
	}
	if strings.Contains(desc, "Argon quota is separate") {
		return "Cota semanal dedicada (Plano Pro)"
	}
	return desc
}
