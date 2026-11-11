<template>
  <div class="settings-card">
    <div class="settings-header">
      <h3>{{ lang.account_management }}</h3>
      <p class="settings-desc">
        {{ lang.account_management_desc }}
      </p>
    </div>

    <div class="table-toolbar">
      <a-button
        type="primary"
        size="small"
        @click="createUser"
      >
        <template #icon>
          <IconPlus />
        </template>
        New
      </a-button>
    </div>

    <div class="table-container">
      <a-table
        :data="userList"
        :columns="columns"
        :pagination="false"
        class="modern-table"
      >
        <template #status="{ record }">
          <a-tag
            :color="record.Disabled === 1 ? 'gray' : 'green'"
            size="small"
          >
            {{ record.Disabled === 1 ? lang.disabled : lang.enabled }}
          </a-tag>
        </template>
        <template #mfa="{ record }">
          <a-tag
            :color="record.MfaEnabled ? 'green' : 'gray'"
            size="small"
          >
            {{ record.MfaEnabled ? lang.mfa_enabled : lang.mfa_disabled }}
          </a-tag>
        </template>
        <template #actions="{ record }">
          <div class="row-actions">
            <a-button
              size="small"
              type="text"
              @click="openMfa(record)"
            >
              MFA
            </a-button>
            <a-button
              size="small"
              type="text"
              @click="handleEdit(record)"
            >
              Edit
            </a-button>
          </div>
        </template>
      </a-table>
    </div>

    <div class="pagination-wrapper">
      <a-pagination
        :current="currentPage"
        :total="totalPage * 10"
        :page-size="10"
        @change="handlePageChange"
      />
    </div>

    <a-modal
      v-model:visible="userInfoDialog"
      :title="title"
      :width="450"
      :ok-text="lang.submit"
      :cancel-text="lang.cancel"
      @ok="submit"
    >
      <div class="dialog-content">
        <a-form
          :model="editUserInfo"
          layout="vertical"
        >
          <a-form-item :label="lang.account">
            <a-input
              v-model="editUserInfo.account"
              :disabled="editModel === 'edit'"
            />
          </a-form-item>

          <a-form-item :label="lang.user_name">
            <a-input v-model="editUserInfo.name" />
          </a-form-item>

          <a-form-item :label="lang.password">
            <a-input-password
              v-model="editUserInfo.password"
              :placeholder="lang.resetPwd"
            />
          </a-form-item>

          <a-form-item>
            <div class="status-switch">
              <span class="switch-label">Status</span>
              <a-switch
                v-model="editUserInfo.disabled"
                :checked-text="lang.disabled"
                :unchecked-text="lang.enabled"
                checked-color="#ef4444"
                unchecked-color="#10b981"
              />
            </div>
          </a-form-item>
        </a-form>
      </div>
    </a-modal>

    <a-modal
      v-model:visible="mfaDialogVisible"
      :title="lang.mfa_title"
      :footer="false"
      :width="480"
    >
      <div
        v-if="mfaTarget"
        class="mfa-panel"
      >
        <div class="mfa-target">
          <span class="mfa-target-label">{{ lang.account }}</span>
          <strong>{{ mfaTarget.Account }}</strong>
          <a-tag
            :color="mfaTarget.MfaEnabled ? 'green' : 'gray'"
            size="small"
          >
            {{ mfaTarget.MfaEnabled ? lang.mfa_enabled : lang.mfa_disabled }}
          </a-tag>
          <span
            v-if="mfaTarget.MfaEnabled"
            class="mfa-muted"
          >
            {{ lang.mfa_recovery_remaining }}: {{ mfaTarget.MfaRecoveryCodesRemaining }}
          </span>
        </div>

        <template v-if="!mfaTarget.MfaEnabled">
          <p class="mfa-desc">
            {{ lang.mfa_bind_hint }}
          </p>
          <a-button
            v-if="!mfaSetup"
            type="primary"
            :loading="mfaLoading"
            @click="startMfaSetup"
          >
            {{ lang.mfa_bind }}
          </a-button>
          <template v-else>
            <p class="mfa-desc">
              {{ lang.mfa_scan_desc }}
            </p>
            <img
              :src="mfaSetup.qrDataUrl"
              alt="MFA QR"
              class="mfa-qr"
            >
            <div class="mfa-key-row">
              <span class="mfa-muted">{{ lang.mfa_manual_key }}</span>
              <code>{{ mfaSetup.secret }}</code>
            </div>
            <a-input
              v-model="mfaCode"
              :placeholder="lang.mfa_code_6"
              @press-enter="confirmMfaSetup"
            />
            <a-button
              type="primary"
              long
              :loading="mfaLoading"
              @click="confirmMfaSetup"
            >
              {{ lang.mfa_confirm }}
            </a-button>
          </template>
        </template>

        <template v-else-if="recoveryCodes.length === 0">
          <p class="mfa-desc">
            {{ lang.mfa_reset_desc }}
          </p>
          <a-input-password
            v-model="mfaPassword"
            :placeholder="lang.password"
          />
          <a-button
            status="danger"
            type="outline"
            long
            :loading="mfaLoading"
            @click="resetMfa"
          >
            {{ lang.mfa_unbind }}
          </a-button>
        </template>

        <div
          v-if="recoveryCodes.length > 0"
          class="mfa-recovery"
        >
          <p class="mfa-desc">
            {{ lang.mfa_recovery_desc }}
          </p>
          <p
            v-if="!isMfaSelf"
            class="mfa-desc"
          >
            {{ lang.mfa_admin_recovery_hint }}
          </p>
          <div class="mfa-recovery-list">
            <code
              v-for="code in recoveryCodes"
              :key="code"
            >{{ code }}</code>
          </div>
          <a-button
            type="primary"
            long
            @click="recoveryCodes = []"
          >
            {{ lang.mfa_saved }}
          </a-button>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Message, Notification } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'
import type { MFASetupData } from '@/types/api'

interface UserItem {
  ID: number
  Account: string
  Name: string
  Disabled: number
  MfaEnabled: boolean
  MfaRecoveryCodesRemaining: number
}

interface UserListResponse {
  current_page: number
  total_page: number
  list: UserItem[]
}

const globalStatus = useGlobalStatusStore()
const userList = ref<UserItem[]>([])
const currentPage = ref(1)
const totalPage = ref(1)
const userInfoDialog = ref(false)
const editModel = ref<'edit' | 'create'>('edit')
const mfaDialogVisible = ref(false)
const mfaTarget = ref<UserItem | null>(null)
const mfaSetup = ref<MFASetupData | null>(null)
const mfaCode = ref('')
const mfaPassword = ref('')
const mfaLoading = ref(false)
const recoveryCodes = ref<string[]>([])
const isMfaSelf = computed(() => mfaTarget.value?.Account === globalStatus.userInfos.account)
const editUserInfo = reactive({
  account: '',
  name: '',
  password: '',
  disabled: false
})
const title = ref(lang.editUser)

const columns = [
  { title: 'ID', dataIndex: 'ID', width: 80 },
  { title: lang.account, dataIndex: 'Account', ellipsis: true, tooltip: true },
  { title: lang.user_name, dataIndex: 'Name', ellipsis: true, tooltip: true },
  { title: lang.disabled, dataIndex: 'Disabled', slotName: 'status', width: 120 },
  { title: 'MFA', dataIndex: 'MfaEnabled', slotName: 'mfa', width: 120 },
  { title: '', slotName: 'actions', width: 160, align: 'right' as const }
]

const reflushList = () => {
  http.post<UserListResponse>('/api/user/list', { current_page: currentPage.value, page_size: 10 }).then((res) => {
    userList.value = res.data?.list ?? []
    totalPage.value = res.data?.total_page ?? 1
  })
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  reflushList()
}

const handleEdit = (row: UserItem) => {
  editUserInfo.account = row.Account
  editUserInfo.name = row.Name
  editUserInfo.disabled = row.Disabled === 1
  editUserInfo.password = ''
  editModel.value = 'edit'
  title.value = lang.editUser
  userInfoDialog.value = true
}

const createUser = () => {
  editUserInfo.account = ''
  editUserInfo.name = ''
  editUserInfo.disabled = false
  editUserInfo.password = ''
  editModel.value = 'create'
  title.value = lang.newUser
  userInfoDialog.value = true
}

const openMfa = (row: UserItem) => {
  mfaTarget.value = row
  mfaSetup.value = null
  mfaCode.value = ''
  mfaPassword.value = ''
  recoveryCodes.value = []
  mfaDialogVisible.value = true
}

const startMfaSetup = () => {
  if (!mfaTarget.value) return
  mfaLoading.value = true
  http
    .post<MFASetupData>('/api/mfa/setup', { account: mfaTarget.value.Account })
    .then((res) => {
      if (res.errorNo !== 0) {
        Notification.error({ title: lang.fail, content: res.errorMsg || lang.fail })
        return
      }
      mfaSetup.value = res.data
    })
    .finally(() => {
      mfaLoading.value = false
    })
}

const confirmMfaSetup = () => {
  const target = mfaTarget.value
  if (!target) return
  if (!mfaCode.value.trim()) {
    Message.warning(lang.mfa_code_6)
    return
  }
  mfaLoading.value = true
  http
    .post<{ recoveryCodes: string[] }>('/api/mfa/confirm', {
      account: target.Account,
      code: mfaCode.value.trim()
    })
    .then((res) => {
      if (res.errorNo !== 0) {
        Notification.error({ title: lang.fail, content: res.errorMsg || lang.fail })
        return
      }
      recoveryCodes.value = res.data?.recoveryCodes ?? []
      target.MfaEnabled = true
      target.MfaRecoveryCodesRemaining = recoveryCodes.value.length
      mfaSetup.value = null
      mfaCode.value = ''
      reflushList()
      Notification.success({ title: lang.succ, content: lang.mfa_bind_success })
    })
    .finally(() => {
      mfaLoading.value = false
    })
}

const resetMfa = () => {
  const target = mfaTarget.value
  if (!target) return
  if (!mfaPassword.value) {
    Message.warning(lang.password)
    return
  }
  mfaLoading.value = true
  http
    .post('/api/mfa/reset', { account: target.Account, password: mfaPassword.value })
    .then((res) => {
      if (res.errorNo !== 0) {
        Notification.error({ title: lang.fail, content: res.errorMsg || lang.fail })
        return
      }
      target.MfaEnabled = false
      target.MfaRecoveryCodesRemaining = 0
      mfaPassword.value = ''
      recoveryCodes.value = []
      reflushList()
      Notification.success({ title: lang.succ, content: lang.mfa_unbind_success })
    })
    .finally(() => {
      mfaLoading.value = false
    })
}

const submit = () => {
  const newData: Record<string, string | number> = {
    account: editUserInfo.account,
    username: editUserInfo.name,
    disabled: editUserInfo.disabled ? 1 : 0
  }
  if (editUserInfo.password !== '') {
    newData.password = editUserInfo.password
  }
  const api = editModel.value === 'edit' ? '/api/user/edit' : '/api/user/create'
  if (editModel.value === 'create' && !newData.password) {
    newData.password = ''
  }
  http.post<string>(api, newData).then((res) => {
    Notification[res.errorNo === 0 ? 'success' : 'error']({
      title: res.errorNo === 0 ? lang.succ : lang.fail,
      content: res.errorNo === 0 ? '' : res.data
    })
    if (res.errorNo === 0) {
      reflushList()
      userInfoDialog.value = false
    }
  })
}

reflushList()
</script>

<style scoped>
.settings-card {
  padding: 0;
}

.settings-header {
  margin-bottom: 24px;
}

.settings-header h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin: 0 0 8px 0;
}

.settings-desc {
  font-size: 14px;
  color: var(--pm-text-secondary);
  margin: 0;
}

.table-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.table-container {
  border: 1px solid var(--pm-border-color);
  border-radius: var(--pm-radius-sm);
  overflow: hidden;
  margin-bottom: 24px;
}

.pagination-wrapper {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

.dialog-content {
  padding: 0 8px;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
}

.mfa-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.mfa-target {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.mfa-target-label {
  color: var(--pm-text-secondary);
  font-size: 13px;
}

.mfa-muted {
  color: var(--pm-text-secondary);
  font-size: 13px;
}

.mfa-desc {
  font-size: 14px;
  color: var(--pm-text-secondary);
  margin: 0;
}

.mfa-qr {
  width: 220px;
  height: 220px;
  align-self: center;
  padding: 8px;
  border-radius: 8px;
  background: #fff;
}

.mfa-key-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.mfa-key-row code {
  padding: 4px 8px;
  border-radius: 6px;
  background: var(--pm-bg-secondary);
  word-break: break-all;
}

.mfa-recovery {
  display: flex;
  flex-direction: column;
  gap: 12px;
  border-top: 1px solid var(--pm-border-color);
  padding-top: 16px;
}

.mfa-recovery-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.mfa-recovery-list code {
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--pm-bg-secondary);
  text-align: center;
}

.status-switch {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
  padding: 8px 12px;
  background: var(--pm-bg-secondary);
  border-radius: var(--pm-radius-sm);
  border: 1px solid var(--pm-border-color);
}

.switch-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--pm-text-primary);
}
</style>
