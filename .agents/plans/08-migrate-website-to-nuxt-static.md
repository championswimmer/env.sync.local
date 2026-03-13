# Plan 08: Migrate `website/` from static HTML to Nuxt 3 (Vue + Vite + TypeScript, prerendered)

## 1) Goal

Replace the hand-authored static HTML site under `website/` with a **Nuxt 3** application that:

- is implemented in **TypeScript**
- uses **Vue components/layouts** to remove repeated header/footer/nav markup
- is built as a **fully static prerendered** site (no runtime server)
- continues deploying to `gh-pages` from GitHub Actions
- preserves current URLs/SEO behavior and existing assets/content

---

## 2) Current State Summary (to preserve behavior)

- Static routes are served from directory-style pages (`<route>/index.html`).
- Shared navigation/footer is duplicated across many files.
- Deployment workflow currently:
  1. checks out code
  2. copies `install.sh` into `website/install.sh`
  3. runs `build/fetch_latest_release_for_website.py`
  4. publishes `./website` to `gh-pages`

Migration must keep these capabilities, but move page composition into reusable Nuxt components.

---

## 3) Target Stack & Build Mode

### Framework/tooling

- **Nuxt 3** (Vue 3 + Vite)
- **TypeScript** everywhere possible (`lang="ts"` in SFCs, typed composables/data)
- Optional Nuxt modules only if needed for parity (prefer minimal dependencies)

### Static output

- Use Nuxt static generation:
  - `nuxi generate`
  - output directory: `.output/public`
- Deploy generated `.output/public` contents to `gh-pages`

### Rendering mode

- Prefer `ssr: true` + prerender for static output (best SEO parity)
- Explicitly prerender all route paths listed below

---

## 4) Proposed Repository Layout

Create a dedicated Nuxt app at `website-app/` and treat `website/` as legacy during transition.

```text
env.sync.local/
├── website-app/
│   ├── app.vue
│   ├── nuxt.config.ts
│   ├── package.json
│   ├── tsconfig.json
│   ├── public/
│   │   ├── CNAME
│   │   ├── robots.txt
│   │   ├── sitemap.xml                  # initially copied; can become generated later
│   │   ├── install.sh                   # committed placeholder; overwritten in CI from repo root install.sh
│   │   ├── .nojekyll                    # committed once so generated output always includes it
│   │   └── assets/                      # copied from current website/assets
│   ├── assets/
│   │   └── css/
│   │       └── main.css                 # migrated/adapted from website/styles.css
│   ├── layouts/
│   │   └── default.vue                  # shared header + footer shell
│   ├── components/
│   │   ├── layout/
│   │   │   ├── SiteHeader.vue
│   │   │   ├── SiteFooter.vue
│   │   │   ├── SiteNav.vue
│   │   │   └── InstallDropdown.vue
│   │   ├── common/
│   │   │   ├── HeroSection.vue
│   │   │   ├── CtaBanner.vue
│   │   │   ├── ComparisonCard.vue
│   │   │   └── TrustBadges.vue
│   │   ├── download/
│   │   │   ├── DownloadSelector.vue
│   │   │   └── PostInstallTips.vue
│   │   └── tables/
│   │       └── ModesComparisonTable.vue
│   ├── composables/
│   │   ├── useSiteMeta.ts
│   │   ├── useInstallLinks.ts
│   │   └── useLatestRelease.ts
│   ├── data/
│   │   ├── nav.ts
│   │   ├── footer.ts
│   │   ├── comparisons.ts
│   │   └── install-guides.ts
│   ├── pages/
│   │   ├── index.vue
│   │   ├── download/index.vue
│   │   ├── installation/index.vue
│   │   ├── installation/quickstart.vue
│   │   ├── installation/trusted-peers.vue
│   │   ├── installation/secure-peers.vue
│   │   ├── usage/index.vue
│   │   ├── modes/index.vue
│   │   ├── security/index.vue
│   │   └── comparisons/
│   │       ├── index.vue
│   │       ├── vault-vs-envsync.vue
│   │       ├── infisical-vs-envsync.vue
│   │       ├── doppler-vs-envsync.vue
│   │       ├── dotenvx-vs-envsync.vue
│   │       └── sops-vs-envsync.vue
│   └── server/
│       └── tsconfig.json                # Nuxt internal (generated/managed)
├── build/
│   ├── fetch_latest_release_for_website.py
│   └── website/
│       ├── prepare_nuxt_site.sh         # CI helper: fetch release json, copy install.sh, nojekyll
│       └── generate_sitemap.py          # optional future step
└── website/                             # legacy static source (removed after cutover)
```

Notes:
- Keep `public/assets` filenames stable to avoid broken links and preserve cacheability.
- Keep root-level static files (`CNAME`, `robots.txt`) in `public/` for exact output parity.

---

## 5) Route Mapping Plan (URL compatibility)

All existing public paths should remain unchanged:

| Existing URL | Nuxt page file |
|---|---|
| `/` | `pages/index.vue` |
| `/download` | `pages/download/index.vue` |
| `/installation` | `pages/installation/index.vue` |
| `/installation/quickstart` | `pages/installation/quickstart.vue` |
| `/installation/trusted-peers` | `pages/installation/trusted-peers.vue` |
| `/installation/secure-peers` | `pages/installation/secure-peers.vue` |
| `/usage` | `pages/usage/index.vue` |
| `/modes` | `pages/modes/index.vue` |
| `/security` | `pages/security/index.vue` |
| `/comparisons` | `pages/comparisons/index.vue` |
| `/comparisons/vault-vs-envsync` | `pages/comparisons/vault-vs-envsync.vue` |
| `/comparisons/infisical-vs-envsync` | `pages/comparisons/infisical-vs-envsync.vue` |
| `/comparisons/doppler-vs-envsync` | `pages/comparisons/doppler-vs-envsync.vue` |
| `/comparisons/dotenvx-vs-envsync` | `pages/comparisons/dotenvx-vs-envsync.vue` |
| `/comparisons/sops-vs-envsync` | `pages/comparisons/sops-vs-envsync.vue` |

---

## 6) Componentization Plan (remove duplication)

### Shared layout

- `layouts/default.vue`
  - wraps every page with:
    - `<SiteHeader />`
    - `<main><slot /></main>`
    - `<SiteFooter />`

### Header/footer decomposition

- `components/layout/SiteHeader.vue`
  - brand link
  - top-level nav
  - GitHub CTA
- `components/layout/InstallDropdown.vue`
  - install menu links used by header nav
- `components/layout/SiteFooter.vue`
  - footer brand/description
  - grouped links (Install, Docs, Community, etc.)

This is the main duplication fix: one source for nav/footer used by all routes.

### Shared content blocks

- `HeroSection`, `CtaBanner`, `ComparisonCard`, `TrustBadges`, `ModesComparisonTable`
  - extract repeated visual sections that appear in homepage/comparison/installation pages

### Data-driven links/content

- `data/nav.ts` and `data/footer.ts` own link metadata once, consumed by header/footer components.
- `data/comparisons.ts` owns comparison-card metadata.
- Use typed interfaces for all data objects.

---

## 7) Metadata/SEO Parity Plan

- For each page, migrate title/description/canonical/OpenGraph/Twitter metadata using:
  - `useHead(...)`
  - helper composable `useSiteMeta.ts`
- Preserve existing canonical URLs.
- Preserve `robots.txt` and `CNAME`.
- Keep `sitemap.xml` initially as static file in `public/`, then optionally automate generation later.

---

## 8) Download page dynamic data plan

Current behavior depends on `website/download/latest-release.json`.

Nuxt migration approach:

1. Keep generating `latest-release.json` in CI using existing script.
2. Place output at `website-app/public/download/latest-release.json`.
3. In `pages/download/index.vue`, fetch it client-side with a typed helper (`useLatestRelease.ts`).
4. Preserve selector-driven tip behavior (mode/os/arch filters) with Vue reactive state instead of manual DOM toggles.

---

## 9) GitHub Actions workflow changes

Current workflow file: `.github/workflows/website-pages.yml`

### New high-level job flow

1. Checkout repository
2. Setup Node (`actions/setup-node`) with cache (`npm`) using active LTS (Node 20)
3. Install dependencies in `website-app/`
4. Prepare site artifacts:
   - copy `install.sh` to `website-app/public/install.sh`
   - run `python3 build/fetch_latest_release_for_website.py` with output path updated to `website-app/public/download/latest-release.json`
   - keep `website-app/public/.nojekyll` in repo so it is emitted in generated output
5. Run `npm run generate` in `website-app/`
6. Publish `website-app/.output/public` to `gh-pages` via `peaceiris/actions-gh-pages`

### Workflow trigger/path updates

- Replace static-website path trigger:
  - from `website/**`
  - to include `website-app/**`, related build scripts, and workflow file
- Keep release + workflow_dispatch triggers.

### Example deployment deltas

- `publish_dir`: `./website-app/.output/public`
- add Node setup + generate step
- keep `peaceiris/actions-gh-pages` action for continuity

---

## 10) Migration phases (small, safe increments)

Before phase execution, standardize `website-app/package.json` scripts.
`nuxi` is available via the local Nuxt dependency when run through npm scripts (no global install required):

- `"dev": "nuxt dev"`
- `"build": "nuxt build"`
- `"generate": "nuxi generate"`
- `"preview": "nuxt preview"`

### Phase 1: Scaffold & CI dry-run

- Create `website-app/` with Nuxt TypeScript scaffold.
- Copy static assets and baseline CSS.
- Add CI steps for install + generate (without cutting over publish dir yet, if desired for dry run).

### Phase 2: Shared shell first

- Implement `default.vue`, `SiteHeader`, `SiteFooter`, and shared nav/footer data.
- Port homepage and one inner page to validate component approach.

### Phase 3: Complete route parity

- Port all existing routes listed above.
- Add explicit prerender route list in `nuxt.config.ts`.
- Validate link integrity and canonical tags.

### Phase 4: Download parity & release metadata

- Port download selector + tips behavior.
- Wire latest-release JSON fetch to new public path.

### Phase 5: Cutover and cleanup

- Switch workflow `publish_dir` to Nuxt generated output.
- Remove legacy `website/` source files after parity verification.
- Keep static files that remain relevant only in `website-app/public/`.

---

## 11) Validation checklist for migration PR

- Build:
  - `cd website-app && npm ci && npm run generate`
- Verify generated output contains:
  - all expected route directories with `index.html`
  - `install.sh`, `CNAME`, `robots.txt`, `sitemap.xml`, `assets/*`
  - `download/latest-release.json`
- Manual checks:
  - header/footer/nav dropdown identical across pages
  - no broken links for installation and comparison routes
  - metadata tags present on each route
- Deploy check:
  - GitHub Pages workflow publishes generated directory successfully

---

## 12) Risks and mitigations

- **Risk:** Route regressions due to filename/path mismatches  
  **Mitigation:** explicit route mapping table + prerender route list.

- **Risk:** CSS drift during component extraction  
  **Mitigation:** migrate existing stylesheet first, then refactor incrementally.

- **Risk:** Download page behavior regressions  
  **Mitigation:** port selector logic with typed reactive state and test with multiple combinations.

- **Risk:** CI output path mistakes  
  **Mitigation:** verify `publish_dir` and generated file presence before deploy step.

---

## 13) Definition of Done

Migration is complete when:

1. all current public routes render from Nuxt-generated static output,
2. duplicated header/footer HTML is removed and centralized in shared components/layout,
3. website code is TypeScript-based,
4. GitHub Pages deploys Nuxt prerendered output from CI,
5. parity checks for metadata, links, and download-release data pass.
