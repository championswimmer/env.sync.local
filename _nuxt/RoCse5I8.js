import{_ as r}from"./D1yEI2ZL.js";import{u as a}from"./0zMB5DQv.js";import{e as i,c as p,f as l,a as n,b as t,w as c,F as d,o as y,d as o}from"./iHxvOU8d.js";const v={class:"cta-banner"},m={class:"cta-row",style:{"justify-content":"center"}},b=i({__name:"index",setup(u){return a({title:"Usage guide | env-sync",meta:[{name:"description",content:"Complete env-sync usage reference: manage modes, peers, secrets, sync, backups, cron, and service lifecycle from the CLI."},{property:"og:title",content:"Usage guide | env-sync CLI reference"},{property:"og:description",content:"Complete env-sync CLI reference: manage modes, peers, secrets, sync, backups, cron, and service lifecycle."},{property:"og:type",content:"article"},{property:"og:url",content:"https://envsync.arnav.tech/usage"},{property:"og:image",content:"https://envsync.arnav.tech/assets/cover.png"},{name:"twitter:card",content:"summary_large_image"},{name:"twitter:title",content:"Usage guide | env-sync CLI reference"},{name:"twitter:description",content:"Complete env-sync CLI reference: manage modes, peers, secrets, sync, backups, cron, and service lifecycle."},{name:"twitter:image",content:"https://envsync.arnav.tech/assets/cover.png"},{name:"keywords",content:"env-sync usage, env-sync CLI, secrets sync commands, peer management, dotenv sync guide"}],link:[{rel:"canonical",href:"https://envsync.arnav.tech/usage"}]}),(h,e)=>{const s=r;return y(),p(d,null,[e[4]||(e[4]=l(`<div class="subpage-hero"><h1>Usage guide</h1><p>Everything you can do with env-sync — from adding your first secret to managing peer access across machines.</p></div><section class="panel"><h2>Quick commands</h2><p>The most common commands you will use day to day:</p><pre><code>env-sync              # sync secrets from peers
env-sync status       # show current mode, host, peer count
env-sync discover     # list discovered peers on the network
env-sync --help       # full command reference</code></pre></section><section class="panel"><h2>Desktop GUI</h2><p>Prefer a visual workflow? Install <code>env-sync-gui</code> with <code>--all</code> or <code>--gui-only</code> and launch the desktop app alongside the CLI.</p><p>The GUI works on the same <code>~/.config/env-sync/</code> state as the CLI, so secrets, peers, keys, backups, and mode changes stay in sync whichever interface you use.</p><p>Inside the app, the dashboard gives you status at a glance, while dedicated views handle secrets, sync, peers, keys, settings, and logs. For the complete GUI guide, see <a href="https://github.com/championswimmer/env.sync.local/blob/main/docs/GUI.md">GUI.md</a>.</p></section><section class="panel"><h2>Mode management</h2><p>Switch between security modes at any time. Mode switching is non-destructive by default.</p><pre><code>env-sync mode get
env-sync mode set trusted-owner-ssh
env-sync mode set secure-peer
env-sync mode set dev-plaintext-http
env-sync mode set secure-peer --yes
env-sync mode set trusted-owner-ssh --prune-old-material --yes</code></pre></section><section class="panel"><h2>Peer management</h2><p><strong>secure-peer mode</strong> — invitation-based access with explicit approval:</p><pre><code>env-sync peer invite --expires 1h
env-sync peer request-access --to hostname.local --token &lt;token&gt;
env-sync peer list --pending
env-sync peer approve new-host.local
env-sync peer revoke compromised-host.local
env-sync peer trust show hostname.local</code></pre><p><strong>trusted-owner mode</strong> — any SSH-reachable peer can sync without approval.</p></section><section class="panel"><h2>Secret &amp; sync operations</h2><pre><code># manage secrets
env-sync add KEY=&quot;value&quot;
env-sync remove KEY
env-sync list
env-sync show KEY
eval &quot;$(env-sync load 2&gt;/dev/null)&quot;

# sync with peers
env-sync sync
env-sync sync hostname.local
env-sync sync --force-pull hostname.local
env-sync sync --dry-run</code></pre><p><code>--force-pull</code> fully overwrites local secrets from the selected host. A backup is created first.</p></section><section class="panel"><h2>Service, cron &amp; backups</h2><pre><code># background service
env-sync serve -d
env-sync service stop
env-sync service restart
env-sync service uninstall

# automatic sync via cron
env-sync cron --install --interval 30
env-sync cron --show
env-sync cron --remove

# restore from backup
env-sync restore       # list available backups
env-sync restore 1     # restore specific backup</code></pre></section><section class="panel"><h2>Troubleshooting</h2><pre><code># check the log file
tail -f ~/.config/env-sync/logs/env-sync.log

# verbose output
env-sync sync --verbose

# verify connectivity
env-sync discover</code></pre></section>`,8)),n("section",v,[e[2]||(e[2]=n("h2",null,"Need help choosing a mode?",-1)),e[3]||(e[3]=n("p",null,"Learn about the three security models and when to use each one.",-1)),n("div",m,[t(s,{class:"btn btn-primary",to:"/modes"},{default:c(()=>[...e[0]||(e[0]=[o("Mode comparison →",-1)])]),_:1}),t(s,{class:"btn btn-secondary",to:"/security"},{default:c(()=>[...e[1]||(e[1]=[o("Security details",-1)])]),_:1})])])],64)}}});export{b as default};
