export default defineNuxtConfig({
  ssr: true,
  compatibilityDate: '2025-05-10',

  site: {
    url: 'https://envsync.arnav.tech',
  },

  css: ['~/assets/css/main.css'],

  app: {
    head: {
      charset: 'utf-8',
      viewport: 'width=device-width, initial-scale=1',
      meta: [
        { name: 'author', content: 'env-sync contributors' },
        { name: 'keywords', content: 'env sync, dotenv sync, secrets management, peer-to-peer, local network, mDNS, SSH, mTLS, AGE encryption, open source' },
        { property: 'og:site_name', content: 'env-sync' },
        { property: 'og:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
        { name: 'twitter:card', content: 'summary_large_image' },
        { name: 'twitter:image', content: 'https://envsync.arnav.tech/assets/cover.png' },
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/assets/favicon.ico' },
        { rel: 'icon', type: 'image/png', sizes: '16x16', href: '/assets/favicon-16x16.png' },
        { rel: 'icon', type: 'image/png', sizes: '32x32', href: '/assets/favicon-32x32.png' },
        { rel: 'icon', type: 'image/png', sizes: '48x48', href: '/assets/favicon-48x48.png' },
        { rel: 'icon', type: 'image/png', sizes: '96x96', href: '/assets/favicon-96x96.png' },
        { rel: 'icon', type: 'image/png', sizes: '192x192', href: '/assets/favicon-192x192.png' },
        { rel: 'icon', type: 'image/png', sizes: '512x512', href: '/assets/favicon-512x512.png' },
        { rel: 'apple-touch-icon', href: '/assets/apple-touch-icon.png' },
        { rel: 'stylesheet', href: 'https://cdn.jsdelivr.net/npm/@fortawesome/fontawesome-free@6/css/all.min.css' },
      ],
      script: [
        {
          innerHTML: `!function(t,e){var o,n,p,r;e.__SV||(window.posthog && window.posthog.__loaded)||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]),t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}(p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host.replace(".i.posthog.com","-assets.i.posthog.com")+"/static/array.js",(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r);var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="fi init Ci Mi ft Fi Ai Ri capture calculateEventProperties Ui register register_once register_for_session unregister unregister_for_session qi getFeatureFlag getFeatureFlagPayload getFeatureFlagResult isFeatureEnabled reloadFeatureFlags updateFlags updateEarlyAccessFeatureEnrollment getEarlyAccessFeatures on onFeatureFlags onSurveysLoaded onSessionId getSurveys getActiveMatchingSurveys renderSurvey displaySurvey cancelPendingSurvey canRenderSurvey canRenderSurveyAsync identify setPersonProperties group resetGroups setPersonPropertiesForFlags resetPersonPropertiesForFlags setGroupPropertiesForFlags resetGroupPropertiesForFlags reset get_distinct_id getGroups get_session_id get_session_replay_url alias set_config startSessionRecording stopSessionRecording sessionRecordingStarted captureException startExceptionAutocapture stopExceptionAutocapture loadToolbar get_property getSessionProperty Hi ji createPersonProfile setInternalOrTestUser Bi Pi Vi opt_in_capturing opt_out_capturing has_opted_in_capturing has_opted_out_capturing get_explicit_consent_status is_capturing clear_opt_in_out_capturing Di debug bt zi getPageViewId captureTraceFeedback captureTraceMetric Si".split(" "),n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);posthog.init('phc_BrfRxNDklIQEZmAK27UZ2PAdnMHjCBPKRB3fWtOsF9c',{api_host:'https://us.i.posthog.com',defaults:'2026-01-30',person_profiles:'identified_only'})`,
        },
      ],
    },
  },

  nitro: {
    preset: 'static',
    prerender: {
      routes: [
        '/',
        '/comparisons',
        '/comparisons/doppler-vs-envsync',
        '/comparisons/dotenvx-vs-envsync',
        '/comparisons/infisical-vs-envsync',
        '/comparisons/sops-vs-envsync',
        '/comparisons/vault-vs-envsync',
        '/download',
        '/installation',
        '/installation/quickstart',
        '/installation/secure-peers',
        '/installation/trusted-peers',
        '/modes',
        '/security',
        '/usage',
      ],
    },
  },
})
