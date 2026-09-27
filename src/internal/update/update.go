// Package update implements self-update version checking and upgrading
// for env-sync via GitHub releases and the remote install script.
package update

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"envsync/internal/config"
)

const (
	// githubRepo is the "owner/repo" used for release checks.
	githubRepo = "championswimmer/env.sync.local"
	// latestReleaseAPI is the primary source for the latest version.
	latestReleaseAPI = "https://api.github.com/repos/championswimmer/env.sync.local/releases/latest"
	// releasesPageBase links users to release notes/tags.
	releasesPageBase = "https://github.com/championswimmer/env.sync.local/releases/tag"

	// internalArg is the hidden subcommand argument the detached
	// background checker re-executes this binary with.
	internalArg = "__check-update-internal"
	// internalEnv guards TriggerBackgroundCheck against recursion.
	internalEnv = "ENV_SYNC_UPDATE_INTERNAL"

	// checkInterval is how often a background check may run.
	checkInterval = 12 * time.Hour
	// httpTimeout bounds all update-related network calls.
	httpTimeout = 10 * time.Second
)

// installerURLs lists the install.sh sources in priority order.
var installerURLs = []string{
	"https://envsync.arnav.tech/install.sh",
	"https://raw.githubusercontent.com/championswimmer/env.sync.local/main/install.sh",
}

// versionState is the JSON shape of the on-disk version-check cache:
// {"latest_version":"x.y.z","last_checked_at":"RFC3339"}.
type versionState struct {
	LatestVersion string `json:"latest_version"`
	LastCheckedAt string `json:"last_checked_at"`
}

// stateFilePath returns the cache file inside the config dir.
func stateFilePath() string {
	return filepath.Join(config.ConfigDir(), "version-check.json")
}

// readCache loads the cache file. Callers treat any error as "no cache".
func readCache() (versionState, error) {
	var st versionState
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	return st, nil
}

// writeCache persists the cache file with 0600 perms (dir 0700).
func writeCache(st versionState) error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(stateFilePath(), data, 0o600)
}

// isQuiet reports whether update output should be suppressed.
func isQuiet(args []string) bool {
	if strings.EqualFold(os.Getenv("ENV_SYNC_QUIET"), "true") {
		return true
	}
	for _, a := range args {
		if a == "-q" || a == "--quiet" {
			return true
		}
	}
	return false
}

// isTerminal reports whether stdin is a character device (interactive).
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// IsNewer reports whether latest is a newer semantic version than current.
// Leading "v"/"V" and surrounding whitespace are ignored. Each dot-separated
// segment is compared by its leading numeric prefix (missing segments count
// as 0). A "-" pre-release suffix sorts older than the same numbers without
// a suffix; two pre-releases with equal numbers compare lexically.
func IsNewer(latest, current string) bool {
	lNorm, lPre := splitPre(normalizeVersion(latest))
	cNorm, cPre := splitPre(normalizeVersion(current))

	lParts := strings.Split(lNorm, ".")
	cParts := strings.Split(cNorm, ".")

	n := len(lParts)
	if len(cParts) > n {
		n = len(cParts)
	}
	for i := 0; i < n; i++ {
		l := segmentNum(partAt(lParts, i))
		c := segmentNum(partAt(cParts, i))
		if l != c {
			return l > c
		}
	}
	// Numeric cores are equal: pre-release is older than plain release.
	if lPre == cPre {
		if lPre == "" {
			return false // equal
		}
		return lPre > cPre
	}
	if lPre == "" {
		return true // latest is a release, current is a pre-release
	}
	return false // latest is a pre-release, current is a release
}

// normalizeVersion trims whitespace and one leading v/V, and drops "+"
// build metadata (which does not affect precedence).
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = strings.TrimSpace(v[1:])
	}
	if i := strings.Index(v, "+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// splitPre cuts a "-pre-release" suffix, returning core and suffix.
func splitPre(v string) (string, string) {
	if i := strings.Index(v, "-"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func partAt(parts []string, i int) string {
	if i < len(parts) {
		return strings.TrimSpace(parts[i])
	}
	return "0"
}

// segmentNum parses the leading run of digits of one version segment.
// Segments without digits count as 0.
func segmentNum(s string) int {
	j := 0
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == 0 {
		return 0
	}
	n, err := strconv.Atoi(s[:j])
	if err != nil {
		return 0
	}
	return n
}

// releaseTagURL builds a release page link for a version tag,
// adding the "v" prefix when missing.
func releaseTagURL(version string) string {
	tag := strings.TrimSpace(version)
	if tag == "" {
		return "https://github.com/" + githubRepo + "/releases"
	}
	if !strings.HasPrefix(tag, "v") && !strings.HasPrefix(tag, "V") {
		tag = "v" + tag
	}
	return releasesPageBase + "/" + tag
}

// fetchLatestVersion queries the GitHub releases API for the latest tag.
// The returned version has whitespace and a leading v/V stripped.
func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: httpTimeout}
	req, err := http.NewRequest(http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "env-sync/"+config.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("github api: unexpected status %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	latest := normalizeVersion(payload.TagName)
	if latest == "" {
		return "", fmt.Errorf("github api: empty tag_name")
	}
	return latest, nil
}

// TriggerBackgroundCheck spawns a detached background version check when
// one is due (cache missing or older than 12h). It is non-blocking and
// returns quickly (<50ms, file read only, no network). It never prints and
// never fails hard; all errors are silently ignored. When the
// ENV_SYNC_UPDATE_INTERNAL guard is set it is a no-op (no recursion).
func TriggerBackgroundCheck() {
	defer func() {
		_ = recover()
	}()
	if os.Getenv(internalEnv) == "1" {
		return
	}
	if st, err := readCache(); err == nil && st.LastCheckedAt != "" {
		if ts, err := time.Parse(time.RFC3339, st.LastCheckedAt); err == nil {
			if time.Since(ts) < checkInterval {
				return
			}
		}
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return
	}
	cmd := exec.Command(exe, internalArg)
	cmd.Env = append(os.Environ(), internalEnv+"=1")
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer devNull.Close()
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull
	if err := cmd.Start(); err != nil {
		return
	}
	_ = cmd.Process.Release()
}

// MaybeShowUpdateNotice prints an upgrade notice to stderr when the cached
// latest version is newer than the running version. It performs only a
// file read (no network) and stays silent when the cache is missing or
// unparseable, when no newer version is cached, or in quiet mode
// (ENV_SYNC_QUIET=true or -q/--quiet in args). It never fails.
func MaybeShowUpdateNotice(args []string) {
	defer func() {
		_ = recover()
	}()
	st, err := readCache()
	if err != nil {
		return
	}
	latest := strings.TrimSpace(st.LatestVersion)
	if latest == "" || !IsNewer(latest, config.Version) {
		return
	}
	if isQuiet(args) {
		return
	}
	fmt.Fprintf(os.Stderr, "A new version of env-sync is available: %s (you have %s)\n", latest, config.Version)
	fmt.Fprintln(os.Stderr, "Run `env-sync update` to upgrade.")
	fmt.Fprintln(os.Stderr, releaseTagURL(latest))
}

// RunCheckNow fetches the latest release from GitHub, refreshes the cache
// file, and returns any fetch error. On failure the previous latest_version
// (if any) is preserved while last_checked_at is still refreshed, so a
// failing endpoint does not cause a retry hammer loop. It is called by the
// hidden internal subcommand; the caller decides the exit code silently.
func RunCheckNow() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("update check panicked: %v", r)
		}
	}()
	now := time.Now().UTC().Format(time.RFC3339)
	latest, fetchErr := fetchLatestVersion()
	if fetchErr != nil {
		st, readErr := readCache()
		if readErr != nil {
			st = versionState{}
		}
		st.LastCheckedAt = now
		_ = writeCache(st)
		return fetchErr
	}
	if err := writeCache(versionState{LatestVersion: latest, LastCheckedAt: now}); err != nil {
		return err
	}
	return nil
}

// RunUpdate implements `env-sync update`.
//
// Supported flags: --check (print current vs latest), --yes/-y (skip the
// confirm prompt), --user/--gui/--gui-only/--all (passed through to the
// installer), --dry-run (print what would run), --help/-h (usage).
//
// By default it prompts for confirmation; when stdin is not a TTY,
// --yes is required. It downloads install.sh from the first reachable
// installer URL into a temp file and executes `bash <tmpfile> [passthrough
// args]` with inherited stdio. It returns the process exit code.
func RunUpdate(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "update failed:", r)
			code = 1
		}
	}()

	var (
		flagCheck   bool
		flagYes     bool
		flagDryRun  bool
		passthrough []string
	)
	for _, a := range args {
		switch a {
		case "--check":
			flagCheck = true
		case "--yes", "-y":
			flagYes = true
		case "--dry-run":
			flagDryRun = true
		case "--help", "-h":
			printUpdateUsage()
			return 0
		case "--user", "--gui", "--gui-only", "--all":
			passthrough = append(passthrough, a)
		}
	}

	if flagCheck {
		return runUpdateCheck()
	}

	latest := cachedLatest()
	want := "latest release"
	if latest != "" {
		want = latest
	}

	if flagDryRun {
		fmt.Println("Would download installer from one of:")
		for _, u := range installerURLs {
			fmt.Println("  " + u)
		}
		installerArgs := strings.Join(passthrough, " ")
		if installerArgs != "" {
			installerArgs = " " + installerArgs
		}
		fmt.Printf("Would run: bash <temp install.sh>%s\n", installerArgs)
		return 0
	}

	if !flagYes {
		if !isTerminal() {
			fmt.Fprintln(os.Stderr, "Not running interactively; re-run with --yes to update non-interactively.")
			return 1
		}
		fmt.Fprintf(os.Stderr, "Update env-sync to %s? [y/N]: ", want)
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			fmt.Fprintln(os.Stderr, "Update cancelled.")
			return 1
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Update cancelled.")
			return 1
		}
	}

	fmt.Fprintln(os.Stderr, "Downloading installer...")
	tmpFile, err := downloadInstaller()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Update failed:", err)
		return 1
	}
	defer os.RemoveAll(filepath.Dir(tmpFile))

	fmt.Fprintln(os.Stderr, "Running installer...")
	cmd := exec.Command("bash", append([]string{tmpFile}, passthrough...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "Update failed:", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "Update complete.")
	return 0
}

// runUpdateCheck implements `env-sync update --check`: a live fetch with a
// short timeout plus a "Current vs Latest" report. It always returns 0.
func runUpdateCheck() int {
	current := config.Version
	latest, err := fetchLatestVersion()
	if err != nil {
		if cached := cachedLatest(); cached != "" {
			fmt.Printf("Current: %s, Latest: %s (cached, live check failed: %v)\n", current, cached, err)
			if IsNewer(cached, current) {
				fmt.Println("Update available: run `env-sync update` to upgrade.")
			} else {
				fmt.Println("You are up to date.")
			}
			return 0
		}
		fmt.Printf("Current: %s, Latest: unknown (check failed: %v)\n", current, err)
		return 0
	}
	fmt.Printf("Current: %s, Latest: %s\n", current, latest)
	if IsNewer(latest, current) {
		fmt.Println("Update available: run `env-sync update` to upgrade.")
	} else {
		fmt.Println("You are up to date.")
	}
	return 0
}

// cachedLatest returns the cached latest version, or "" when absent.
func cachedLatest() string {
	st, err := readCache()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(st.LatestVersion)
}

// downloadInstaller fetches install.sh from the first reachable installer
// URL into a temp file and returns its path. The caller removes it.
func downloadInstaller() (string, error) {
	client := &http.Client{Timeout: httpTimeout}
	var lastErr error
	for _, u := range installerURLs {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "env-sync/"+config.Version)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s: unexpected status %s", u, resp.Status)
			continue
		}
		if len(body) == 0 {
			lastErr = fmt.Errorf("%s: empty installer", u)
			continue
		}
		dir, err := os.MkdirTemp("", "env-sync-update-*")
		if err != nil {
			return "", err
		}
		tmp := filepath.Join(dir, "install.sh")
		if err := os.WriteFile(tmp, body, 0o700); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		return tmp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no installer URLs configured")
	}
	return "", fmt.Errorf("failed to download installer: %w", lastErr)
}

// printUpdateUsage prints `env-sync update` help text.
func printUpdateUsage() {
	fmt.Println(`Usage: env-sync update [flags]

Update env-sync to the latest release.

Flags:
  --check      Show current vs latest version without updating
  --yes, -y    Skip the confirmation prompt
  --user       Install for the current user (passed to installer)
  --gui        Install GUI components (passed to installer)
  --gui-only   Install only GUI components (passed to installer)
  --all        Install all components (passed to installer)
  --dry-run    Print what would run without changing anything
  --help, -h   Show this help`)
}
