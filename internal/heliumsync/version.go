package heliumsync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/tinywifi/helium-sync-windows/releases/latest"

func (a App) AppVersion() string {
	if v := os.Getenv("HELIUM_SYNC_VERSION"); v != "" {
		return v
	}
	rev := strings.TrimSpace(a.GitRevision())
	if strings.HasPrefix(rev, "v") {
		return strings.TrimPrefix(rev, "v")
	}
	return "unknown"
}

func (a App) GitRevision() string {
	out, err := a.RunGit("describe", "--tags", "--dirty", "--always")
	if err != nil || strings.TrimSpace(out) == "" {
		return "not available"
	}
	return strings.TrimSpace(out)
}

func (a App) MaybeShowUpdateBanner() {
	if os.Getenv("HELIUM_SYNC_NO_UPDATE_CHECK") != "" {
		return
	}
	current := a.AppVersion()
	currentTuple, ok := versionTuple(current)
	if !ok {
		return
	}
	latest := latestReleaseVersion()
	latestTuple, ok := versionTuple(latest)
	if !ok || !isNewerVersion(latestTuple, currentTuple) {
		return
	}
	fmt.Println(updateBanner(current, latest))
	fmt.Println()
}

func latestReleaseVersion() string {
	client := http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", latestReleaseURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "helium-sync")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	return strings.TrimPrefix(payload.TagName, "v")
}

func versionTuple(version string) ([3]int, bool) {
	var tuple [3]int
	re := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
	match := re.FindStringSubmatch(version)
	if match == nil {
		return tuple, false
	}
	for i := 0; i < 3; i++ {
		for _, ch := range match[i+1] {
			tuple[i] = tuple[i]*10 + int(ch-'0')
		}
	}
	return tuple, true
}

func isNewerVersion(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func updateBanner(current, latest string) string {
	lines := []string{
		fmt.Sprintf(" Update available: helium-sync %s -> %s ", current, latest),
		" Run: scoop update && scoop update helium-sync ",
	}
	width := len(lines[0])
	if len(lines[1]) > width {
		width = len(lines[1])
	}
	border := "+" + strings.Repeat("-", width) + "+"
	return strings.Join([]string{
		border,
		"|" + padRight(lines[0], width) + "|",
		"|" + padRight(lines[1], width) + "|",
		border,
	}, "\n")
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func runtimeSummary() string {
	return fmt.Sprintf("%s/%s %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
}
