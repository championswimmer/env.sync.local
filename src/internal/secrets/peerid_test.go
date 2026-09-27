package secrets

import (
	"testing"
)

func TestParseEndpointUserAtHost(t *testing.T) {
	ep := ParseEndpoint("alice@beelink.local:5740")
	if ep.User != "alice" {
		t.Fatalf("user = %q, want alice", ep.User)
	}
	if ep.Host != "beelink.local" {
		t.Fatalf("host = %q, want beelink.local", ep.Host)
	}
	if ep.Port != "5740" {
		t.Fatalf("port = %q, want 5740", ep.Port)
	}
	if ep.PeerID != "alice@beelink" {
		t.Fatalf("peerID = %q, want alice@beelink", ep.PeerID)
	}
	if ep.Legacy {
		t.Fatal("should not be legacy")
	}
	if got := ep.SSHDial(); got != "alice@beelink.local" {
		t.Fatalf("ssh dial = %q", got)
	}
	if got := ep.Dial(""); got != "beelink.local:5740" {
		t.Fatalf("dial = %q", got)
	}
}

func TestParseEndpointLegacy(t *testing.T) {
	ep := ParseEndpoint("beelink.local")
	if !ep.Legacy {
		t.Fatal("should be legacy")
	}
	if ep.PeerID != "beelink" {
		t.Fatalf("peerID = %q, want beelink", ep.PeerID)
	}
	if ep.Host != "beelink.local" {
		t.Fatalf("host = %q", ep.Host)
	}
}

func TestParseEndpointIP(t *testing.T) {
	ep := ParseEndpoint("127.0.0.1:5739")
	if ep.Host != "127.0.0.1" || ep.Port != "5739" {
		t.Fatalf("bad IP parse: %+v", ep)
	}
	if got := ep.Dial(""); got != "127.0.0.1:5739" {
		t.Fatalf("dial = %q", got)
	}
	if got := ShortHost("127.0.0.1"); got != "127.0.0.1" {
		t.Fatalf("ShortHost IP = %q", got)
	}
	if got := DialHost("127.0.0.1"); got != "127.0.0.1" {
		t.Fatalf("DialHost IP = %q", got)
	}
}

func TestShortHostStripsLocal(t *testing.T) {
	if got := ShortHost("beelink.local"); got != "beelink" {
		t.Fatalf("got %q", got)
	}
	if got := ShortHost("beelink"); got != "beelink" {
		t.Fatalf("got %q", got)
	}
	if got := ShortHost("localhost"); got != "localhost" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizePeerID(t *testing.T) {
	if got := NormalizePeerID("alice", "beelink.local"); got != "alice@beelink" {
		t.Fatalf("got %q", got)
	}
}

func TestLocalPeerIDWithOverrides(t *testing.T) {
	t.Setenv("ENV_SYNC_USER", "alice")
	t.Setenv("ENV_SYNC_HOSTNAME", "beelink.local")
	if got := LocalPeerID(); got != "alice@beelink" {
		t.Fatalf("got %q", got)
	}
	if got := LocalInstanceName(); got != "alice@beelink" {
		t.Fatalf("got %q", got)
	}
}

func TestIsSelf(t *testing.T) {
	t.Setenv("ENV_SYNC_USER", "alice")
	t.Setenv("ENV_SYNC_HOSTNAME", "beelink.local")
	if !ParseEndpoint("alice@beelink").IsSelf() {
		t.Fatal("alice@beelink should be self")
	}
	if !ParseEndpoint("alice@beelink.local:5740").IsSelf() {
		t.Fatal("alice@beelink.local:5740 should be self")
	}
	if ParseEndpoint("bob@beelink").IsSelf() {
		t.Fatal("bob@beelink should NOT be self")
	}
	// Legacy bare hostname matching own host counts as self (can't
	// disambiguate users from a bare name).
	if !ParseEndpoint("beelink.local").IsSelf() {
		t.Fatal("beelink.local should be self (legacy)")
	}
}
