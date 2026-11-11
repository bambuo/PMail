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
        <template #actions="{ record }">
          <a-button
            size="small"
            type="text"
            @click="handleEdit(record)"
          >
            Edit
          </a-button>
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
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Notification } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'

interface UserItem {
  ID: number
  Account: string
  Name: string
  Disabled: number
}

interface UserListResponse {
  current_page: number
  total_page: number
  list: UserItem[]
}

const userList = ref<UserItem[]>([])
const currentPage = ref(1)
const totalPage = ref(1)
const userInfoDialog = ref(false)
const editModel = ref<'edit' | 'create'>('edit')
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
  { title: '', slotName: 'actions', width: 100, align: 'right' as const }
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
