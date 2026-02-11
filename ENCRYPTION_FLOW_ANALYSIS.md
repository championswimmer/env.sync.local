# Encryption Flow Analysis: Two-Machine Onboarding Issue

## Problem Statement

When two machines (hostA and hostB) are both initialized with encryption and each adds secrets before syncing, the secrets from one machine may not be decryptable by the other after sync.

## Root Cause

The issue is a **mismatch between file-level PUBLIC_KEYS metadata and individual secret encryption recipients**.

### Detailed Flow Analysis

#### Initial State

**HostA:**
```bash
env-sync init --encrypted
# Creates: ENCRYPTED: true, PUBLIC_KEYS: A:age1aaa...
# File is plaintext (no secrets yet)

env-sync add KEY1=val1
# Encrypts KEY1 to recipients from PUBLIC_KEYS: [age1aaa...]
# Result: KEY1="age_encrypted_blob_for_A_only"
```

**HostB:**
```bash
env-sync init --encrypted
# Creates: ENCRYPTED: true, PUBLIC_KEYS: B:age1bbb...

env-sync add KEY2=val2
# Encrypts KEY2 to recipients from PUBLIC_KEYS: [age1bbb...]
# Result: KEY2="age_encrypted_blob_for_B_only"
```

#### First Sync: HostB → HostA

**Step 1: HostB runs `env-sync`**
- Discovers hostA via mDNS
- Fetches hostA's file via SCP
- File has: `PUBLIC_KEYS: A:age1aaa...` and `KEY1="encrypted_to_A_only"`

**Step 2: CanDecryptFile Check (sync.go:193)**
- `IsFileEncrypted()` = true (has `ENCRYPTED: true`)
- `GetRecipientsFromFile()` extracts from PUBLIC_KEYS = `[age1aaa...]`
- HostB's key is `age1bbb...`
- **CanDecryptFile() = FALSE** ❌

**Step 3: Auto-Registration (sync.go:195-207)**
- SSH to hostA: `mkdir -p keys/known_hosts && printf age1bbb... > known_hosts/B.pub && env-sync`
- On hostA, `env-sync` runs → `maybeReencryptLocal()` executes
- `GetAllKnownRecipients()` = `[age1aaa..., age1bbb...]` (includes newly cached B)
- Detects age1bbb is missing from file's PUBLIC_KEYS
- **Re-encrypts KEY1 to [age1aaa..., age1bbb...]** ✅
- Updates PUBLIC_KEYS to `A:age1aaa...,B:age1bbb...`

**Step 4: HostB Re-fetches and Merges (sync.go:199-261)**
- Re-fetches hostA's file (now has KEY1 encrypted to both)
- Caches A's pubkey from PUBLIC_KEYS (line 212)
- **Calls `maybeReencryptLocal()`** (line 218)
  - `GetAllKnownRecipients()` = `[age1bbb..., age1aaa...]`
  - Checks if file needs re-encryption...
  - **Re-encrypts local file** (hostB's file)
  - **Updates PUBLIC_KEYS to `B:age1bbb...,A:age1aaa...`**
- Merges content (line 249-251):
  - Local: `KEY2="encrypted_to_B_only"`
  - Remote: `KEY1="encrypted_to_both"`
  - **Merged: Both keys, but KEY2 still encrypted to B only!**
- Writes merged content (line 253)
- Refreshes PUBLIC_KEYS metadata (line 256)

**THE BUG**: After `maybeReencryptLocal()` on line 218, the re-encryption happens. But then at line 251, we merge with remote content. The problem is:

1. `maybeReencryptLocal()` at line 218 re-encrypts the current file
2. But at line 249-250, we read `localContent` and `remoteContent` **fresh from files**
3. The `localContent` read at line 249 might be from the file BEFORE re-encryption!

Wait, let me check if the re-encryption updates the file on disk...

Looking at `maybeReencryptLocal()` line 369:
```go
_ = os.Rename(tempPath, config.SecretsFile())
```

Yes, it DOES update the file on disk. So the flow is:

1. Fetch remote file → temp file
2. Cache remote's public keys
3. `maybeReencryptLocal()` → Re-encrypts local file on disk with new recipients
4. Read localContent from disk (now has all local secrets re-encrypted)
5. Read remoteContent from temp file
6. Merge
7. Write merged back to disk

So after step 4, hostB's file should have `KEY2` encrypted to `[age1aaa..., age1bbb...]`.

#### Second Sync: HostA → HostB

**HostA runs `env-sync`:**
- Discovers hostB
- Fetches hostB's file
- hostB's file has:
  - `PUBLIC_KEYS: B:age1bbb...,A:age1aaa...`
  - `KEY1="encrypted_to_both"` (from hostA originally, already had both)
  - `KEY2="encrypted_to_both"` (re-encrypted by maybeReencryptLocal on line 218!)
- `CanDecryptFile()` = TRUE (age1aaa is in PUBLIC_KEYS) ✅
- Merges successfully
- Both machines now have both secrets, all encrypted to both keys ✅

## Conclusion

**The flow SHOULD work correctly!** The key is that `maybeReencryptLocal()` on line 218 (sync.go) re-encrypts the local file BEFORE the merge happens, so the merged file has all secrets properly encrypted.

## Potential Issues

However, there could be edge cases:

### Issue 1: Timing of `maybeReencryptLocal()`

If there's an error in `maybeReencryptLocal()`, it silently returns without error. Looking at lines 329-372:

```go
func maybeReencryptLocal() {
    if _, err := os.Stat(config.SecretsFile()); err != nil {
        return  // Silent return
    }
    if !keys.IsFileEncrypted(config.SecretsFile()) {
        return  // Silent return
    }
    if !keys.CanDecryptFile(config.SecretsFile()) {
        return  // Silent return - THIS IS THE PROBLEM!
    }
    ...
}
```

**BUG FOUND**: At line 336, if `CanDecryptFile()` returns false, the function silently returns without re-encrypting!

**Scenario where this breaks the flow:**

After caching A's pubkey at line 212, hostB calls `maybeReencryptLocal()` at line 218. At this point:
- hostB's file has `PUBLIC_KEYS: B:age1bbb...` (hasn't been updated yet)
- hostB's file has `KEY2="encrypted_to_B_only"`
- `CanDecryptFile()` checks: Is age1bbb in PUBLIC_KEYS? **YES** ✅
- Re-encryption proceeds normally

So this isn't the issue in the normal flow.

### Issue 2: Race Condition in File Reads

Between lines 218 and 249, the file is re-encrypted. But what if line 249 reads a cached version?

No, Go's `os.ReadFile` reads directly from disk, so this shouldn't be an issue.

### Issue 3: Partial Decryption Failures

In `reencryptSecrets()` at lines 430-434 (sync.go):

```go
decrypted, err := keys.DecryptValue(matches[2])
if err != nil {
    logging.Log("WARN", "Failed to decrypt "+matches[1]+" during re-encryption (skipping re-encryption for this key)")
    newLines = append(newLines, line)  // Keep original encrypted line!
    continue
}
```

**THIS IS THE ACTUAL BUG!**

If `maybeReencryptLocal()` is called on hostB after caching A's pubkey:
- hostB tries to re-encrypt its file
- hostB's file has `KEY2="encrypted_to_B_only"`
- hostB can decrypt KEY2 (it's encrypted to B's key) ✅
- But... wait, this should work fine.

Let me think about when this would fail...

**AH! I found it!**

What if the flow is different? What if hostB syncs from hostA BEFORE adding KEY2?

1. hostB init --encrypted
2. hostB sync (gets KEY1 from hostA, but can't decrypt - triggers registration)
3. After registration, hostB can decrypt KEY1
4. hostB add KEY2=val2
5. At this point, hostB has:
   - `KEY1="encrypted_to_both"` (from hostA)
   - `PUBLIC_KEYS: B:age1bbb...,A:age1aaa...` (both keys)
6. When hostB adds KEY2, it encrypts to recipients from PUBLIC_KEYS = `[age1aaa..., age1bbb...]` ✅
7. So KEY2 is encrypted to both from the start!

This works fine too.

## The REAL Issue

Let me re-read the problem statement more carefully...

The user says:
> "analyse the project and see what could be a reason when first time we onboard two machines into a network, the keys of machineA are not decryptable on machineB?"

The expected flow is:
1. hostA init encrypted
2. hostA set KEY1=val1 (only decryptable on hostA)
3. hostB init
4. hostB set KEY2=val2 (only decryptable on hostB)
5. sync from hostB
6. hostB should trigger re-encryption on hostA with hostB pubkey
7. hostB should also re-encrypt KEY2 so hostA can decrypt it
8. hostA syncs - should be able to decrypt KEY2

The question is: **What could go wrong?**

After my analysis, the flow SHOULD work. But let me check if there's a logging/visibility issue or an actual code bug...

## Potential Bugs Found

### Bug 1: Silent Failure in `maybeReencryptLocal()` (Line 336)

If a file is encrypted but the local machine can't decrypt it (lost key, key mismatch), `maybeReencryptLocal()` silently returns without logging an error. This could hide issues.

**Fix**: Add logging when returning early.

### Bug 2: Silent Failure in `reencryptSecrets()` (Lines 430-434)

If a specific secret fails to decrypt during re-encryption, it's kept in its old encrypted form. This could mean some secrets aren't re-encrypted to new recipients.

**Fix**: Return error instead of silently skipping, OR log more prominently.

### Bug 3: No Verification After Auto-Registration

After auto-registration (line 207), we re-fetch and assume it worked. But what if the remote re-encryption failed?

**Fix**: Add verification that the re-fetched file is actually decryptable.

### Bug 4: `maybeReencryptLocal()` Called Before File Exists

In the initial flow, if hostB hasn't created its own secrets file yet, `maybeReencryptLocal()` at line 218 returns early at line 330. This is fine, but could be more explicit.

## Recommended Fixes

1. Add verbose logging to `maybeReencryptLocal()` to show when and why it returns early
2. Add error instead of warning when `reencryptSecrets()` fails to decrypt a value
3. Add verification after auto-registration that file is decryptable
4. Add integration test for the two-machine scenario

