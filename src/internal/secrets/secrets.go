package secrets

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"envsync/internal/config"
	"envsync/internal/logging"
	"envsync/internal/metadata"
)

var (
	headerStart          = "# === ENV_SYNC_METADATA ==="
	headerEnd            = "# === END_METADATA ==="
	footerStart          = "# === ENV_SYNC_FOOTER ==="
	footerEnd            = "# === END_FOOTER ==="
	lineTimestampPattern = regexp.MustCompile(`ENVSYNC_UPDATED_AT=([0-9TZ:.-]+)`)
	lineKeyPattern       = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=`)
)

func InitSecretsFile(file string, initTimestamp string) error {
	if initTimestamp == "" {
		initTimestamp = config.InitTimestamp()
	}
	hostname := GetHostname()
	content := fmt.Sprintf(`# === ENV_SYNC_METADATA ===
# VERSION: %s
# TIMESTAMP: %s
# HOST: %s
# MODIFIED: %s
# CHECKSUM: 
# === END_METADATA ===

# Add your secrets below this line
# Example:
# OPENAI_API_KEY="sk-..."

# === ENV_SYNC_FOOTER ===
# VERSION: %s
# TIMESTAMP: %s
# HOST: %s
# === END_FOOTER ===
`, config.Version, initTimestamp, hostname, initTimestamp, config.Version, initTimestamp, hostname)

	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		return err
	}
	if err := metadata.UpdateChecksum(file); err != nil {
		return err
	}
	logging.Log("SUCCESS", "Initialized secrets file: "+file)
	return nil
}

func UpdateMetadata(file string, newVersion string) error {
	if _, err := os.Stat(file); err != nil {
		logging.Log("ERROR", "File not found: "+file)
		return err
	}

	hostname := GetHostname()
	timestamp := GetTimestamp()
	currentVersion := metadata.GetFileVersion(file)
	version := currentVersion
	if newVersion != "" {
		version = newVersion
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "# TIMESTAMP:"):
			lines[i] = "# TIMESTAMP: " + timestamp
		case strings.HasPrefix(line, "# HOST:"):
			lines[i] = "# HOST: " + hostname
		case strings.HasPrefix(line, "# MODIFIED:"):
			lines[i] = "# MODIFIED: " + timestamp
		case strings.HasPrefix(line, footerStart):
			// no-op
		case strings.HasPrefix(line, "# VERSION:"):
			// Update footer version and timestamp; header version should remain as is
			if inFooter(lines, i) {
				lines[i] = "# VERSION: " + version
			}
		}
	}

	updated := strings.Join(lines, "\n")
	if err := os.WriteFile(file, []byte(updated), 0o600); err != nil {
		return err
	}

	return metadata.UpdateChecksum(file)
}

func ValidateSecretsFile(file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		logging.Log("ERROR", "Secrets file not found: "+file)
		return err
	}

	text := string(content)
	if !strings.Contains(text, headerStart) {
		logging.Log("ERROR", "Invalid secrets file: missing metadata header")
		return errors.New("missing metadata header")
	}
	if !strings.Contains(text, footerStart) {
		logging.Log("ERROR", "Invalid secrets file: missing metadata footer")
		return errors.New("missing metadata footer")
	}

	stored := metadata.GetFileChecksum(file)
	if stored != "" {
		current, err := metadata.GenerateChecksum(file)
		if err != nil {
			return err
		}
		if stored != current {
			logging.Log("WARN", "Checksum mismatch - file may be corrupted")
			return errors.New("checksum mismatch")
		}
	}

	return nil
}

func GetSecretsContent(file string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	text := string(content)
	if strings.Contains(text, headerEnd) {
		re := regexp.MustCompile(`(?s)^.*?` + regexp.QuoteMeta(headerEnd) + `\n`)
		text = re.ReplaceAllString(text, "")
		if idx := strings.Index(text, footerStart); idx >= 0 {
			text = text[:idx]
		}
		return strings.TrimSuffix(text, "\n"), nil
	}

	return strings.TrimSuffix(text, "\n"), nil
}

func SetSecretsContent(file string, content string) error {
	if !fileHasMetadata(file) {
		if err := InitSecretsFile(file, ""); err != nil {
			return err
		}
	}

	header, footer, err := getHeaderFooter(file)
	if err != nil {
		return err
	}

	builder := strings.Builder{}
	builder.WriteString(header)
	builder.WriteString("\n\n")
	builder.WriteString(strings.TrimSuffix(content, "\n"))
	builder.WriteString("\n\n")
	builder.WriteString(footer)
	builder.WriteString("\n")

	if err := os.WriteFile(file, []byte(builder.String()), 0o600); err != nil {
		return err
	}

	return UpdateMetadata(file, "")
}

func fileHasMetadata(file string) bool {
	content, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), headerStart)
}

func getHeaderFooter(file string) (string, string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(content), "\n")
	var headerLines []string
	var footerLines []string
	inHeader := false
	inFooter := false

	for _, line := range lines {
		if strings.HasPrefix(line, headerStart) {
			inHeader = true
		}
		if inHeader {
			headerLines = append(headerLines, line)
		}
		if strings.HasPrefix(line, headerEnd) {
			inHeader = false
		}
		if strings.HasPrefix(line, footerStart) {
			inFooter = true
		}
		if inFooter {
			footerLines = append(footerLines, line)
		}
		if strings.HasPrefix(line, footerEnd) {
			inFooter = false
		}
	}

	if len(headerLines) == 0 || len(footerLines) == 0 {
		return "", "", errors.New("missing metadata")
	}

	return strings.Join(headerLines, "\n"), strings.Join(footerLines, "\n"), nil
}

func inFooter(lines []string, index int) bool {
	inFooter := false
	for i := 0; i <= index && i < len(lines); i++ {
		if strings.HasPrefix(lines[i], footerStart) {
			inFooter = true
		}
		if strings.HasPrefix(lines[i], footerEnd) {
			inFooter = false
		}
	}
	return inFooter
}

func CompareVersions(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}
	s1 := strings.Split(v1, ".")
	s2 := strings.Split(v2, ".")
	length := len(s1)
	if len(s2) > length {
		length = len(s2)
	}
	for i := 0; i < length; i++ {
		var a, b int
		if i < len(s1) {
			fmt.Sscanf(s1[i], "%d", &a)
		}
		if i < len(s2) {
			fmt.Sscanf(s2[i], "%d", &b)
		}
		if a > b {
			return 1
		}
		if a < b {
			return -1
		}
	}
	return 0
}

func CompareTimestamps(t1, t2 string) int {
	if t1 == t2 {
		return 0
	}
	time1, err1 := parseTimestamp(t1)
	time2, err2 := parseTimestamp(t2)
	if err1 != nil {
		time1 = 0
	}
	if err2 != nil {
		time2 = 0
	}
	if time1 > time2 {
		return 1
	}
	if time1 < time2 {
		return -1
	}
	return 0
}

func IsNewer(file1, file2 string) bool {
	v1 := metadata.GetFileVersion(file1)
	v2 := metadata.GetFileVersion(file2)
	t1 := metadata.GetFileTimestamp(file1)
	t2 := metadata.GetFileTimestamp(file2)
	h1 := metadata.GetFileHost(file1)
	h2 := metadata.GetFileHost(file2)

	if t1 != "" && t2 != "" {
		ts := CompareTimestamps(t1, t2)
		if ts == 1 {
			return true
		}
		if ts == -1 {
			return false
		}
	}

	if v1 != "" && v2 != "" {
		vs := CompareVersions(v1, v2)
		if vs == 1 {
			return true
		}
		if vs == -1 {
			return false
		}
	}

	return h1 < h2
}

func MergeSecretsContent(localContent string, remoteContent string) string {
	lines := map[string]string{}
	timestamps := map[string]string{}

	applyLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			return
		}
		key := getLineKey(line)
		if key == "" {
			return
		}
		lines[key] = line
		timestamps[key] = getLineTimestamp(line)
	}

	for _, line := range strings.Split(localContent, "\n") {
		applyLine(line)
	}

	for _, line := range strings.Split(remoteContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key := getLineKey(line)
		if key == "" {
			continue
		}
		remoteTS := getLineTimestamp(line)
		localTS := timestamps[key]
		if localTS == "" {
			lines[key] = line
			timestamps[key] = remoteTS
			continue
		}
		if CompareTimestamps(remoteTS, localTS) == 1 {
			lines[key] = line
			timestamps[key] = remoteTS
		}
	}

	keys := make([]string, 0, len(lines))
	for key := range lines {
		keys = append(keys, key)
	}
	sortStrings(keys)

	output := make([]string, 0, len(keys))
	for _, key := range keys {
		output = append(output, lines[key])
	}
	return strings.Join(output, "\n")
}

func getLineTimestamp(line string) string {
	match := lineTimestampPattern.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func getLineKey(line string) string {
	match := lineKeyPattern.FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func GetHostname() string {
	// ENV_SYNC_HOSTNAME overrides the machine hostname (useful for tests
	// and for simulating multiple machines on one host).
	if override := strings.TrimSpace(os.Getenv("ENV_SYNC_HOSTNAME")); override != "" {
		return override
	}
	if output, err := execCommand("hostname", "-f"); err == nil {
		host := strings.TrimSpace(output)
		if host != "" {
			return host
		}
	}
	if host, err := os.Hostname(); err == nil {
		return host
	}
	return "unknown"
}

func GetTimestamp() string {
	return timeNowUTC().Format("2006-01-02T15:04:05Z")
}

func parseTimestamp(ts string) (int64, error) {
	t, err := timeParseUTC(ts)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

func timeParseUTC(value string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05Z", value)
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func execCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	output, err := cmd.Output()
	return string(output), err
}

func sortStrings(values []string) {
	sort.Strings(values)
}
