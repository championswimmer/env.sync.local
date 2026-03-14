<script setup lang="ts">
</script>

<template>
  <header>
    <div class="nav">
      <NuxtLink class="brand" to="/">env-sync</NuxtLink>
      <nav><ul>
        <li class="dropdown"><NuxtLink to="/installation">Install</NuxtLink><ul class="dropdown-menu"><li><NuxtLink to="/download">Download</NuxtLink></li><li><NuxtLink to="/installation/quickstart">Quickstart</NuxtLink></li><li><NuxtLink to="/installation/trusted-peers">Trusted Peers Setup</NuxtLink></li><li><NuxtLink to="/installation/secure-peers">Secure Peers Setup</NuxtLink></li></ul></li>
        <li><NuxtLink to="/usage">Usage</NuxtLink></li>
        <li><NuxtLink to="/modes">Modes</NuxtLink></li>
        <li><NuxtLink to="/security">Security</NuxtLink></li>
        <li><NuxtLink to="/comparisons">Comparisons</NuxtLink></li>
      </ul></nav>
      <a class="github-link" href="https://github.com/championswimmer/env.sync.local"><i class="fa-brands fa-github"></i> GitHub</a>
    </div>
  </header>
</template>
