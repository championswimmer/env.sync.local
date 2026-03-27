import{_ as i}from"./CAB_tGgj.js";import{u as r}from"./BDsUCjYK.js";import{e as c,c as l,b as s,w as o,f as d,a as e,F as u,o as p,d as a}from"./BBUWz-2P.js";const m={class:"cta-banner"},y={class:"cta-row",style:{"justify-content":"center"}},k=c({__name:"quickstart",setup(h){return r({title:"Quickstart | env-sync",meta:[{name:"description",content:"Get env-sync running in under a minute. One command install, initialize, and start syncing secrets across your machines."},{name:"keywords",content:"env-sync quickstart, dotenv sync setup, fast install, peer-to-peer sync"},{property:"og:title",content:"Quickstart | env-sync"},{property:"og:description",content:"Get env-sync running in under a minute. One command install, initialize, and start syncing secrets across your machines."},{property:"og:type",content:"article"},{property:"og:url",content:"https://envsync.arnav.tech/installation/quickstart"},{property:"og:image",content:"https://envsync.arnav.tech/assets/cover.png"},{name:"twitter:card",content:"summary_large_image"},{name:"twitter:title",content:"Quickstart | env-sync"},{name:"twitter:description",content:"Get env-sync running in under a minute. One command, no accounts, no server."},{name:"twitter:image",content:"https://envsync.arnav.tech/assets/cover.png"}],link:[{rel:"canonical",href:"https://envsync.arnav.tech/installation/quickstart"}]}),(v,n)=>{const t=i;return p(),l(u,null,[s(t,{class:"back-link",to:"/installation"},{default:o(()=>[...n[0]||(n[0]=[a("← Installation guides",-1)])]),_:1}),n[5]||(n[5]=d(`<div class="subpage-hero"><h1>Quickstart</h1><p>Get peer-to-peer secret synchronization running in under a minute. One command, no accounts, no server to provision.</p></div><section class="panel"><h2>1 — Install</h2><pre><code># system-wide install
curl -fsSL https://envsync.arnav.tech/install.sh | sudo bash

# or user-only install (no sudo)
curl -fsSL https://envsync.arnav.tech/install.sh | bash -s -- --user</code></pre><p>The installer detects your platform, downloads the binary, and places it in your <code>PATH</code>. Works on Linux, macOS, and WSL2.</p></section><section class="panel"><h2>2 — Initialize</h2><pre><code># verify installation
env-sync --version

# initialize (uses trusted-owner-ssh mode by default)
env-sync init</code></pre><p>This creates the configuration directory at <code>~/.config/env-sync/</code> and generates any keys needed for your security mode.</p></section><section class="panel"><h2>3 — Add secrets &amp; sync</h2><pre><code># add your first secret
env-sync add OPENAI_API_KEY=&quot;sk-abc123xyz&quot;

# discover other machines on your network
env-sync discover

# sync with all discovered peers
env-sync sync</code></pre></section><section class="panel"><h2>4 — Automate (optional)</h2><pre><code># install a cron job to sync every 30 minutes
env-sync cron --install

# or load secrets automatically on shell startup
# add this to ~/.bashrc or ~/.zshrc:
eval &quot;$(env-sync load 2&gt;/dev/null)&quot;</code></pre></section><section class="panel"><h2>Prerequisites</h2><ul><li><i class="fa-brands fa-golang"></i> <strong>Go 1.24+</strong> — only needed if building from source</li><li><i class="fa-solid fa-terminal"></i> <strong>SSH client</strong> — for <code>trusted-owner-ssh</code> mode (installed by default on most systems)</li><li><i class="fa-solid fa-satellite-dish"></i> <strong>mDNS support</strong> — Avahi on Linux (<code>sudo apt install avahi-daemon avahi-utils</code>), Bonjour on macOS (built-in)</li></ul></section><section class="panel"><h2>Build from source</h2><pre><code>git clone https://github.com/championswimmer/env.sync.local.git
cd env.sync.local
make build
make test
sudo make install        # system-wide
# or: make install-user  # user-only</code></pre></section><section class="panel"><h2>Upgrade &amp; uninstall</h2><pre><code># upgrade — re-run the installer
curl -fsSL https://envsync.arnav.tech/install.sh | sudo bash

# uninstall
env-sync service uninstall
rm -rf ~/.config/env-sync
sudo rm -f /usr/local/bin/env-sync</code></pre></section>`,8)),e("section",m,[n[3]||(n[3]=e("h2",null,"What's next?",-1)),n[4]||(n[4]=e("p",null,"Choose the setup guide that matches your scenario — personal devices or cross-team collaboration.",-1)),e("div",y,[s(t,{class:"btn btn-primary",to:"/installation/trusted-peers"},{default:o(()=>[...n[1]||(n[1]=[a("Trusted Peers guide →",-1)])]),_:1}),s(t,{class:"btn btn-secondary",to:"/installation/secure-peers"},{default:o(()=>[...n[2]||(n[2]=[a("Secure Peers guide →",-1)])]),_:1})])])],64)}}});export{k as default};
