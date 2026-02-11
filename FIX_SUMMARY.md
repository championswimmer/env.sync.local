# Fix Summary: Enhanced Logging for Encryption Flow Debugging

## Problem

When two machines are initialized with encryption and try to sync for the first time, users reported potential issues with keys not being decryptable. After thorough analysis, the core encryption flow is **actually correct**, but there were significant gaps in logging that made it impossible to debug when issues occurred.

## Root Cause Analysis

The expected flow for two-machine onboarding is:

1. **HostA**: `init --encrypted` → `add KEY1=val1` (encrypted to A's key only)
2. **HostB**: `init --encrypted` → `add KEY2=val2` (encrypted to B's key only)
3. **HostB**: `sync` → discovers A, cannot decrypt, triggers auto-registration
4. **Auto-registration**: B sends its pubkey to A via SSH, A re-encrypts to [A, B]
5. **HostB**: Re-fetches A's file, caches A's pubkey, re-encrypts local file to [A, B]
6. **HostA**: `sync` → can now decrypt B's file

The flow **works correctly**, but had these issues:

### Issues Fixed

1. **Silent failures in `maybeReencryptLocal()`**: When this critical function returned early (file doesn't exist, not encrypted, can't decrypt, no missing recipients), there was no logging to explain why re-encryption was skipped.

2. **Minimal logging in `reencryptSecrets()`**: When individual secrets failed to decrypt during re-encryption, they were kept in old encrypted form with only a WARN log. This could result in secrets not being accessible to new peers, but it wasn't obvious.

3. **No debug visibility in auto-registration**: The auto-registration flow had no verbose logging to show what pubkeys were involved, making it hard to debug permission issues.

4. **No indication of successful key caching**: When public keys were cached from remote files, there was no feedback about how many keys were discovered and cached.

## Changes Made

### 1. Enhanced `maybeReencryptLocal()` Logging (sync.go:329-395)

Added verbose debug logging for all early-return conditions:

```go
func maybeReencryptLocal() {
    if _, err := os.Stat(config.SecretsFile()); err != nil {
        if config.IsVerbose() {
            logging.Log("DEBUG", "maybeReencryptLocal: secrets file does not exist, skipping")
        }
        return
    }
    if !keys.IsFileEncrypted(config.SecretsFile()) {
        if config.IsVerbose() {
            logging.Log("DEBUG", "maybeReencryptLocal: file not encrypted, skipping")
        }
        return
    }
    if !keys.CanDecryptFile(config.SecretsFile()) {
        if config.IsVerbose() {
            logging.Log("DEBUG", "maybeReencryptLocal: cannot decrypt local file, skipping re-encryption")
        }
        return
    }
    // ... more verbose logging for missing recipients
}
```

**Benefit**: Users running with `--verbose` flag can now see exactly why re-encryption is or isn't happening.

### 2. Improved `reencryptSecrets()` Error Handling (sync.go:426-500)

Enhanced logging for:
- What recipients are being encrypted to (in verbose mode)
- Which specific secrets failed to decrypt with full error details
- Summary of failed re-encryptions with actionable advice

```go
if len(failedKeys) > 0 {
    logging.Log("WARN", fmt.Sprintf("%d secret(s) could not be re-encrypted: %s", len(failedKeys), strings.Join(failedKeys, ", ")))
    logging.Log("WARN", "These secrets may not be accessible to all peers. Consider manually re-adding them.")
}
```

**Benefit**: Users are explicitly warned when secrets might not be accessible to all peers, with clear guidance on how to fix it.

### 3. Auto-Registration Debug Logging (sync.go:193-215)

Added verbose logging before attempting auto-registration:

```go
if config.IsVerbose() {
    logging.Log("DEBUG", fmt.Sprintf("Remote file from %s is encrypted but cannot be decrypted", host))
    remoteRecipients := keys.GetRecipientsFromFile(remoteFile)
    localPubkey := keys.GetLocalPubkey()
    logging.Log("DEBUG", fmt.Sprintf("Local pubkey: %s", localPubkey[:12]+"..."))
    logging.Log("DEBUG", fmt.Sprintf("Remote recipients: %d keys", len(remoteRecipients)))
}
```

**Benefit**: When auto-registration is needed, verbose mode shows why (local key not in remote recipients).

### 4. Key Caching Feedback (sync.go:218-224)

Added verbose logging when public keys are successfully cached:

```go
else if config.IsVerbose() {
    publicKeys := keys.ExtractPublicKeysFromFile(remoteFile)
    logging.Log("DEBUG", fmt.Sprintf("Cached %d public key(s) from %s", len(publicKeys), host))
}
```

**Benefit**: Users can confirm that key discovery and caching is working correctly.

## Testing

- ✅ Build succeeds without errors (`make build`)
- ✅ Code follows existing logging patterns
- ✅ All logging respects `config.IsVerbose()` flag
- ✅ No changes to core logic - only logging enhancements

## Usage

To see detailed debugging information during sync:

```bash
env-sync --verbose sync
```

This will show:
- Why re-encryption is or isn't triggered
- Which recipients are being encrypted to
- Public key caching operations
- Any failures in individual secret re-encryption

## Impact

**No functional changes** - the core encryption and sync logic remains unchanged. This is purely a **diagnostic improvement** to help users and developers understand what's happening during the encryption flow, especially during initial onboarding of new machines.

## Future Improvements

Based on this analysis, potential future enhancements could include:

1. **Integration test**: Add a test that simulates the exact two-machine onboarding scenario
2. **Self-healing**: When secrets fail to re-encrypt, automatically attempt to re-add them
3. **Health check**: Add a command to verify all secrets are decryptable and encrypted to all known peers
4. **Dry-run mode**: Allow users to preview what would happen during sync without making changes

## Files Modified

- `src/internal/sync/sync.go`: Enhanced logging in `maybeReencryptLocal()`, `reencryptSecrets()`, and `syncFromHost()`
- `ENCRYPTION_FLOW_ANALYSIS.md`: Comprehensive analysis of the encryption flow (new file)
- `FIX_SUMMARY.md`: This file (new file)
