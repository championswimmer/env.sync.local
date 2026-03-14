<script setup lang="ts">
useHead({
  title: 'Installation guides | env-sync',
  meta: [
    { name: 'description', content: 'Install env-sync on Linux, macOS, and WSL2. Choose from Quickstart, Trusted Peers (SSH), or Secure Peers (mTLS + AGE) setup guides.' },
    { name: 'keywords', content: 'env-sync install, dotenv sync setup, Linux secrets sync, macOS secrets sync, WSL2, peer-to-peer sync install' },
    { property: 'og:title', content: 'Install env-sync | Setup guides for every scenario' },
    { property: 'og:description', content: 'Quickstart in under a minute, or follow step-by-step guides for trusted-owner SSH or secure-peer mTLS setups.' },
    { property: 'og:type', content: 'article' },
    { property: 'og:url', content: 'https://envsync.arnav.tech/installation' },
    { property: 'og:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:title', content: 'Install env-sync | Setup guides for every scenario' },
    { name: 'twitter:description', content: 'Quickstart in under a minute, or follow step-by-step guides for trusted-owner SSH or secure-peer mTLS setups.' },
    { name: 'twitter:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
  ],
  link: [
    { rel: 'canonical', href: 'https://envsync.arnav.tech/installation' },
  ],
})
</script>

<template>
  <div class="subpage-hero">
    <h1>Installation guides</h1>
    <p>Pick the guide that matches your scenario — from a 60-second quickstart to full step-by-step setup for multi-device networks.</p>
  </div>

  <div class="features" style="margin:0 0 2.5rem;">
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-bolt"></i></div>
      <h3><NuxtLink to="/installation/quickstart">Quickstart</NuxtLink></h3>
      <p>Install env-sync in under a minute. One command, no accounts, no server. Covers install, init, and first sync.</p>
      <div style="margin-top:1rem;">
        <NuxtLink class="btn btn-primary" to="/installation/quickstart">Quickstart →</NuxtLink>
      </div>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-terminal"></i></div>
      <h3><NuxtLink to="/installation/trusted-peers">Trusted Peers (SSH)</NuxtLink></h3>
      <p>Step-by-step guide for <strong>trusted-owner-ssh</strong> mode. Add your first device, onboard more machines, enable encryption, and remove devices.</p>
      <div style="margin-top:1rem;">
        <NuxtLink class="btn btn-primary" to="/installation/trusted-peers">Trusted Peers guide →</NuxtLink>
      </div>
    </article>
    <article class="panel feature">
      <div class="icon"><i class="fa-solid fa-shield-halved"></i></div>
      <h3><NuxtLink to="/installation/secure-peers">Secure Peers (mTLS)</NuxtLink></h3>
      <p>Step-by-step guide for <strong>secure-peer</strong> mode. Invitation-based onboarding, mTLS authentication, AGE encryption, and peer revocation.</p>
      <div style="margin-top:1rem;">
        <NuxtLink class="btn btn-primary" to="/installation/secure-peers">Secure Peers guide →</NuxtLink>
      </div>
    </article>
  </div>

  <section class="panel" style="max-width:720px;margin:0 auto 1rem;">
    <h2>Quick install (all modes)</h2>
    <pre><code># system-wide install
curl -fsSL https://envsync.arnav.tech/install.sh | sudo bash

# user-only install (no sudo)
curl -fsSL https://envsync.arnav.tech/install.sh | bash -s -- --user</code></pre>
    <p>The installer defaults to the CLI. Add <code>--all</code> to install both the CLI and desktop GUI, or use <code>--gui-only</code> if you only want the GUI app.</p>
    <pre><code># install both CLI + GUI
curl -fsSL https://envsync.arnav.tech/install.sh | bash -s -- --user --all

# install only the GUI
curl -fsSL https://envsync.arnav.tech/install.sh | bash -s -- --user --gui-only</code></pre>
  </section>

  <section class="panel" style="max-width:720px;margin:0 auto 1rem;">
    <h2>Prefer a desktop app?</h2>
    <p>env-sync also ships an optional desktop GUI built with Wails. It uses the same configuration, secrets, keys, and peer registry as the CLI, so you can install one or both and switch between them at any time.</p>
    <p>For the full GUI walkthrough, platform notes, and development details, see <a href="https://github.com/championswimmer/env.sync.local/blob/main/docs/GUI.md">GUI.md</a>.</p>
  </section>

  <section class="panel" style="max-width:720px;margin:0 auto 1rem;">
    <h2>Not sure which guide?</h2>
    <table>
      <thead><tr><th>Scenario</th><th>Guide</th></tr></thead>
      <tbody>
        <tr><td data-label="Scenario">Just trying it out, single machine</td><td data-label="Guide"><NuxtLink to="/installation/quickstart">Quickstart</NuxtLink></td></tr>
        <tr><td data-label="Scenario">All machines are yours (laptop, desktop, server)</td><td data-label="Guide"><NuxtLink to="/installation/trusted-peers">Trusted Peers</NuxtLink></td></tr>
        <tr><td data-label="Scenario">Multiple people sharing secrets, no SSH access to each other</td><td data-label="Guide"><NuxtLink to="/installation/secure-peers">Secure Peers</NuxtLink></td></tr>
      </tbody>
    </table>
  </section>

  <section class="cta-banner">
    <h2>Already installed?</h2>
    <p>Check out the usage guide for secret management, sync operations, and automation.</p>
    <div class="cta-row" style="justify-content:center;">
      <NuxtLink class="btn btn-primary" to="/usage">Usage guide →</NuxtLink>
      <NuxtLink class="btn btn-secondary" to="/modes">Explore modes</NuxtLink>
    </div>
  </section>
</template>
