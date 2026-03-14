<script setup lang="ts">
useHead({
  title: 'Security modes explained | env-sync',
  meta: [
    { name: 'description', content: 'Compare env-sync\'s three security modes: trusted-owner-ssh for personal fleets, secure-peer for cross-team collaboration, and dev-plaintext-http for debugging.' },
    { property: 'og:title', content: 'Security modes explained | env-sync' },
    { property: 'og:description', content: 'Compare env-sync\'s three security modes: trusted-owner-ssh for personal fleets, secure-peer for cross-team collaboration, dev-plaintext-http for debugging.' },
    { property: 'og:type', content: 'article' },
    { property: 'og:url', content: 'https://envsync.arnav.tech/modes' },
    { property: 'og:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:title', content: 'Security modes explained | env-sync' },
    { name: 'twitter:description', content: 'Three security modes for different trust boundaries: SSH, mTLS+AGE, or plaintext debug. Pick the one that fits.' },
    { name: 'twitter:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
    { name: 'keywords', content: 'env-sync modes, SSH sync, mTLS secrets, AGE encryption, trusted-owner, secure-peer, security modes' },
  ],
  link: [
    { rel: 'canonical', href: 'https://envsync.arnav.tech/modes' },
  ],
})
</script>

<template>
  <div class="subpage-hero">
    <h1>Three modes, one CLI</h1>
    <p>env-sync gives you explicit control over how secrets are stored, transported, and who gets access. Pick the security model that fits your scenario.</p>
  </div>

  <section class="panel">
    <h2>Mode comparison at a glance</h2>
    <table>
      <thead><tr><th>Dimension</th><th>dev-plaintext-http</th><th>trusted-owner-ssh</th><th>secure-peer</th></tr></thead>
      <tbody>
        <tr><td data-label="Dimension"><strong>Storage</strong></td><td data-label="dev-plaintext-http">Plaintext</td><td data-label="trusted-owner-ssh">Plaintext (optional AGE)</td><td data-label="secure-peer">AGE encrypted (mandatory)</td></tr>
        <tr><td data-label="Dimension"><strong>Transport</strong></td><td data-label="dev-plaintext-http"><i class="fa-solid fa-network-wired"></i> HTTP</td><td data-label="trusted-owner-ssh"><i class="fa-solid fa-terminal"></i> SCP / SSH</td><td data-label="secure-peer"><i class="fa-solid fa-shield-halved"></i> HTTPS + mTLS</td></tr>
        <tr><td data-label="Dimension"><strong>Onboarding</strong></td><td data-label="dev-plaintext-http">Open — any peer on the network</td><td data-label="trusted-owner-ssh">Zero-touch if <i class="fa-solid fa-terminal"></i> SSH access exists</td><td data-label="secure-peer">Invitation + explicit approval</td></tr>
        <tr><td data-label="Dimension"><strong>Authorization</strong></td><td data-label="dev-plaintext-http">None</td><td data-label="trusted-owner-ssh">Implicit via SSH trust</td><td data-label="secure-peer">Explicit approved / revoked states</td></tr>
        <tr><td data-label="Dimension"><strong>Best for</strong></td><td data-label="dev-plaintext-http">Local debugging only</td><td data-label="trusted-owner-ssh">All your own machines</td><td data-label="secure-peer">Multiple owners sharing secrets</td></tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <h2>Mode A: dev-plaintext-http</h2>
    <p>Debug-only mode with <strong>no encryption</strong> at rest or in transit and <strong>no authentication</strong>. Use this exclusively for isolated local testing — never for real secrets.</p>
    <pre><code>env-sync mode set dev-plaintext-http</code></pre>
  </section>

  <section class="panel">
    <h2>Mode B: trusted-owner-ssh <span class="badge">default</span></h2>
    <p>Ideal when every machine belongs to you. SSH provides encrypted transport and authentication automatically. Storage is plaintext by default because trust is already broad in this model; optional AGE encryption can be enabled for defense-in-depth.</p>
    <h3>How sync works</h3>
    <ol>
      <li><i class="fa-solid fa-satellite-dish"></i> Discover peers with mDNS, filter by SSH reachability.</li>
      <li><i class="fa-solid fa-terminal"></i> Fetch secrets from peers via SCP/SSH.</li>
      <li>Compare metadata versions and timestamps.</li>
      <li>Merge changes and write locally with automatic backup.</li>
    </ol>
    <pre><code>env-sync mode set trusted-owner-ssh</code></pre>
  </section>

  <section class="panel">
    <h2>Mode C: secure-peer</h2>
    <p>Designed for cross-owner collaboration. No shell access is shared between peers — mTLS handles authentication and AGE handles encryption at rest. Access requires an explicit invitation and approval step.</p>
    <h3>How sync works</h3>
    <ol>
      <li><i class="fa-solid fa-satellite-dish"></i> Discover peers over mDNS.</li>
      <li><i class="fa-solid fa-shield-halved"></i> Establish mTLS connection and verify authorization.</li>
      <li><i class="fa-solid fa-lock"></i> Fetch encrypted secrets and decrypt locally with your AGE key.</li>
      <li>Merge using per-key timestamps for granular conflict resolution.</li>
      <li>Re-encrypt to all known recipients and save with backup.</li>
      <li>Replay signed membership events for offline catch-up.</li>
    </ol>
    <pre><code>env-sync mode set secure-peer</code></pre>
  </section>

  <section class="panel">
    <h2>Choosing the right mode</h2>
    <div class="features" style="margin:1rem 0 0;">
      <article class="panel feature">
        <div class="icon"><i class="fa-solid fa-house"></i></div>
        <h3>Personal fleet</h3>
        <p>Use <strong>trusted-owner-ssh</strong>. Your laptop, desktop, server, NUC — all behind SSH trust you already manage.</p>
      </article>
      <article class="panel feature">
        <div class="icon"><i class="fa-solid fa-users"></i></div>
        <h3>Team collaboration</h3>
        <p>Use <strong>secure-peer</strong>. Team members get secrets without SSH access to each other's machines.</p>
      </article>
      <article class="panel feature">
        <div class="icon"><i class="fa-solid fa-wrench"></i></div>
        <h3>Quick debugging</h3>
        <p>Use <strong>dev-plaintext-http</strong>. Fast setup for throwaway testing — never store real credentials here.</p>
      </article>
    </div>
  </section>

  <section class="cta-banner">
    <h2>Ready to get started?</h2>
    <p>Install env-sync and initialize with your preferred security mode.</p>
    <div class="cta-row" style="justify-content:center;">
      <NuxtLink class="btn btn-primary" to="/installation">Install now</NuxtLink>
      <NuxtLink class="btn btn-secondary" to="/security">Security deep-dive</NuxtLink>
    </div>
  </section>
</template>
