package secrets

import (
	"net"
	"os"
	"os/user"
	"strings"
)

// GetUsername returns the current OS user login name.
// ENV_SYNC_USER overrides the OS user (useful for tests and for
// simulating multiple users on one machine).
func GetUsername() string {
	if override := strings.TrimSpace(os.Getenv("ENV_SYNC_USER")); override != "" {
		if clean := SanitizeInstancePart(override); clean != "" {
			return clean
		}
	}
	if u, err := user.Current(); err == nil && u != nil && u.Username != "" {
		// On macOS/Linux this may be "arnav"; on some systems "DOMAIN\\user".
		// Keep the part after the last \ or / for mDNS safety.
		name := u.Username
		if i := strings.LastIndex(name, "\\"); i >= 0 {
			name = name[i+1:]
		}
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		name = strings.TrimSpace(name)
		if name != "" {
			return name
		}
	}
	for _, env := range []string{"USER", "LOGNAME"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return "unknown"
}

// ShortHost strips .local suffix and any domain, e.g. "beelink.local" -> "beelink".
// IP addresses and "localhost" are returned unchanged (never truncated).
func ShortHost(host string) string {
	h := strings.TrimSpace(host)
	h = strings.TrimSuffix(h, ".")
	if h == "" {
		return "unknown"
	}
	if h == "localhost" || net.ParseIP(h) != nil {
		return h
	}
	h = strings.TrimSuffix(h, ".local")
	if i := strings.Index(h, "."); i >= 0 {
		h = h[:i]
	}
	if h == "" {
		return "unknown"
	}
	return h
}

// DialHost returns a dialable mDNS name for a short host, e.g. "beelink" -> "beelink.local".
// "localhost", IP addresses, and anything already qualified are returned as-is.
func DialHost(host string) string {
	h := strings.TrimSpace(host)
	h = strings.TrimSuffix(h, ".")
	if h == "" {
		return "unknown.local"
	}
	if h == "localhost" {
		return h
	}
	if strings.Contains(h, ":") {
		return h
	}
	if net.ParseIP(h) != nil {
		return h
	}
	if strings.Contains(h, ".") {
		return h
	}
	return h + ".local"
}

// LocalPeerID returns the canonical per-user peer ID: "user@shorthost".
// This is stable per (OS user, machine) and is used as the mDNS instance
// name, TLS certificate CN, and peer registry ID.
func LocalPeerID() string {
	return NormalizePeerID(GetUsername(), GetHostname())
}

// NormalizePeerID builds "user@shorthost" from a username and any hostname form.
func NormalizePeerID(username, hostname string) string {
	u := SanitizeInstancePart(strings.TrimSpace(username))
	if u == "" {
		u = "unknown"
	}
	return u + "@" + ShortHost(hostname)
}

// LocalInstanceName returns the mDNS instance name for this user on this machine.
func LocalInstanceName() string {
	return SanitizeInstanceName(LocalPeerID())
}

// SanitizeInstancePart sanitizes a single user/host part for use in a peer ID.
func SanitizeInstancePart(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-._")
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// SanitizeInstanceName sanitizes a full "user@host" instance name.
// The "@" separator is preserved; everything else is restricted to
// mDNS-safe characters.
func SanitizeInstanceName(s string) string {
	s = strings.TrimSpace(s)
	if at := strings.Index(s, "@"); at >= 0 {
		return SanitizeInstancePart(s[:at]) + "@" + SanitizeInstancePart(s[at+1:])
	}
	return SanitizeInstancePart(s)
}

// Endpoint is a parsed peer address. It accepts:
//   - "beelink.local" (legacy)
//   - "beelink"
//   - "arnav@beelink", "arnav@beelink.local"
//   - any of the above with ":port" suffix
type Endpoint struct {
	Raw    string
	PeerID string // "user@shorthost" or legacy short host
	User   string // "" for legacy peers
	Host   string // dialable DNS name, e.g. "beelink.local"
	Port   string // "" means default port
	Legacy bool
}

// ParseEndpoint parses a user-supplied or discovered peer address.
func ParseEndpoint(s string) Endpoint {
	raw := strings.TrimSpace(s)
	e := Endpoint{Raw: raw}
	if raw == "" {
		return e
	}

	rest := raw
	// Split off :port (last colon; ignore bracketed IPv6 which we don't support).
	if i := strings.LastIndex(rest, ":"); i >= 0 && !strings.Contains(rest[i:], "@") {
		maybePort := rest[i+1:]
		if maybePort != "" && isDigits(maybePort) {
			e.Port = maybePort
			rest = rest[:i]
		}
	}

	rest = strings.TrimSuffix(rest, ".")
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		e.User = SanitizeInstancePart(rest[:at])
		hostPart := rest[at+1:]
		e.Host = DialHost(ShortHost(hostPart))
		e.PeerID = e.User + "@" + ShortHost(hostPart)
		e.Legacy = false
		return e
	}

	// Legacy: bare hostname.
	e.Host = DialHost(ShortHost(rest))
	e.PeerID = ShortHost(rest)
	e.User = ""
	e.Legacy = true
	return e
}

// SSHDial returns the ssh/scp dial string, e.g. "arnav@beelink.local".
func (e Endpoint) SSHDial() string {
	if e.User != "" {
		return e.User + "@" + e.Host
	}
	return e.Host
}

// Dial returns the host[:port] dial string for HTTP/mTLS.
func (e Endpoint) Dial(fallbackPort string) string {
	port := e.Port
	if port == "" {
		port = fallbackPort
	}
	if port != "" {
		return e.Host + ":" + port
	}
	return e.Host
}

// SplitDial splits a "host[:port]" or "user@host[:port]" dial string into
// its connect host and port parts. The user part (if any) is stripped:
// it is only meaningful for SSH, not HTTP/mTLS dialing.
func SplitDial(dial string) (host, port string) {
	ep := ParseEndpoint(dial)
	return ep.Host, ep.Port
}

// ResolvePort fills in e.Port via discovery when the endpoint did not
// specify one explicitly. resolve maps Endpoint -> port.
func (e Endpoint) ResolvePort(resolve func(Endpoint) string) Endpoint {
	if e.Port != "" || resolve == nil {
		return e
	}
	if port := resolve(e); port != "" {
		e.Port = port
	}
	return e
}

func (e Endpoint) IsSelf() bool {
	if e.PeerID == "" {
		return false
	}
	if e.Legacy {
		// Legacy bare hostnames can't distinguish users on the same machine;
		// only treat as self when the host matches AND no local disambiguation
		// is possible. Callers using structured Peers should compare PeerID.
		return strings.EqualFold(e.Host, DialHost(ShortHost(GetHostname())))
	}
	return e.PeerID == LocalPeerID()
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
