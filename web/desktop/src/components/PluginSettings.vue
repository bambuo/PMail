<template>
  <div class="settings-card">
    <template v-if="isAdmin">
      <div class="settings-header">
        <div class="header-row">
          <div>
            <h3>{{ lang.wasm_plugins }}</h3>
            <p class="settings-desc">{{ lang.wasm_plugins_desc }}</p>
          </div>
          <el-button text :icon="Refresh" :loading="loading" @click="load">{{ lang.plugin_refresh }}</el-button>
        </div>
      </div>

      <el-alert v-if="!enabled" :title="lang.wasm_disabled" type="info" :closable="false" show-icon />

      <el-table
          v-else
          v-loading="loading"
          :data="plugins"
          size="small"
          row-key="id"
          class="plugin-table"
          :empty-text="lang.plugin_no_wasm"
      >
        <el-table-column :label="lang.plugin_name" min-width="170">
          <template #default="{ row }">
            <div class="plugin-name">{{ row.name || row.id }}</div>
            <div class="plugin-id">{{ row.id }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="version" :label="lang.plugin_version" width="90" />
        <el-table-column :label="lang.plugin_status" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="lang.plugin_hooks" min-width="150">
          <template #default="{ row }">
            <div class="chips">
              <el-tag v-for="hook in row.hooks" :key="hook" size="small" effect="plain">{{ hook }}</el-tag>
              <span v-if="!row.hooks || row.hooks.length === 0" class="muted">—</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="lang.plugin_granted" min-width="160">
          <template #default="{ row }">
            <div class="chips">
              <el-tag
                  v-for="cap in row.grantedCapabilities"
                  :key="cap"
                  size="small"
                  type="success"
                  effect="plain"
              >{{ cap }}</el-tag>
              <span v-if="!row.grantedCapabilities || row.grantedCapabilities.length === 0" class="muted">—</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="lang.plugin_calls" width="110">
          <template #default="{ row }">
            <span>{{ row.callCount }}</span>
            <span class="muted"> / {{ row.errorCount }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="lang.plugin_actions" width="230">
          <template #default="{ row }">
            <div class="actions">
              <el-button
                  v-if="canApprove(row.status)"
                  link
                  type="primary"
                  size="small"
                  @click="openApprove(row)"
              >{{ lang.plugin_approve }}</el-button>
              <el-button
                  v-if="canEnable(row.status)"
                  link
                  type="success"
                  size="small"
                  @click="act('enable', row)"
              >{{ lang.plugin_enable }}</el-button>
              <el-button
                  v-if="row.status === 'enabled'"
                  link
                  size="small"
                  @click="act('disable', row)"
              >{{ lang.plugin_disable }}</el-button>
              <el-button
                  v-if="row.status === 'enabled'"
                  link
                  size="small"
                  @click="act('reload', row)"
              >{{ lang.plugin_reload }}</el-button>
              <el-button
                  v-if="row.status !== 'rejected'"
                  link
                  type="danger"
                  size="small"
                  @click="act('reject', row)"
              >{{ lang.plugin_reject }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-dialog v-model="approveVisible" :title="lang.plugin_approve_title" width="520px" append-to-body>
        <p class="dialog-desc">{{ lang.plugin_approve_desc }}</p>
        <div class="dialog-section">
          <div class="dialog-label">{{ lang.plugin_approve_hooks }}</div>
          <el-checkbox-group v-model="selectedHooks">
            <el-checkbox v-for="hook in currentPlugin?.hooks || []" :key="hook" :label="hook" :value="hook" />
          </el-checkbox-group>
        </div>
        <div class="dialog-section">
          <div class="dialog-label">{{ lang.plugin_approve_caps }}</div>
          <el-checkbox-group v-if="(currentPlugin?.capabilities || []).length" v-model="selectedCaps">
            <el-checkbox v-for="cap in currentPlugin.capabilities" :key="cap" :label="cap" :value="cap" />
          </el-checkbox-group>
          <span v-else class="muted">—</span>
        </div>
        <template #footer>
          <el-button @click="approveVisible = false">{{ lang.cancel }}</el-button>
          <el-button type="primary" :loading="submitting" @click="submitApprove">{{ lang.submit }}</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { http } from '@/utils/axios'
import lang from '../i18n/i18n'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'

const globalStatus = useGlobalStatusStore()
const isAdmin = computed(() => [1, true].includes(globalStatus.userInfos?.is_admin))

const plugins = ref([])
const enabled = ref(false)
const loading = ref(false)
const submitting = ref(false)
const approveVisible = ref(false)
const currentPlugin = ref(null)
const selectedHooks = ref([])
const selectedCaps = ref([])

const statusMap = {
  pending: { key: 'status_pending', type: 'warning' },
  approved: { key: 'status_approved', type: '' },
  enabled: { key: 'status_enabled', type: 'success' },
  disabled: { key: 'status_disabled', type: 'info' },
  rejected: { key: 'status_rejected', type: 'danger' },
  error: { key: 'status_error', type: 'danger' }
}

function statusText(status) {
  return lang[statusMap[status]?.key] || status
}

function statusType(status) {
  return statusMap[status]?.type || 'info'
}

function canApprove(status) {
  return status === 'pending' || status === 'rejected'
}

function canEnable(status) {
  return status === 'approved' || status === 'disabled' || status === 'error'
}

async function load() {
  if (!isAdmin.value) return
  loading.value = true
  try {
    const res = await http.get('/api/plugin/manage/list')
    if (res.errorNo === 0 && res.data) {
      enabled.value = Boolean(res.data.enabled)
      plugins.value = res.data.plugins || []
    } else if (res.errorNo) {
      ElMessage.error(res.errorMsg || lang.plugin_action_failed)
    }
  } catch {
    ElMessage.error(lang.plugin_action_failed)
  } finally {
    loading.value = false
  }
}

function openApprove(row) {
  currentPlugin.value = row
  selectedHooks.value = [...(row.grantedHooks || [])]
  selectedCaps.value = [...(row.grantedCapabilities || [])]
  approveVisible.value = true
}

async function submitApprove() {
  if (!currentPlugin.value) return
  submitting.value = true
  try {
    const res = await http.post('/api/plugin/manage/approve', {
      pluginId: currentPlugin.value.id,
      hooks: selectedHooks.value,
      capabilities: selectedCaps.value
    })
    handleResult(res, lang.plugin_approve_success)
  } catch {
    ElMessage.error(lang.plugin_action_failed)
  } finally {
    submitting.value = false
  }
}

async function act(action, row) {
  try {
    const res = await http.post(`/api/plugin/manage/${action}`, { pluginId: row.id })
    handleResult(res, lang.plugin_action_success)
  } catch {
    ElMessage.error(lang.plugin_action_failed)
  }
}

function handleResult(res, okMessage) {
  if (res.errorNo === 0) {
    ElMessage.success(okMessage)
    approveVisible.value = false
    load()
  } else {
    ElMessage.error(res.errorMsg || lang.plugin_action_failed)
  }
}

watch(isAdmin, (admin) => {
  if (admin && plugins.value.length === 0) {
    load()
  }
})

onMounted(() => {
  load()
})
</script>

<style scoped>
.settings-card {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.settings-header {
  margin-bottom: 20px;
}

.settings-header h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin: 0 0 8px 0;
}

.header-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.settings-desc {
  font-size: 14px;
  color: var(--pm-text-secondary);
  margin: 0;
}

.plugin-table {
  margin-bottom: 24px;
}

.plugin-name {
  font-weight: 600;
  color: var(--pm-text-primary);
}

.plugin-id {
  margin-top: 2px;
  font-size: 12px;
  color: var(--pm-text-secondary);
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.muted {
  color: var(--pm-text-secondary);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 10px;
}

.dialog-desc {
  margin: 0 0 16px;
  color: var(--pm-text-secondary);
  font-size: 13px;
}

.dialog-section {
  margin-bottom: 16px;
}

.dialog-label {
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--pm-text-primary);
}

</style>
