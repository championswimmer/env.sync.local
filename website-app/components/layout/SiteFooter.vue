<script setup lang="ts">
</script>

<template>
  <footer>
    <div class="footer-inner">
      <div>
        <div class="footer-brand">env-sync</div>
        <p>Distributed .env sync for local networks.</p>
        <p>Open source · <a href="https://github.com/championswimmer/env.sync.local"><i class="fa-brands fa-github"></i> GitHub</a></p>
      </div>
      <div class="footer-links">
        <div>
          <h4>Install</h4>
          <NuxtLink to="/download">Download</NuxtLink>
          <NuxtLink to="/installation/quickstart">Quickstart</NuxtLink>
          <NuxtLink to="/installation/trusted-peers">Trusted Peers</NuxtLink>
          <NuxtLink to="/installation/secure-peers">Secure Peers</NuxtLink>
        </div>
        <div>
          <h4>Product</h4>
          <NuxtLink to="/usage">Usage</NuxtLink>
          <NuxtLink to="/modes">Modes</NuxtLink>
          <NuxtLink to="/security">Security</NuxtLink>
        </div>
        <div>
          <h4>Compare</h4>
          <NuxtLink to="/comparisons/vault-vs-envsync">vs Vault</NuxtLink>
          <NuxtLink to="/comparisons/infisical-vs-envsync">vs Infisical</NuxtLink>
          <NuxtLink to="/comparisons/doppler-vs-envsync">vs Doppler</NuxtLink>
          <NuxtLink to="/comparisons/dotenvx-vs-envsync">vs dotenvx</NuxtLink>
          <NuxtLink to="/comparisons/sops-vs-envsync">vs SOPS</NuxtLink>
        </div>
        <div>
          <h4>Resources</h4>
          <a href="https://github.com/championswimmer/env.sync.local"><i class="fa-brands fa-github"></i> Source code</a>
          <a href="https://github.com/championswimmer/env.sync.local/issues"><i class="fa-brands fa-github"></i> Report an issue</a>
        </div>
      </div>
    </div>
  </footer>
</template>
