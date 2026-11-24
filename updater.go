package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// version can be overridden at build time via -ldflags "-X main.version=v1.2.1".
var version = "v1.2.1"

const (
	githubLatestReleaseAPI = "https://api.github.com/repos/Herbert-Neto/antigravity-quota-monitor/releases/latest"
	githubReleasesPageURL  = "https://github.com/Herbert-Neto/antigravity-quota-monitor/releases/latest"
)

// UpdateInfo describes the latest available release on GitHub.
type UpdateInfo struct {
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion"`
	ReleaseURL     string    `json:"releaseURL"`
	HasUpdate      bool      `json:"hasUpdate"`
	CheckedAt      time.Time `json:"checkedAt"`
}

// CurrentVersion returns the embedded application version string.
func CurrentVersion() string {
	v := strings.TrimSpace(version)
	if v == "" {
		return "dev"
	}
	return v
}

// CheckForUpdates queries the GitHub Releases API for a newer release tag.
func CheckForUpdates(ctx context.Context) (*UpdateInfo, error) {
	return checkForUpdatesFromURL(ctx, githubLatestReleaseAPI, CurrentVersion())
}

type githubReleasePayload struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func checkForUpdatesFromURL(ctx context.Context, apiURL, currentVer string) (*UpdateInfo, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao preparar requisicao de atualizacao: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agy-quota-monitor/"+currentVer)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub retornou HTTP %d", resp.StatusCode)
	}

	var payload githubReleasePayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("erro ao ler resposta do GitHub: %w", err)
	}

	latest := strings.TrimSpace(payload.TagName)
	if latest == "" {
		return nil, fmt.Errorf("release sem tag_name")
	}

	releaseURL := strings.TrimSpace(payload.HTMLURL)
	if !isAllowedBrowserURL(releaseURL) {
		releaseURL = githubReleasesPageURL
	}

	hasUpdate := !payload.Draft && !payload.Prerelease && CompareSemver(latest, currentVer) > 0
	return &UpdateInfo{
		CurrentVersion: currentVer,
		LatestVersion:  latest,
		ReleaseURL:     releaseURL,
		HasUpdate:      hasUpdate,
		CheckedAt:      time.Now(),
	}, nil
}

// CompareSemver compares two semantic versions (e.g., "v1.2.0" vs "1.1.0").
// Returns 1 if a > b, -1 if a < b, and 0 if equal or non-comparable (e.g., "dev").
func CompareSemver(a, b string) int {
	pa, okA := parseSemver(a)
	pb, okB := parseSemver(b)
	if !okA || !okB {
		return 0
	}

	for i := 0; i < 3; i++ {
		if pa.nums[i] > pb.nums[i] {
			return 1
		}
		if pa.nums[i] < pb.nums[i] {
			return -1
		}
	}

	switch {
	case pa.pre == "" && pb.pre != "":
		return 1
	case pa.pre != "" && pb.pre == "":
		return -1
	default:
		return strings.Compare(pa.pre, pb.pre)
	}
}

type semverParts struct {
	nums [3]int
	pre  string
}

func parseSemver(raw string) (semverParts, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if s == "" || strings.EqualFold(s, "dev") {
		return semverParts{}, false
	}

	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		s = s[:idx]
	}

	var pre string
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		pre = s[idx+1:]
		s = s[:idx]
	}

	fields := strings.Split(s, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return semverParts{}, false
	}

	var out semverParts
	out.pre = pre
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return semverParts{}, false
		}
		out.nums[i] = n
	}
	return out, true
}

func isAllowedBrowserURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || strings.ContainsAny(trimmed, "\r\n\x00") {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := u.Hostname()
	return strings.EqualFold(host, "github.com") || strings.EqualFold(host, "www.github.com")
}
