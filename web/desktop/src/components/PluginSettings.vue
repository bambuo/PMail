<template>
  <div class="settings-card">
    <template v-if="isAdmin">
      <div class="settings-header">
        <div class="header-row">
          <div>
            <h3>{{ lang.wasm_plugins }}</h3>
            <p class="settings-desc">
              {{ lang.wasm_plugins_desc }}
            </p>
          </div>
          <div class="header-actions">
            <input
              ref="uploadInput"
              type="file"
              accept=".tgz,application/gzip,application/x-gzip"
              class="hidden-input"
              @change="onUploadChange"
            >
            <a-button
              type="text"
              :loading="uploading"
              :disabled="!enabled"
              @click="uploadInput?.click()"
            >
              <template #icon>
                <IconUpload />
              </template>
              {{ lang.plugin_upload }}
            </a-button>
            <a-button
              type="text"
              :loading="loading"
              @click="load"
            >
              <template #icon>
                <IconRefresh />
              </template>
              {{ lang.plugin_refresh }}
            </a-button>
          </div>
        </div>
      </div>

      <a-alert
        v-if="!enabled"
        :title="lang.wasm_disabled"
        type="info"
        :closable="false"
        show-icon
      />

      <a-table
        v-else
        :data="plugins"
        :columns="columns"
        :loading="loading"
        :pagination="false"
        :scroll="{ x: 900 }"
        :expandable="{ width: 40, icon: expandIcon }"
        row-key="id"
        class="plugin-table"
      >
        <template #name="{ record }">
          <div class="plugin-name">
            {{ record.name || record.id }}
          </div>
          <div class="plugin-id">
            {{ record.id }}
          </div>
        </template>
        <template #status="{ record }">
          <a-tag
            :color="statusColor(record.status)"
            size="small"
          >
            {{ statusText(record.status) }}
          </a-tag>
        </template>
        <template #calls="{ record }">
          <span>{{ record.callCount }}</span>
          <span class="muted"> / {{ record.errorCount }}</span>
        </template>
        <template #actions="{ record }">
          <div class="actions">
            <a-button
              v-if="canApprove(record.status)"
              type="text"
              size="small"
              @click="openApprove(record)"
            >
              {{ lang.plugin_approve }}
            </a-button>
            <a-button
              v-if="canEnable(record.status)"
              type="text"
              size="small"
              @click="act('enable', record)"
            >
              {{ lang.plugin_enable }}
            </a-button>
            <a-button
              v-if="record.status === 'enabled'"
              type="text"
              size="small"
              @click="act('disable', record)"
            >
              {{ lang.plugin_disable }}
            </a-button>
            <a-button
              v-if="record.status === 'enabled'"
              type="text"
              size="small"
              @click="act('reload', record)"
            >
              {{ lang.plugin_reload }}
            </a-button>
            <a-button
              v-if="record.status !== 'rejected'"
              type="text"
              status="danger"
              size="small"
              @click="act('reject', record)"
            >
              {{ lang.plugin_reject }}
            </a-button>
          </div>
        </template>
        <template #expand-row="{ record }">
          <div class="plugin-detail">
            <div class="detail-section">
              <div class="detail-label">
                {{ lang.plugin_hooks }}
              </div>
              <div class="chips">
                <a-tag
                  v-for="hook in record.hooks"
                  :key="hook"
                  size="small"
                >
                  {{ hookLabel(hook) }}
                </a-tag>
                <span
                  v-if="!record.hooks || record.hooks.length === 0"
                  class="muted"
                >—</span>
              </div>
            </div>
            <div class="detail-section">
              <div class="detail-label">
                {{ lang.plugin_granted }}
              </div>
              <div class="chips">
                <a-tag
                  v-for="cap in record.grantedCapabilities"
                  :key="cap"
                  size="small"
                  color="green"
                >
                  {{ capLabel(cap) }}
                </a-tag>
                <span
                  v-if="!record.grantedCapabilities || record.grantedCapabilities.length === 0"
                  class="muted"
                >—</span>
              </div>
            </div>
          </div>
        </template>
        <template #empty>
          <div class="muted">
            {{ lang.plugin_no_wasm }}
          </div>
        </template>
      </a-table>

      <a-modal
        v-model:visible="approveVisible"
        :title="lang.plugin_approve_title"
        :width="520"
        :ok-text="lang.submit"
        :cancel-text="lang.cancel"
        :ok-loading="submitting"
        @ok="submitApprove"
      >
        <p class="dialog-desc">
          {{ lang.plugin_approve_desc }}
        </p>
        <div class="approval-hint">
          <IconInfoCircle class="hint-icon" />
          <div>
            <div class="hint-title">
              {{ lang.plugin_min_caps }}
            </div>
            <div class="chips">
              <a-tag
                v-for="cap in currentPlugin?.capabilities || []"
                :key="cap"
                size="small"
                color="arcoblue"
              >
                {{ capLabel(cap) }}
              </a-tag>
              <span
                v-if="!(currentPlugin?.capabilities || []).length"
                class="muted"
              >{{ lang.plugin_min_caps_none }}</span>
            </div>
            <div class="hint-desc">
              {{ lang.plugin_min_caps_hint }}
            </div>
          </div>
        </div>
        <div class="dialog-section">
          <div class="dialog-label">
            {{ lang.plugin_approve_hooks }}
          </div>
          <a-checkbox-group
            v-model="selectedHooks"
            class="approval-options"
          >
            <a-checkbox
              v-for="hook in currentPlugin?.hooks || []"
              :key="hook"
              :value="hook"
            >
              {{ hookLabel(hook) }}
            </a-checkbox>
          </a-checkbox-group>
        </div>
        <div class="dialog-section">
          <div class="dialog-label">
            {{ lang.plugin_approve_caps }}
          </div>
          <a-checkbox-group
            v-if="(currentPlugin?.capabilities || []).length"
            v-model="selectedCaps"
          >
            <a-checkbox
              v-for="cap in currentPlugin?.capabilities || []"
              :key="cap"
              :value="cap"
            >
              {{ capLabel(cap) }}
            </a-checkbox>
          </a-checkbox-group>
          <span
            v-else
            class="muted"
          >—</span>
        </div>
      </a-modal>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconDown, IconInfoCircle, IconRefresh, IconRight, IconUpload } from '@arco-design/web-vue/es/icon'
import { http } from '@/utils/axios'
import lang from '@/i18n/i18n'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'
import type { ApiResponse } from '@/types/api'

interface PluginInfo {
  id: string
  name?: string
  version: string
  status: string
  hooks: string[]
  capabilities: string[]
  grantedHooks?: string[]
  grantedCapabilities?: string[]
  callCount: number
  errorCount: number
}

interface PluginListData {
  enabled: boolean
  plugins: PluginInfo[]
}

const globalStatus = useGlobalStatusStore()
const isAdmin = computed(() => Boolean(globalStatus.userInfos.is_admin))

const plugins = ref<PluginInfo[]>([])
const enabled = ref(false)
const loading = ref(false)
const uploading = ref(false)
const uploadInput = ref<HTMLInputElement>()
const submitting = ref(false)
const approveVisible = ref(false)
const currentPlugin = ref<PluginInfo | null>(null)
const selectedHooks = ref<string[]>([])
const selectedCaps = ref<string[]>([])

const columns = [
  { title: lang.plugin_name, slotName: 'name', width: 190 },
  { title: lang.plugin_version, dataIndex: 'version', width: 90 },
  { title: lang.plugin_status, slotName: 'status', width: 110 },
  { title: lang.plugin_calls, slotName: 'calls', width: 110 },
  { title: lang.plugin_actions, slotName: 'actions', width: 250 }
]

const hookLabels: Record<string, string> = {
  SendBefore: lang.plugin_hook_send_before,
  SendAfter: lang.plugin_hook_send_after,
  ReceiveParseBefore: lang.plugin_hook_receive_parse_before,
  ReceiveParseAfter: lang.plugin_hook_receive_parse_after,
  ReceiveSaveAfter: lang.plugin_hook_receive_save_after,
  GetName: lang.plugin_hook_get_name,
  SettingsHtml: lang.plugin_hook_settings_html
}

const capLabels: Record<string, string> = {
  log: lang.plugin_cap_log,
  'email.read': lang.plugin_cap_email_read,
  'email.write': lang.plugin_cap_email_write,
  http: lang.plugin_cap_http
}

const hookLabel = (hook: string) => hookLabels[hook] ?? hook
const capLabel = (cap: string) => capLabels[cap] ?? cap

const expandIcon = (expanded: boolean) => h(expanded ? IconDown : IconRight, { class: 'expand-icon' })

const statusMap: Record<string, { key: string; color: string }> = {
  pending: { key: 'status_pending', color: 'orange' },
  approved: { key: 'status_approved', color: 'blue' },
  enabled: { key: 'status_enabled', color: 'green' },
  disabled: { key: 'status_disabled', color: 'gray' },
  rejected: { key: 'status_rejected', color: 'red' },
  error: { key: 'status_error', color: 'red' }
}

const statusText = (status: string) => lang[statusMap[status]?.key] || status
const statusColor = (status: string) => statusMap[status]?.color || 'gray'
const canApprove = (status: string) => status === 'pending' || status === 'rejected'
const canEnable = (status: string) => status === 'approved' || status === 'disabled' || status === 'error'

const onUploadChange = (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.tgz')) {
    Message.warning(lang.plugin_upload_tgz_only)
    return
  }
  if (file.size > 32 * 1024 * 1024) {
    Message.warning(lang.plugin_upload_too_large)
    return
  }
  uploading.value = true
  const form = new FormData()
  form.append('file', file)
  http
    .postForm<PluginInfo>('/api/plugin/manage/upload', form)
    .then((res) => {
      if (res.errorNo !== 0) {
        Message.error(res.errorMsg || lang.plugin_upload_failed)
        return
      }
      Message.success(lang.plugin_upload_success)
      load()
    })
    .catch(() => Message.error(lang.plugin_upload_failed))
    .finally(() => {
      uploading.value = false
    })
}

const load = async () => {
  if (!isAdmin.value) return
  loading.value = true
  try {
    const res = await http.get<PluginListData>('/api/plugin/manage/list')
    if (res.errorNo === 0 && res.data) {
      enabled.value = Boolean(res.data.enabled)
      plugins.value = res.data.plugins ?? []
    } else if (res.errorNo) {
      Message.error(res.errorMsg || lang.plugin_action_failed)
    }
  } catch {
    Message.error(lang.plugin_action_failed)
  } finally {
    loading.value = false
  }
}

const openApprove = (row: PluginInfo) => {
  currentPlugin.value = row
  selectedHooks.value = [...(row.grantedHooks ?? [])]
  selectedCaps.value = [...(row.grantedCapabilities ?? [])]
  approveVisible.value = true
}

const submitApprove = async () => {
  if (!currentPlugin.value) return
  submitting.value = true
  try {
    const res = await http.post<string>('/api/plugin/manage/approve', {
      pluginId: currentPlugin.value.id,
      hooks: selectedHooks.value,
      capabilities: selectedCaps.value
    })
    handleResult(res, lang.plugin_approve_success)
  } catch {
    Message.error(lang.plugin_action_failed)
  } finally {
    submitting.value = false
  }
}

const act = async (action: string, row: PluginInfo) => {
  try {
    const res = await http.post<string>(`/api/plugin/manage/${action}`, { pluginId: row.id })
    handleResult(res, lang.plugin_action_success)
  } catch {
    Message.error(lang.plugin_action_failed)
  }
}

const handleResult = (res: ApiResponse<unknown>, okMessage: string) => {
  if (res.errorNo === 0) {
    Message.success(okMessage)
    approveVisible.value = false
    load()
  } else {
    Message.error(res.errorMsg || lang.plugin_action_failed)
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

.header-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.hidden-input {
  display: none;
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

.plugin-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 8px 4px;
}

.expand-icon {
  font-size: 14px;
  color: var(--pm-text-secondary);
}

.detail-section {
  min-width: 220px;
}

.detail-label {
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--pm-text-secondary);
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

.approval-hint {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid var(--pm-border-color);
  border-radius: var(--pm-radius-sm);
  background: var(--pm-bg-secondary);
}

.hint-icon {
  flex-shrink: 0;
  margin-top: 2px;
  color: rgb(var(--primary-6));
}

.hint-title {
  margin-bottom: 6px;
  font-weight: 600;
  color: var(--pm-text-primary);
}

.hint-desc {
  margin-top: 8px;
  color: var(--pm-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.dialog-section {
  margin-bottom: 16px;
}

.dialog-label {
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--pm-text-primary);
}

.approval-options {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px 16px;
  width: 100%;
}

.approval-options :deep(.arco-checkbox) {
  margin: 0;
  padding-left: 0;
  white-space: nowrap;
  align-items: center;
}

.approval-options :deep(.arco-checkbox-text) {
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
