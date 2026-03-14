<script setup lang="ts">
useHead({
  title: 'Download env-sync | Latest CLI and GUI builds',
  meta: [
    { name: 'description', content: 'Download the latest env-sync release assets from GitHub. Choose CLI or GUI builds by OS and architecture.' },
    { property: 'og:title', content: 'Download env-sync | Latest CLI and GUI builds' },
    { property: 'og:description', content: 'Pick CLI or GUI, select your OS and architecture, and download the latest env-sync release directly from GitHub.' },
    { property: 'og:type', content: 'website' },
    { property: 'og:url', content: 'https://envsync.arnav.tech/download' },
    { property: 'og:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:title', content: 'Download env-sync | Latest CLI and GUI builds' },
    { name: 'twitter:description', content: 'Choose CLI or GUI downloads for Linux, macOS, and Windows from the latest env-sync GitHub release.' },
    { name: 'twitter:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
  ],
  link: [
    { rel: 'canonical', href: 'https://envsync.arnav.tech/download' },
  ],
})

interface ReleaseAsset {
  name: string
  size: number
  browser_download_url: string
}

interface AssetIndex {
  [appType: string]: {
    [os: string]: {
      [arch: string]: ReleaseAsset[]
    }
  }
}

const OS_META: Record<string, { label: string; icon: string }> = {
  linux: { label: 'Linux', icon: 'fa-brands fa-linux' },
  macos: { label: 'macOS', icon: 'fa-brands fa-apple' },
  windows: { label: 'Windows', icon: 'fa-brands fa-windows' },
}

const assetIndex = ref<AssetIndex>({})
const selectedType = ref('')
const selectedOS = ref('')
const selectedArch = ref('')
const releaseLabel = ref('Loading latest release…')
const loadError = ref(false)
const loaded = ref(false)

function classifyAsset(asset: ReleaseAsset) {
  const guiMatch = asset.name.match(/^env-sync-gui-(linux|macos|windows)-([a-z0-9_]+)(?:-(?:installer|portable))?(?:\.[a-z0-9.]+)?$/)
  if (guiMatch) {
    return { appType: 'gui', os: guiMatch[1], arch: guiMatch[2] }
  }

  const cliMatch = asset.name.match(/^env-sync-(linux|macos|windows)-([a-z0-9_]+)(?:\.[a-z0-9.]+)?$/)
  if (cliMatch) {
    return { appType: 'cli', os: cliMatch[1], arch: cliMatch[2] }
  }

  return null
}

function buildAssetIndex(assets: ReleaseAsset[]): AssetIndex {
  const index: AssetIndex = {}
  for (const asset of assets) {
    const info = classifyAsset(asset)
    if (!info) continue

    index[info.appType] = index[info.appType] || {}
    index[info.appType][info.os] = index[info.appType][info.os] || {}
    index[info.appType][info.os][info.arch] = index[info.appType][info.os][info.arch] || []
    index[info.appType][info.os][info.arch].push(asset)
  }
  return index
}

function formatFileSize(bytes: number): string {
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
}

const appTypes = computed(() => Object.keys(assetIndex.value))

const osList = computed(() => Object.keys(assetIndex.value[selectedType.value] || {}))

const archList = computed(() =>
  Object.keys(assetIndex.value[selectedType.value]?.[selectedOS.value] || {}),
)

const showArchSelector = computed(() => archList.value.length > 1)

const currentAssets = computed<ReleaseAsset[]>(() =>
  assetIndex.value[selectedType.value]?.[selectedOS.value]?.[selectedArch.value] || [],
)

function selectType(type: string) {
  selectedType.value = type
  const oses = Object.keys(assetIndex.value[type] || {})
  if (!oses.includes(selectedOS.value)) {
    selectedOS.value = oses[0] || ''
  }
  reconcileArch()
}

function selectOS(os: string) {
  selectedOS.value = os
  reconcileArch()
}

function selectArch(arch: string) {
  selectedArch.value = arch
}

function reconcileArch() {
  const archs = Object.keys(
    assetIndex.value[selectedType.value]?.[selectedOS.value] || {},
  )
  if (!archs.includes(selectedArch.value)) {
    selectedArch.value = archs[0] || ''
  }
}

// Post-install tips logic
interface Tip {
  html: string
  modes?: string[]
  oses?: string[]
  arches?: string[]
}

const installDocsMap: Record<string, string> = {
  linux: '/installation/quickstart#linux',
  macos: '/installation/quickstart#macos',
  windows: '/installation/quickstart#windows',
}

const TIP_OS_CLASS: Record<string, string> = { linux: 'os-lin', macos: 'os-mac', windows: 'os-win' }
const TIP_MODE_CLASS: Record<string, string> = { cli: 'mode-cli', gui: 'mode-gui' }

function sanitizeTagValue(value: string): string {
  return String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '-')
}

function tagClass(prefix: string, value: string, map?: Record<string, string>): string {
  const mapped = map?.[value]
  if (mapped) return mapped
  return `${prefix}-${sanitizeTagValue(value)}`
}

function tipClasses(tip: Tip): string[] {
  const classes: string[] = []
  for (const mode of tip.modes || []) {
    classes.push(tagClass('mode', mode, TIP_MODE_CLASS))
  }
  for (const os of tip.oses || []) {
    classes.push(tagClass('os', os, TIP_OS_CLASS))
  }
  for (const arch of tip.arches || []) {
    classes.push(tagClass('arch', arch))
  }
  return classes
}

function matchesTip(tip: Tip): boolean {
  const modeMatch = !tip.modes || tip.modes.includes(selectedType.value)
  const osMatch = !tip.oses || tip.oses.includes(selectedOS.value)
  const archMatch = !tip.arches || tip.arches.includes(selectedArch.value)
  return modeMatch && osMatch && archMatch
}

const baseTips: Tip[] = [
  {
    html: 'CLI binaries must be added to your PATH before you can run env-sync from any terminal.',
    modes: ['cli'],
  },
  {
    html: 'On Windows, place env-sync.exe in a stable folder and add that folder to your User PATH.',
    modes: ['cli'],
    oses: ['windows'],
  },
  {
    html: 'On Linux, put the binary in a PATH directory such as <code>~/.local/bin</code> or <code>/usr/local/bin</code>, then run <code>chmod +x env-sync</code> before first use.',
    modes: ['cli'],
    oses: ['linux'],
  },
  {
    html: 'On macOS, move the binary to /usr/local/bin (or another PATH directory) and run <code>chmod +x env-sync</code> before first use.',
    modes: ['cli'],
    oses: ['macos'],
  },
  {
    html: 'After PATH setup, verify with <code>env-sync --version</code> and then run <code>env-sync setup</code>.',
    modes: ['cli'],
  },
  {
    html: 'For first-time setup guidance, open the installation docs linked below after installing.',
    modes: ['gui'],
  },
  {
    html: 'For Linux app integration, prefer the .deb asset when available. It installs app metadata so env-sync appears in your desktop app launcher.',
    modes: ['gui'],
    oses: ['linux'],
  },
  {
    html: 'If you use the raw Linux GUI binary instead, run <code>chmod +x</code> on it and create a desktop launcher entry manually.',
    modes: ['gui'],
    oses: ['linux'],
  },
  {
    html: 'If Windows SmartScreen warns on first launch, click "More info" and then "Run anyway" for this unsigned build.',
    modes: ['gui'],
    oses: ['windows'],
  },
  {
    html: 'macOS may block unsigned binaries/apps with quarantine. See <a href="https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unidentified-developer-mh40616/mac" target="_blank" rel="noopener">Apple guidance</a> and, if you want a walkthrough, <a href="https://chatgpt.com/?q=how+to+remove+quarantine+flag+on+macOS+for+an+app+or+binary" target="_blank" rel="noopener">ChatGPT walkthrough</a>.',
    oses: ['macos'],
  },
  {
    html: 'Pick the <code>amd64</code> build for Intel/AMD x86_64 machines.',
    arches: ['amd64'],
  },
  {
    html: 'Pick the <code>arm64</code> build for Apple Silicon and ARM64 Linux devices.',
    arches: ['arm64'],
  },
]

const allTips = computed<Tip[]>(() => {
  const tips = [...baseTips]

  // The quickstart tip always appears (no filters)
  tips.push({
    html: 'Need setup steps after install? Continue with <a href="/installation/quickstart">Quickstart</a>.',
  })

  const osLabel = OS_META[selectedOS.value]?.label || 'your platform'
  const docsHref = installDocsMap[selectedOS.value]

  if (docsHref) {
    tips.push({
      html: `Platform-specific docs: <a href="${docsHref}">${osLabel} install instructions</a>.`,
      oses: [selectedOS.value],
    })
  }

  return tips
})

const visibleTips = computed(() => {
  if (!selectedType.value || !selectedOS.value) return []
  return allTips.value
    .map((tip) => ({
      ...tip,
      classes: tipClasses(tip),
      visible: matchesTip(tip),
    }))
    .filter((tip) => tip.visible)
})

const showDefaultTipMessage = computed(() => !selectedType.value || !selectedOS.value)
const showNoTipsMessage = computed(() => !showDefaultTipMessage.value && visibleTips.value.length === 0)

async function loadLatestRelease() {
  try {
    const response = await fetch('/download/latest-release.json', { cache: 'no-store' })
    if (!response.ok) {
      throw new Error(`Release data returned ${response.status}`)
    }
    const release = await response.json()
    assetIndex.value = buildAssetIndex(release.assets || [])
    releaseLabel.value = release.tag_name ? `Latest release: ${release.tag_name}` : 'Latest release'

    if (!Object.keys(assetIndex.value).length) {
      loaded.value = true
      return
    }

    // Initialize selections
    selectedType.value = Object.keys(assetIndex.value)[0] || ''
    const oses = Object.keys(assetIndex.value[selectedType.value] || {})
    selectedOS.value = oses[0] || ''
    reconcileArch()
    loaded.value = true
  }
  catch (err) {
    console.error('Failed to load prebuilt latest release assets:', err)
    releaseLabel.value = 'Unable to load latest release'
    loadError.value = true
  }
}

onMounted(() => {
  loadLatestRelease()
})
</script>

<template>
  <div class="subpage-hero">
    <h1>Download env-sync</h1>
    <p>Pick your app type, operating system, and architecture. Downloads are sourced from the <strong>latest GitHub release</strong> of this repository.</p>
    <p class="muted-note"><strong>Tip:</strong> The web-based install script is usually easier. If you prefer to manually download binaries (or avoid running untrusted <code>.sh</code> scripts), install from the assets below.</p>
  </div>

  <section class="panel download-controls">
    <div class="download-group">
      <h3>1) Choose app type</h3>
      <div class="option-grid">
        <button
          v-for="type in appTypes"
          :key="type"
          type="button"
          :class="['option-btn', { active: type === selectedType }]"
          @click="selectType(type)"
        >
          {{ type.toUpperCase() }}
        </button>
      </div>
    </div>
    <div class="download-group">
      <h3>2) Choose operating system</h3>
      <div class="option-grid">
        <button
          v-for="os in osList"
          :key="os"
          type="button"
          :class="['option-btn', { active: os === selectedOS }]"
          @click="selectOS(os)"
        >
          <i :class="(OS_META[os] || { icon: 'fa-solid fa-desktop' }).icon"></i>
          {{ (OS_META[os] || { label: os }).label }}
        </button>
      </div>
    </div>
    <div v-show="showArchSelector" class="download-group download-arch">
      <h3>3) Choose architecture</h3>
      <select aria-label="Choose architecture" :value="selectedArch" @change="selectArch(($event.target as HTMLSelectElement).value)">
        <option v-for="arch in archList" :key="arch" :value="arch">{{ arch }}</option>
      </select>
    </div>
  </section>

  <section class="panel download-results">
    <h3>{{ releaseLabel }}</h3>
    <div class="download-assets">
      <!-- Loading state -->
      <p v-if="!loaded && !loadError" class="muted-note">Loading release assets.</p>

      <!-- Error state -->
      <template v-if="loadError">
        <p class="muted-note">Could not load release assets right now. Please use the releases page directly.</p>
        <p><a class="btn btn-secondary" href="https://github.com/championswimmer/env.sync.local/releases"><i class="fa-brands fa-github"></i> Open releases</a></p>
      </template>

      <!-- No assets in release -->
      <p v-if="loaded && !loadError && appTypes.length === 0" class="muted-note">No downloadable assets found in the latest release.</p>

      <!-- No match for current selection -->
      <p v-if="loaded && !loadError && appTypes.length > 0 && currentAssets.length === 0" class="muted-note">No assets found for this combination in the latest release.</p>

      <!-- Asset cards -->
      <article v-for="asset in currentAssets" :key="asset.name" class="panel">
        <h4 style="margin:0 0 0.35rem;">{{ asset.name }}</h4>
        <p class="muted-note" style="margin:0 0 0.7rem;">{{ formatFileSize(asset.size) }}</p>
        <a class="btn btn-primary" :href="asset.browser_download_url">
          <i class="fa-solid fa-download"></i> Download
        </a>
      </article>
    </div>
  </section>

  <section class="panel download-guidance">
    <h3>After download</h3>
    <ul class="post-install-tips">
      <li v-if="showDefaultTipMessage">Pick an app type and OS to see install and first-run guidance.</li>
      <li v-if="showNoTipsMessage">No specific tips for this selection. Continue with the Quickstart guide.</li>
      <li
        v-for="(tip, idx) in visibleTips"
        :key="idx"
        :class="tip.classes"
        v-html="tip.html"
      ></li>
    </ul>
  </section>

  <section class="cta-banner">
    <h2>Need an older version?</h2>
    <p>Browse all published tags and assets on GitHub releases.</p>
    <div class="cta-row" style="justify-content:center;">
      <a class="btn btn-secondary" href="https://github.com/championswimmer/env.sync.local/releases"><i class="fa-brands fa-github"></i> View all releases</a>
    </div>
  </section>
</template>
