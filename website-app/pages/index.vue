<script setup lang="ts">
useHead({
  title: 'env-sync | Sync .env secrets across machines — no server needed',
  meta: [
    { name: 'description', content: 'env-sync is a peer-to-peer .env secrets synchronization tool for local networks. Auto-discover peers with mDNS, sync over SSH or mTLS, and keep every machine in sync — no central server required.' },
    { property: 'og:title', content: 'env-sync | Sync .env secrets across machines — no server needed' },
    { property: 'og:description', content: 'env-sync is a peer-to-peer .env secrets synchronization tool for local networks. Auto-discover peers with mDNS, sync over SSH or mTLS, and keep every machine in sync — no central server required.' },
    { property: 'og:url', content: 'https://envsync.arnav.tech/' },
    { property: 'og:type', content: 'website' },
    { name: 'twitter:title', content: 'env-sync | Sync .env secrets across machines — no server needed' },
    { name: 'twitter:description', content: 'env-sync is a peer-to-peer .env secrets synchronization tool for local networks. Auto-discover peers with mDNS, sync over SSH or mTLS, and keep every machine in sync — no central server required.' },
  ],
  link: [
    { rel: 'canonical', href: 'https://envsync.arnav.tech/' },
  ],
})
</script>

<template>
  <!-- Hero -->
  <section class="hero">
    <div>
      <h1>Stop copying <span class="highlight">.env</span> files<br/>between machines</h1>
      <p>env-sync keeps secrets in sync across every machine on your local network — automatically. No central server, no cloud dependency, no manual copy-paste.</p>
      <p>
        <span class="badge"><i class="fa-brands fa-osi"></i> Open Source</span>
        <span class="badge"><i class="fa-solid fa-satellite-dish"></i> mDNS Auto-Discovery</span>
        <span class="badge"><i class="fa-solid fa-terminal"></i> SSH &amp; <i class="fa-solid fa-shield-halved"></i> mTLS Transport</span>
        <span class="badge"><i class="fa-solid fa-lock"></i> AGE Encryption</span>
      </p>
      <div class="cta-row">
        <NuxtLink class="btn btn-primary" to="/download"><i class="fa-solid fa-download"></i> Download now</NuxtLink>
        <NuxtLink class="btn btn-secondary" to="/installation">Get started — it's free</NuxtLink>
        <a class="btn btn-secondary" href="https://github.com/championswimmer/env.sync.local"><i class="fa-brands fa-github"></i> View on GitHub</a>
      </div>
    </div>
    <img src="/assets/cover.png" alt="env-sync peer-to-peer secrets synchronization across local machines" />
  </section>

  <!-- Install snippet -->
  <section class="panel" style="max-width:720px;margin:0 auto 2.5rem;">
    <pre><code># install in 10 seconds
curl -fsSL https://envsync.arnav.tech/install.sh | bash -s -- --user

# start syncing
env-sync init
env-sync add OPENAI_API_KEY="sk-..."
env-sync sync</code></pre>
  </section>

  <!-- Trust bar -->
  <div class="trust-bar">
    <span><i class="fa-solid fa-lock"></i> End-to-end encrypted</span>
    <span><i class="fa-solid fa-desktop"></i> Linux, macOS &amp; WSL2</span>
    <span><i class="fa-brands fa-golang"></i> Written in Go</span>
    <span><i class="fa-solid fa-satellite-dish"></i> Zero-config discovery</span>
    <span><i class="fa-brands fa-osi"></i> 100% open source</span>
  </div>

  <!-- How it works -->
  <div class="section-heading">
    <h2>Up and running in 3 steps</h2>
    <p>No accounts to create, no servers to manage, no cloud services to configure.</p>
  </div>
  <div class="steps">
    <article class="panel step">
      <h3><i class="fa-solid fa-download"></i> Install</h3>
      <p>One-liner install on every machine. Works on Linux, macOS, and WSL2.</p>
    </article>
    <article class="panel step">
      <h3><i class="fa-solid fa-key"></i> Initialize</h3>
      <p>Run <code>env-sync init</code> to set up keys and choose your security mode.</p>
    </article>
    <article class="panel step">
      <h3><i class="fa-solid fa-rotate"></i> Sync</h3>
      <p>Peers discover each other via <i class="fa-solid fa-satellite-dish"></i> mDNS. Secrets merge automatically with backups.</p>
    </article>
  </div>

  <!-- Why env-sync -->
  <div class="section-heading">
    <h2>Built for teams that work offline-first</h2>
    <p>Unlike cloud-based secret managers, env-sync works entirely on your local network — even without an internet connection.</p>
  </div>
  <div class="features">
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-satellite-dish"></i></div>
      <h3>Auto-discovery with mDNS</h3>
      <p>Machines find each other automatically using Avahi or Bonjour. No manual IP configuration needed.</p>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-shield-halved"></i></div>
      <h3>Three security modes</h3>
      <p>From zero-config SSH for personal devices to mTLS + AGE encryption for cross-team collaboration.</p>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-rotate"></i></div>
      <h3>Smart merge &amp; backup</h3>
      <p>Per-key timestamps prevent data loss. Automatic backups before every write — keeps last 5 versions.</p>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-brands fa-golang"></i></div>
      <h3>Lightweight &amp; fast</h3>
      <p>Single Go binary. No runtime dependencies, no background database, no Docker required.</p>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-clock"></i></div>
      <h3>Sync on autopilot</h3>
      <p>Cron-based sync every 30 minutes, shell hook on startup, or manual trigger — your choice.</p>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-users"></i></div>
      <h3>Peer management</h3>
      <p>Invite, approve, and revoke peers with signed membership events that propagate automatically.</p>
    </article>
  </div>

  <!-- Mode comparison table -->
  <div class="section-heading">
    <h2>Choose your security model</h2>
    <p>env-sync gives you explicit control over trust boundaries. Pick the mode that fits your team.</p>
  </div>
  <section class="panel">
    <table>
      <thead><tr><th>Mode</th><th>Storage</th><th>Transport</th><th>Best for</th></tr></thead>
      <tbody>
        <tr><td data-label="Mode"><strong>trusted-owner-ssh</strong> (default)</td><td data-label="Storage">Plaintext (optional AGE)</td><td data-label="Transport"><i class="fa-solid fa-terminal"></i> SCP / SSH</td><td data-label="Best for">All machines belong to you — your laptop, desktop, NUC, server</td></tr>
        <tr><td data-label="Mode"><strong>secure-peer</strong></td><td data-label="Storage">AGE encrypted (mandatory)</td><td data-label="Transport"><i class="fa-solid fa-shield-halved"></i> HTTPS + mTLS</td><td data-label="Best for">Multiple people sharing secrets without giving each other shell access</td></tr>
        <tr><td data-label="Mode"><strong>dev-plaintext-http</strong></td><td data-label="Storage">Plaintext</td><td data-label="Transport"><i class="fa-solid fa-network-wired"></i> HTTP</td><td data-label="Best for">Quick local debugging only — not for real secrets</td></tr>
      </tbody>
    </table>
    <div style="text-align:center;margin-top:1rem;">
      <NuxtLink class="btn btn-secondary" to="/modes">Learn about each mode →</NuxtLink>
    </div>
  </section>

  <!-- Comparison callout -->
  <div class="section-heading">
    <h2>How does env-sync compare?</h2>
    <p>env-sync fills the gap that centralized tools leave open — secrets on local machines that drift apart.</p>
  </div>
  <div class="grid">
    <article class="panel comparison-card">
      <div class="vs-badge">vs</div>
      <h2><NuxtLink to="/comparisons/vault-vs-envsync">HashiCorp Vault</NuxtLink></h2>
      <p>Enterprise secrets platform vs lightweight peer sync.</p>
    </article>
    <article class="panel comparison-card">
      <div class="vs-badge">vs</div>
      <h2><NuxtLink to="/comparisons/infisical-vs-envsync">Infisical</NuxtLink></h2>
      <p>Open-source secrets platform vs decentralized LAN mesh.</p>
    </article>
    <article class="panel comparison-card">
      <div class="vs-badge">vs</div>
      <h2><NuxtLink to="/comparisons/doppler-vs-envsync">Doppler</NuxtLink></h2>
      <p>Cloud secrets operations vs offline-first sync.</p>
    </article>
    <article class="panel comparison-card">
      <div class="vs-badge">vs</div>
      <h2><NuxtLink to="/comparisons/dotenvx-vs-envsync">dotenvx</NuxtLink></h2>
      <p>Encrypted dotenv runtime injection vs peer synchronization.</p>
    </article>
    <article class="panel comparison-card">
      <div class="vs-badge">vs</div>
      <h2><NuxtLink to="/comparisons/sops-vs-envsync">SOPS</NuxtLink></h2>
      <p>Git-native encrypted files vs always-on machine sync.</p>
    </article>
  </div>

  <!-- Bottom CTA -->
  <section class="cta-banner">
    <h2>Ready to stop sharing .env files over Slack?</h2>
    <p>Get peer-to-peer secret sync running in under a minute. Open source, self-hosted, no accounts needed.</p>
    <div class="cta-row" style="justify-content:center;">
      <NuxtLink class="btn btn-primary" to="/installation">Install env-sync</NuxtLink>
      <NuxtLink class="btn btn-secondary" to="/usage">Read the usage guide</NuxtLink>
    </div>
  </section>
</template>
