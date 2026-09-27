package peer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"envsync/internal/config"
	"envsync/internal/logging"
	"envsync/internal/secrets"
)

// MembershipEvent represents a signed event in the append-only membership log.
type MembershipEvent struct {
	EventID              uint64 `json:"event_id"`
	Action               string `json:"action"` // "approve", "revoke"
	PeerID               string `json:"peer_id"`
	Hostname             string `json:"hostname"`
	TransportFingerprint string `json:"transport_fingerprint"`
	AGEPubkey            string `json:"age_pubkey"`
	SponsorID            string `json:"sponsor_id"`
	Timestamp            string `json:"timestamp"`
	ExpiresAt            string `json:"expires_at"`
	SignatureR           string `json:"signature_r"`
	SignatureS           string `json:"signature_s"`
}

// MembershipLog holds the append-only list of membership events.
type MembershipLog struct {
	Events    []MembershipEvent `json:"events"`
	LastEvent uint64            `json:"last_event"`
}

// LoadMembershipLog reads the membership event log from disk.
func LoadMembershipLog() (*MembershipLog, error) {
	data, err := os.ReadFile(config.MembershipEventsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return &MembershipLog{}, nil
		}
		return nil, err
	}
	var log MembershipLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("corrupt membership log: %w", err)
	}
	return &log, nil
}

// SaveMembershipLog writes the membership event log to disk.
func SaveMembershipLog(log *MembershipLog) error {
	if err := os.MkdirAll(config.PeerRegistryDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.MembershipEventsFile(), data, 0o600)
}

// CreateApproveEvent creates a signed membership approval event.
func CreateApproveEvent(log *MembershipLog, p Peer, sponsorID string, sponsorKey *ecdsa.PrivateKey, eventExpiry time.Duration) (*MembershipEvent, error) {
	return createEvent(log, "approve", p, sponsorID, sponsorKey, eventExpiry)
}

// CreateRevokeEvent creates a signed membership revocation event.
func CreateRevokeEvent(log *MembershipLog, p Peer, sponsorID string, sponsorKey *ecdsa.PrivateKey, eventExpiry time.Duration) (*MembershipEvent, error) {
	return createEvent(log, "revoke", p, sponsorID, sponsorKey, eventExpiry)
}

func createEvent(log *MembershipLog, action string, p Peer, sponsorID string, sponsorKey *ecdsa.PrivateKey, eventExpiry time.Duration) (*MembershipEvent, error) {
	eventID := log.LastEvent + 1

	event := &MembershipEvent{
		EventID:              eventID,
		Action:               action,
		PeerID:               p.ID,
		Hostname:             p.Hostname,
		TransportFingerprint: p.TransportFingerprint,
		AGEPubkey:            p.AGEPubkey,
		SponsorID:            sponsorID,
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:            time.Now().UTC().Add(eventExpiry).Format(time.RFC3339),
	}

	if err := signEvent(event, sponsorKey); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return event, nil
}

// AppendEvent appends a verified event to the log.
func AppendEvent(log *MembershipLog, event *MembershipEvent) error {
	if event.EventID <= log.LastEvent {
		return fmt.Errorf("event %d is stale (last=%d)", event.EventID, log.LastEvent)
	}
	log.Events = append(log.Events, *event)
	log.LastEvent = event.EventID
	return nil
}

// signEvent signs the event digest with the sponsor's ECDSA key.
func signEvent(event *MembershipEvent, key *ecdsa.PrivateKey) error {
	digest := eventDigest(event)
	r, s, err := ecdsa.Sign(rand.Reader, key, digest)
	if err != nil {
		return err
	}
	event.SignatureR = r.Text(16)
	event.SignatureS = s.Text(16)
	return nil
}

// VerifyEvent verifies the event's signature against the sponsor's public key.
func VerifyEvent(event *MembershipEvent, pubkey *ecdsa.PublicKey) bool {
	digest := eventDigest(event)
	r := new(big.Int)
	s := new(big.Int)
	r.SetString(event.SignatureR, 16)
	s.SetString(event.SignatureS, 16)
	return ecdsa.Verify(pubkey, digest, r, s)
}

// IsEventExpired checks if an event has passed its expiry.
func IsEventExpired(event *MembershipEvent) bool {
	expiry, err := time.Parse(time.RFC3339, event.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(expiry)
}

// eventDigest computes a SHA-256 digest of the event's signable fields.
func eventDigest(event *MembershipEvent) []byte {
	data := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s|%s",
		event.EventID, event.Action, event.PeerID, event.Hostname,
		event.TransportFingerprint, event.AGEPubkey,
		event.SponsorID, event.Timestamp)
	hash := sha256.Sum256([]byte(data))
	return hash[:]
}

// ApplyEvents reconciles membership events into the peer registry.
// Events are applied in order: approve adds/updates peers, revoke revokes them.
// When we receive an approval event for ourselves, we also add the sponsor (approver) to our registry.
func ApplyEvents(reg *Registry, log *MembershipLog, lastApplied uint64) (uint64, error) {
	// Per-user peer ID: two OS users on one machine have distinct IDs.
	hostname := secrets.LocalPeerID()

	for _, event := range log.Events {
		if event.EventID <= lastApplied {
			continue
		}

		switch event.Action {
		case "approve":
			existing, err := reg.GetPeer(event.PeerID)
			if err != nil {
				// New peer — add and approve
				p := Peer{
					ID:                   event.PeerID,
					Hostname:             event.Hostname,
					TransportFingerprint: event.TransportFingerprint,
					AGEPubkey:            event.AGEPubkey,
					State:                StateApproved,
					Capabilities:         []Capability{CapRead, CapRequestReencrypt},
				}
				if err := reg.AddPeer(p); err != nil {
					logging.Log("WARN", fmt.Sprintf("Failed to add peer from event: %v", err))
					continue
				}
				// Set state directly since AddPeer sets to pending
				_ = reg.UpdatePeer(event.PeerID, func(peer *Peer) {
					peer.State = StateApproved
					peer.Capabilities = []Capability{CapRead, CapRequestReencrypt}
				})
			} else if existing.State != StateApproved {
				_ = reg.ApprovePeer(event.PeerID)
			}
			logging.Log("INFO", fmt.Sprintf("Applied approval event for %s (sponsor: %s)", event.Hostname, event.SponsorID))

			// If this event is for ourselves, also add the sponsor (approver) to our registry
			if event.PeerID == hostname && event.SponsorID != hostname {
				// Get sponsor's info from the event - we need to fetch their cert and AGE key
				// For now, just add them as approved with empty info - they'll update on next sync
				_, err := reg.GetPeer(event.SponsorID)
				if err != nil {
					sponsorPeer := Peer{
						ID:           event.SponsorID,
						Hostname:     event.SponsorID,
						State:        StateApproved,
						Capabilities: []Capability{CapRead, CapRequestReencrypt},
					}
					_ = reg.AddPeer(sponsorPeer)
					logging.Log("INFO", fmt.Sprintf("Added sponsor %s to registry", event.SponsorID))
				}
			}

		case "revoke":
			if _, err := reg.GetPeer(event.PeerID); err == nil {
				_ = reg.RevokePeer(event.PeerID)
				logging.Log("INFO", fmt.Sprintf("Applied revocation event for %s", event.Hostname))
			}
		}

		lastApplied = event.EventID
	}
	return lastApplied, nil
}

// GetEventsSince returns events after the given event ID (for catch-up).
func GetEventsSince(log *MembershipLog, sinceEventID uint64) []MembershipEvent {
	events := make([]MembershipEvent, 0)
	for _, event := range log.Events {
		if event.EventID > sinceEventID {
			events = append(events, event)
		}
	}
	return events
}

// GenerateSigningKey creates a new ECDSA P-256 key for signing events.
// In production this reuses the transport identity key.
func GenerateSigningKey() (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.New("failed to generate signing key")
	}
	return key, nil
}
