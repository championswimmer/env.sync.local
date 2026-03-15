<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { useStatusStore } from '@/stores/status'
import { useToast } from '@/composables/useToast'

const settings = useSettingsStore()
const status = useStatusStore()
const toast = useToast()

const showModeSwitch = ref(false)
const newMode = ref('')
const pruneOldMaterial = ref(false)
const cronInterval = ref(30)
const showInitialize = ref(false)
const initEncrypted = ref(false)

onMounted(async () => {
  await Promise.all([
    settings.fetchMode(),
    settings.fetchAvailableModes(),
    settings.fetchCron(),
    settings.fetchConfigPaths(),
    settings.checkInitialized(),
  ])
})

async function switchMode() {
  try {
    await settings.setMode(newMode.value, pruneOldMaterial.value)
    toast.success(`Mode switched to ${newMode.value}`)
    showModeSwitch.value = false
  } catch (e) {
    toast.error('Mode switch failed: ' + e)
  }
}

async function installCron() {
  try {
    await settings.installCron(cronInterval.value)
    toast.success('Cron job installed')
  } catch (e) {
    toast.error('Cron install failed: ' + e)
  }
}

async function removeCron() {
  try {
    await settings.removeCron()
    toast.success('Cron job removed')
  } catch (e) {
    toast.error('Cron removal failed: ' + e)
  }
}

async function initEnvSync() {
  try {
    await settings.initialize(initEncrypted.value)
    toast.success('env-sync initialized')
    showInitialize.value = false
    await status.fetchStatus()
  } catch (e) {
    toast.error('Initialization failed: ' + e)
  }
}

async function startServer() {
  try {
    // @ts-expect-error Wails bindings
    const configuredPort = await window.go.main.ServiceMgmtService.GetServerPort()
    // @ts-expect-error Wails bindings
    await window.go.main.ServiceMgmtService.StartServer(configuredPort, true)
    toast.success('Server started')
    await status.fetchStatus()
    if (!status.serverStatus?.running) {
      for (let attempt = 0; attempt < 4; attempt++) {
        await new Promise((resolve) => setTimeout(resolve, 250))
        await status.fetchStatus()
        if (status.serverStatus?.running) {
          break
        }
      }
    }
  } catch (e) {
    toast.error('Failed to start server: ' + e)
  }
}

async function stopServer() {
  try {
    // @ts-expect-error Wails bindings
    await window.go.main.ServiceMgmtService.StopServer()
    toast.success('Server stopped')
    await status.fetchStatus()
  } catch (e) {
    toast.error('Failed to stop server: ' + e)
  }
}
</script>

<template>
  <div class="settings-view">
    <div class="section-header">
      <div>
        <h1 class="section-title">Settings</h1>
        <p class="section-subtitle">Configuration &amp; preferences</p>
      </div>
    </div>

    <!-- Initialization -->
    <div class="card" style="margin-top: 16px" v-if="!settings.initialized">
      <div class="card-header">
        <span class="card-title">⚠️ Setup Required</span>
      </div>
      <p class="text-muted" style="margin-bottom: 12px">
        env-sync has not been initialized on this machine.
      </p>
      <button class="btn btn-primary" @click="showInitialize = true">Initialize env-sync</button>
    </div>

    <!-- Mode Settings -->
    <div class="card" style="margin-top: 16px">
      <div class="card-header">
        <span class="card-title">🛡️ Security Mode</span>
        <button class="btn btn-secondary btn-sm" @click="showModeSwitch = true">Change</button>
      </div>
      <div class="setting-info" v-if="settings.currentMode">
        <div class="info-row">
          <span class="text-muted">Current Mode</span>
          <span class="badge badge-info">{{ settings.currentMode.current }}</span>
        </div>
        <div class="info-row">
          <span class="text-muted">Description</span>
          <span>{{ settings.currentMode.description }}</span>
        </div>
        <div class="info-row" v-if="settings.currentMode.transport">
          <span class="text-muted">Transport</span>
          <span class="mono">{{ settings.currentMode.transport }}</span>
        </div>
        <div class="info-row" v-if="settings.currentMode.encryption">
          <span class="text-muted">Encryption</span>
          <span class="mono">{{ settings.currentMode.encryption }}</span>
        </div>
      </div>
    </div>

    <!-- Server -->
    <div class="card" style="margin-top: 16px">
      <div class="card-header">
        <span class="card-title">🖥️ Server</span>
      </div>
      <div class="setting-info" v-if="status.serverStatus">
        <div class="info-row">
          <span class="text-muted">Status</span>
          <span class="badge" :class="status.serverStatus.running ? 'badge-success' : 'badge-error'">
            {{ status.serverStatus.running ? '● Running' : '● Stopped' }}
          </span>
        </div>
        <div class="info-row" v-if="status.serverStatus.running">
          <span class="text-muted">Port</span>
          <span class="mono">{{ status.serverStatus.port }}</span>
        </div>
      </div>
      <div class="setting-actions">
        <button
          class="btn btn-primary btn-sm"
          v-if="!status.serverStatus?.running"
          @click="startServer"
        >
          Start Server
        </button>
        <button
          class="btn btn-danger btn-sm"
          v-if="status.serverStatus?.running"
          @click="stopServer"
        >
          Stop Server
        </button>
      </div>
    </div>

    <!-- Cron -->
    <div class="card" style="margin-top: 16px">
      <div class="card-header">
        <span class="card-title">⏰ Cron Schedule</span>
      </div>
      <div class="setting-info">
        <div class="info-row">
          <span class="text-muted">Status</span>
          <span class="badge" :class="settings.cronInfo?.installed ? 'badge-success' : 'badge-warning'">
            {{ settings.cronInfo?.installed ? 'Installed' : 'Not Installed' }}
          </span>
        </div>
        <div class="info-row" v-if="settings.cronInfo?.installed">
          <span class="text-muted">Interval</span>
          <span>{{ settings.cronInfo.interval }} minutes</span>
        </div>
      </div>
      <div class="setting-actions" style="margin-top: 12px">
        <div class="inline-form" v-if="!settings.cronInfo?.installed">
          <label class="text-muted" style="font-size: 13px">Every</label>
          <input class="input" style="width: 80px" type="number" v-model="cronInterval" min="5" max="1440" />
          <label class="text-muted" style="font-size: 13px">minutes</label>
          <button class="btn btn-primary btn-sm" @click="installCron">Install</button>
        </div>
        <button class="btn btn-danger btn-sm" v-else @click="removeCron">Remove Cron</button>
      </div>
    </div>

    <!-- Config Paths -->
    <div class="card" style="margin-top: 16px" v-if="settings.configPaths">
      <div class="card-header">
        <span class="card-title">📂 Paths</span>
      </div>
      <div class="setting-info">
        <div class="info-row">
          <span class="text-muted">Config Dir</span>
          <span class="mono">{{ settings.configPaths.configDir }}</span>
        </div>
        <div class="info-row">
          <span class="text-muted">Secrets File</span>
          <span class="mono">{{ settings.configPaths.secretsFile }}</span>
        </div>
        <div class="info-row">
          <span class="text-muted">Keys Dir</span>
          <span class="mono">{{ settings.configPaths.keysDir }}</span>
        </div>
        <div class="info-row">
          <span class="text-muted">Backup Dir</span>
          <span class="mono">{{ settings.configPaths.backupDir }}</span>
        </div>
      </div>
    </div>

    <!-- About -->
    <div class="card" style="margin-top: 16px">
      <div class="card-header">
        <span class="card-title">ℹ️ About</span>
      </div>
      <div class="setting-info">
        <div class="info-row">
          <span class="text-muted">Version</span>
          <span class="mono">{{ settings.version }}</span>
        </div>
      </div>
    </div>

    <!-- Mode Switch Modal -->
    <div class="modal-overlay" v-if="showModeSwitch" @click.self="showModeSwitch = false">
      <div class="modal">
        <h2 class="modal-title">Switch Security Mode</h2>
        <div class="form-group">
          <label class="form-label">New Mode</label>
          <select class="input" v-model="newMode">
            <option value="" disabled>Select mode...</option>
            <option v-for="m in settings.availableModes" :key="m.current" :value="m.current">
              {{ m.current }} – {{ m.description }}
            </option>
          </select>
        </div>
        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="pruneOldMaterial" />
            Prune old mode material
          </label>
          <p class="text-muted" style="font-size: 12px; margin-top: 4px">
            Remove keys and config from the old mode
          </p>
        </div>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showModeSwitch = false">Cancel</button>
          <button class="btn btn-primary" @click="switchMode" :disabled="!newMode">Switch Mode</button>
        </div>
      </div>
    </div>

    <!-- Initialize Modal -->
    <div class="modal-overlay" v-if="showInitialize" @click.self="showInitialize = false">
      <div class="modal">
        <h2 class="modal-title">Initialize env-sync</h2>
        <p class="text-muted" style="margin-bottom: 12px">
          This will create the secrets file and generate encryption keys.
        </p>
        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="initEncrypted" />
            Enable encryption (recommended)
          </label>
        </div>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showInitialize = false">Cancel</button>
          <button class="btn btn-primary" @click="initEnvSync">Initialize</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  max-width: 800px;
}

.setting-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.info-row span:first-child {
  font-size: 13px;
  min-width: 120px;
}

.setting-actions {
  margin-top: 12px;
}

.inline-form {
  display: flex;
  align-items: center;
  gap: 8px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  cursor: pointer;
}

.form-group {
  margin-bottom: 12px;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  margin-bottom: 4px;
  color: var(--text-secondary);
}
</style>
