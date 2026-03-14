<script setup lang="ts">
useHead({
  title: 'Security architecture | env-sync',
  meta: [
    { name: 'description', content: 'Security architecture of env-sync: trust boundaries, mTLS, AGE encryption, certificate pinning, signed membership events, and threat model.' },
    { property: 'og:title', content: 'Security architecture | env-sync' },
    { property: 'og:description', content: 'Defense-in-depth security: mTLS, AGE encryption, certificate pinning, signed membership events, and threat model for peer-to-peer secrets sync.' },
    { property: 'og:type', content: 'article' },
    { property: 'og:url', content: 'https://envsync.arnav.tech/security' },
    { property: 'og:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:title', content: 'Security architecture | env-sync' },
    { name: 'twitter:description', content: 'Defense-in-depth: mTLS, AGE encryption, certificate pinning, signed membership events for peer-to-peer secrets sync.' },
    { name: 'twitter:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
    { name: 'keywords', content: 'env-sync security, mTLS, AGE encryption, certificate pinning, threat model, peer-to-peer security, secrets encryption' },
  ],
  link: [
    { rel: 'canonical', href: 'https://envsync.arnav.tech/security' },
  ],
})
</script>

<template>
  <div class="subpage-hero">
    <h1>Security architecture</h1>
    <p>env-sync uses defense-in-depth across three explicit security modes. No implicit trust — you choose the model that fits your threat profile.</p>
  </div>

  <section class="panel">
    <h2>Security baseline by mode</h2>
    <table>
      <thead><tr><th>Dimension</th><th>dev-plaintext-http</th><th>trusted-owner-ssh</th><th>secure-peer</th></tr></thead>
      <tbody>
        <tr><td data-label="Dimension"><strong>Encryption at rest</strong></td><td data-label="dev-plaintext-http">None</td><td data-label="trusted-owner-ssh">Plaintext default (optional AGE)</td><td data-label="secure-peer">AGE encrypted (mandatory)</td></tr>
        <tr><td data-label="Dimension"><strong>Encryption in transit</strong></td><td data-label="dev-plaintext-http">None</td><td data-label="trusted-owner-ssh"><i class="fa-solid fa-terminal"></i> SSH encryption</td><td data-label="secure-peer"><i class="fa-solid fa-shield-halved"></i> HTTPS + mTLS (TLS 1.3)</td></tr>
        <tr><td data-label="Dimension"><strong>Peer authentication</strong></td><td data-label="dev-plaintext-http">None</td><td data-label="trusted-owner-ssh"><i class="fa-solid fa-terminal"></i> SSH keys</td><td data-label="secure-peer"><i class="fa-solid fa-shield-halved"></i> Mutual TLS certificates + approval</td></tr>
        <tr><td data-label="Dimension"><strong>Risk level</strong></td><td data-label="dev-plaintext-http">Debug only — not for real secrets</td><td data-label="trusted-owner-ssh">Strong when all hosts are equally trusted</td><td data-label="secure-peer">Strong for cross-owner boundaries</td></tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <h2>Secure-peer trust model</h2>
    <ul>
      <li><strong>No global root CA</strong> — trust is deployment-local with pinned certificates, not dependent on external certificate authorities.</li>
      <li><strong>Peer registry</strong> — tracks pending, approved, and revoked states for every peer.</li>
      <li><strong>Signed membership events</strong> — approve/revoke actions are cryptographically signed and replicated across all peers.</li>
      <li><strong>Replay protection</strong> — monotonic event IDs and timestamp validation prevent event replay attacks.</li>
      <li><strong>Offline catch-up</strong> — peers that were offline during membership changes receive signed events on next sync.</li>
    </ul>
  </section>

  <section class="panel">
    <h2>Threat model</h2>
    <h3>trusted-owner-ssh</h3>
    <ul>
      <li><strong>Strength:</strong> Mature SSH transport security, operational simplicity, and wide ecosystem support.</li>
      <li><strong>Trade-off:</strong> Broad trust — compromise of one peer can impact others. All peers have equivalent access.</li>
      <li><strong>Mitigation:</strong> Enable optional AGE encryption for defense-in-depth. Rotate SSH keys regularly.</li>
    </ul>
    <h3>secure-peer</h3>
    <ul>
      <li><strong>Strength:</strong> Explicit authorization required. Mandatory encryption at rest. No shell access shared between peers.</li>
      <li><strong>Strength:</strong> mTLS reduces blast radius compared to SSH trust mesh — peers authenticate without system-level access.</li>
      <li><strong>Trade-off:</strong> Higher operational overhead — invitation/approval workflow and identity material management required.</li>
      <li><strong>Mitigation:</strong> Regular peer audits. Revoke compromised peers immediately — revocation propagates via signed events.</li>
    </ul>
  </section>

  <section class="panel">
    <h2>Cryptographic primitives</h2>
    <table>
      <thead><tr><th>Component</th><th>Algorithm</th><th>Purpose</th></tr></thead>
      <tbody>
        <tr><td data-label="Component"><i class="fa-solid fa-lock"></i> AGE encryption</td><td data-label="Algorithm">X25519 + ChaCha20-Poly1305</td><td data-label="Purpose">At-rest encryption of secret values</td></tr>
        <tr><td data-label="Component"><i class="fa-solid fa-shield-halved"></i> mTLS certificates</td><td data-label="Algorithm">TLS 1.3 (X.509)</td><td data-label="Purpose">In-transit encryption and peer authentication</td></tr>
        <tr><td data-label="Component"><i class="fa-solid fa-certificate"></i> Membership events</td><td data-label="Algorithm">Signed with transport key</td><td data-label="Purpose">Cryptographic proof of peer approval/revocation</td></tr>
        <tr><td data-label="Component"><i class="fa-solid fa-terminal"></i> SSH transport</td><td data-label="Algorithm">Ed25519 / RSA keys</td><td data-label="Purpose">Encrypted file transfer in trusted-owner mode</td></tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <h2>Operational security checklist</h2>
    <ul>
      <li><i class="fa-solid fa-circle-check"></i> Keep file permissions strict — <code>600</code> for secrets files.</li>
      <li><i class="fa-solid fa-circle-check"></i> Never log secret values — env-sync's logging respects this boundary.</li>
      <li><i class="fa-solid fa-circle-check"></i> Audit approved peers regularly in secure-peer mode.</li>
      <li><i class="fa-solid fa-circle-check"></i> Use backups and key lifecycle controls during membership changes.</li>
      <li><i class="fa-solid fa-circle-check"></i> Enable AGE encryption in trusted-owner mode for defense-in-depth.</li>
      <li><i class="fa-solid fa-circle-check"></i> Revoke and rotate keys for any compromised peer immediately.</li>
    </ul>
  </section>

  <section class="cta-banner">
    <h2>See how env-sync compares to centralized tools</h2>
    <p>Understand the trade-offs between peer-to-peer sync and cloud-hosted secrets managers.</p>
    <div class="cta-row" style="justify-content:center;">
      <NuxtLink class="btn btn-primary" to="/comparisons">View comparisons</NuxtLink>
      <NuxtLink class="btn btn-secondary" to="/modes">Mode details</NuxtLink>
    </div>
  </section>
</template>
