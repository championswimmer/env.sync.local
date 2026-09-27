package discovery

import (
	"testing"

	sshtransport "envsync/internal/transport/ssh"
)

func TestParseDnssdPeersAcceptsTrailingDotService(t *testing.T) {
	t.Setenv("ENV_SYNC_HOSTNAME", "ci-test-host")
	t.Setenv("ENV_SYNC_USER", "ci-test-user")
	sample := `
DATE: ---Tue 10 Feb 2026---
21:12:46.949  ...STARTING...
Timestamp     A/R    Flags  if Domain               Service Type         Instance Name
21:12:46.949  Add        3  14 local.               _envsync._tcp.       beelink
21:12:46.949  Add        3   1 local.               _envsync._tcp.       mbp16
21:12:46.949  Add        2  14 local.               _envsync._tcp.       mbp16
`

	peers := parseDnssdPeers(sample)
	expected := []string{"beelink.local", "mbp16.local"}

	if len(peers) != len(expected) {
		t.Fatalf("expected %d peers, got %d (%v)", len(expected), len(peers), peers)
	}

	for i, peer := range expected {
		if peers[i] != peer {
			t.Fatalf("peer %d mismatch: expected %s, got %s", i, peer, peers[i])
		}
	}
}

func TestParseDnssdPeersFiltersSelfHostname(t *testing.T) {
	// Pin a fake self identity so the test is hermetic on any machine.
	t.Setenv("ENV_SYNC_HOSTNAME", "ci-self-host")
	t.Setenv("ENV_SYNC_USER", "ci-self-user")

	// Get current hostname to use in test
	selfHostname := sshtransport.Hostname()

	// Extract just the hostname part without .local suffix for instance name
	instanceName := selfHostname
	if len(instanceName) > 6 && instanceName[len(instanceName)-6:] == ".local" {
		instanceName = instanceName[:len(instanceName)-6]
	}

	sample := `
DATE: ---Tue 10 Feb 2026---
21:12:46.949  ...STARTING...
Timestamp     A/R    Flags  if Domain               Service Type         Instance Name
21:12:46.949  Add        3  14 local.               _envsync._tcp.       beelink
21:12:46.949  Add        3   1 local.               _envsync._tcp.       ` + instanceName + `
21:12:46.949  Add        2  14 local.               _envsync._tcp.       mbp16
`

	peers := parseDnssdPeers(sample)

	// Should have beelink.local and mbp16.local, but NOT selfHostname
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers (excluding self), got %d (%v)", len(peers), peers)
	}

	for _, peer := range peers {
		if peer == selfHostname {
			t.Fatalf("self hostname %s should have been filtered out", selfHostname)
		}
	}

	if peers[0] != "beelink.local" {
		t.Fatalf("expected first peer to be beelink.local, got %s", peers[0])
	}
	if peers[1] != "mbp16.local" {
		t.Fatalf("expected second peer to be mbp16.local, got %s", peers[1])
	}
}

func TestParseDnssdPeersFiltersMultipleSelfReferences(t *testing.T) {
	t.Setenv("ENV_SYNC_HOSTNAME", "ci-self-host")
	t.Setenv("ENV_SYNC_USER", "ci-self-user")
	selfHostname := sshtransport.Hostname()

	// Extract just the hostname part without .local suffix for instance name
	instanceName := selfHostname
	if len(instanceName) > 6 && instanceName[len(instanceName)-6:] == ".local" {
		instanceName = instanceName[:len(instanceName)-6]
	}

	// Include self hostname multiple times (different interfaces)
	sample := `
DATE: ---Tue 10 Feb 2026---
21:12:46.949  ...STARTING...
Timestamp     A/R    Flags  if Domain               Service Type         Instance Name
21:12:46.949  Add        3  14 local.               _envsync._tcp.       ` + instanceName + `
21:12:46.949  Add        3   1 local.               _envsync._tcp.       ` + instanceName + `
21:12:46.949  Add        2  14 local.               _envsync._tcp.       beelink
`

	peers := parseDnssdPeers(sample)

	// Should only have beelink.local, all self references filtered
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer (excluding all self references), got %d (%v)", len(peers), peers)
	}

	if peers[0] != "beelink.local" {
		t.Fatalf("expected peer to be beelink.local, got %s", peers[0])
	}
}
