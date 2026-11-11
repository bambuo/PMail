<template>
  <div class="list-view-container">
    <div class="list-header">
      <div class="header-title">
        <h2>{{ groupStore.name }}</h2>
      </div>
      <div class="header-actions">
        <div class="selection-actions">
          <a-button
            class="action-btn selection-btn"
            :disabled="data.length === 0"
            @click="selectAll"
          >
            <template #icon>
              <IconCheckCircle />
            </template>
            {{ lang.select_all_btn }}
          </a-button>
          <a-button
            class="action-btn selection-btn"
            :disabled="data.length === 0"
            @click="invertSelection"
          >
            <template #icon>
              <IconSwap />
            </template>
            {{ lang.invert_selection_btn }}
          </a-button>
        </div>
        <a-button
          class="action-btn"
          @click="del"
        >
          <template #icon>
            <IconDelete />
          </template>
          {{ lang.del_btn }}
        </a-button>
        <a-button
          class="action-btn"
          @click="markRead"
        >
          <template #icon>
            <IconEye />
          </template>
          {{ lang.read_btn }}
        </a-button>
        <a-dropdown
          trigger="click"
          class="move-dropdown"
        >
          <a-button class="action-btn">
            <template #icon>
              <IconFolder />
            </template>
            {{ lang.move_btn }}
            <IconDown class="dropdown-icon" />
          </a-button>
          <template #content>
            <a-doption
              v-for="group in groupList"
              :key="group.id"
              @click="move(group.id, group.name)"
            >
              {{ group.name }}
            </a-doption>
          </template>
        </a-dropdown>
        <a-button
          type="primary"
          class="compose-btn"
          @click="router.push('/editer')"
        >
          <template #icon>
            <IconEdit />
          </template>
          {{ lang.compose }}
        </a-button>
      </div>
    </div>

    <div class="list-content">
      <a-table
        v-model:selected-keys="selectedKeys"
        :data="data"
        :columns="columns"
        :pagination="false"
        :show-header="false"
        :row-selection="{ type: 'checkbox', showCheckedAll: false }"
        row-key="id"
        class="modern-mail-table"
        @row-click="rowClick"
      >
        <template #status="{ record }">
          <div class="status-indicator">
            <a-badge
              v-if="!record.is_read"
              :dot="true"
              :count="1"
              class="unread-dot"
            >
              <span class="badge-anchor" />
            </a-badge>
            <a-tooltip
              v-if="record.dangerous"
              :content="lang.dangerous"
              position="tl"
            >
              <IconExclamationCircleFill class="danger-icon" />
            </a-tooltip>
            <a-tooltip
              v-if="record.error !== ''"
              :content="record.error"
              position="tl"
            >
              <IconExclamationCircleFill class="danger-icon" />
            </a-tooltip>
          </div>
        </template>
        <template #mail="{ record }">
          <div
            class="mail-row-content"
            :class="{ 'is-unread': !record.is_read }"
          >
            <div class="mail-main-info">
              <div class="mail-sender">
                {{ record.sender.Name !== '' ? record.sender.Name : record.sender.EmailAddress }}
              </div>
              <div class="mail-subject">
                {{ record.title }}
              </div>
              <div class="mail-snippet">
                {{ record.desc }}
              </div>
            </div>
            <div class="mail-meta">
              <div class="mail-date">
                {{ formatShortDate(record.datetime) }}
              </div>
            </div>
          </div>
        </template>
      </a-table>
    </div>

    <div
      v-if="totalPage > 0"
      class="pagination-wrapper"
    >
      <a-pagination
        :current="currentPage"
        :total="totalPage * 15"
        :page-size="15"
        @change="pageChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { TableData } from '@arco-design/web-vue'
import { useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconCheckCircle,
  IconDelete,
  IconDown,
  IconEdit,
  IconExclamationCircleFill,
  IconEye,
  IconFolder,
  IconSwap
} from '@arco-design/web-vue/es/icon'
import useGroupStore from '@/stores/group'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'

interface MailSender {
  Name: string
  EmailAddress: string
}

interface EmailRow {
  id: number
  is_read: boolean
  dangerous: boolean
  error: string
  sender: MailSender
  title: string
  desc: string
  datetime: string
}

interface EmailListData {
  list: EmailRow[]
  total_page: number
}

interface GroupItem {
  id: number
  name: string
}

const router = useRouter()
const groupStore = useGroupStore()
const groupList = ref<GroupItem[]>([])
const data = ref<EmailRow[]>([])
const totalPage = ref(0)
const currentPage = ref(1)
const selectedKeys = ref<Array<string | number>>([])

let tag = groupStore.tag
if (tag === '') {
  tag = '{"type":0,"status":-1}'
}

const columns = [
  { title: '', slotName: 'status', width: 46 },
  { title: '', slotName: 'mail' }
]

watch(
  () => groupStore.tag,
  (newTag) => {
    tag = newTag === '' ? '{"type":0,"status":-1}' : newTag
    data.value = []
    selectedKeys.value = []
    updateList(1)
  }
)

const updateList = (page = 1) => {
  currentPage.value = page
  http.post<EmailListData>('/api/email/list', { tag, page_size: 15, current_page: page }).then((res) => {
    data.value = res.data?.list ?? []
    totalPage.value = res.data?.total_page ?? 0
  })
}

const updateGroupList = () => {
  http.post<GroupItem[]>('/api/group/list').then((res) => {
    groupList.value = res.data ?? []
  })
}

updateList()
updateGroupList()

const rowClick = (record: TableData) => {
  const row = record as unknown as EmailRow
  router.push(`/detail/${row.id}`)
}

const formatShortDate = (dateStr: string) => {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

const selectAll = () => {
  selectedKeys.value = data.value.map((row) => row.id)
}

const invertSelection = () => {
  const selected = new Set(selectedKeys.value)
  selectedKeys.value = data.value.filter((row) => !selected.has(row.id)).map((row) => row.id)
}

const markRead = () => {
  if (selectedKeys.value.length === 0) {
    Message.warning('Select emails first')
    return
  }
  http.post('/api/email/read', { ids: selectedKeys.value }).then((res) => {
    if (res.errorNo === 0) {
      updateList(currentPage.value)
      Message.success('Marked as read')
    } else {
      Message.error(res.errorMsg)
    }
  })
}

const move = (groupId: number, groupName: string) => {
  if (selectedKeys.value.length === 0) {
    Message.warning('Select emails first')
    return
  }
  Modal.confirm({
    title: 'Warning',
    content: lang.move_email_confirm,
    okText: 'OK',
    cancelText: 'Cancel',
    onOk: () => {
      http
        .post('/api/email/move', { group_id: groupId, group_name: groupName, ids: selectedKeys.value })
        .then((res) => {
          if (res.errorNo === 0) {
            updateList(currentPage.value)
            Message.success('Move completed')
          } else {
            Message.error(res.errorMsg)
          }
        })
    }
  })
}

const del = () => {
  if (selectedKeys.value.length === 0) {
    Message.warning('Select emails first')
    return
  }
  const groupTag = JSON.parse(tag) as { status?: number }
  Modal.confirm({
    title: 'Warning',
    content: lang.del_email_confirm,
    okText: 'OK',
    cancelText: 'Cancel',
    onOk: () => {
      http
        .post('/api/email/del', { ids: selectedKeys.value, forcedDel: groupTag.status === 3 })
        .then((res) => {
          if (res.errorNo === 0) {
            updateList(currentPage.value)
            Message.success('Deleted successfully')
          } else {
            Message.error(res.errorMsg)
          }
        })
    }
  })
}

const pageChange = (page: number) => {
  updateList(page)
}
</script>

<style scoped>
.list-view-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 100vh;
  background: var(--pm-surface-glass);
  border: 1px solid var(--pm-glass-border);
  border-radius: var(--pm-radius-xl);
  box-shadow: var(--pm-shadow-md);
  backdrop-filter: blur(16px);
  padding: 18px;
  overflow: hidden;
  animation: pm-rise-in 0.36s var(--pm-ease-out);
}

.list-header {
  flex-shrink: 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
  flex-wrap: wrap;
  gap: 12px;
}

.header-title h2 {
  font-size: 30px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin: 0;
  letter-spacing: -0.02em;
}

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.selection-actions {
  display: flex;
  gap: 8px;
  padding-right: 8px;
  margin-right: 2px;
  border-right: 1px solid var(--pm-border-color);
}

.dropdown-icon {
  margin-left: 4px;
}

.list-content {
  min-height: 0;
  flex: 1;
  overflow: hidden;
  background: var(--pm-surface-glass);
  border-radius: var(--pm-radius-lg);
  border: 1px solid var(--pm-border-color);
  box-shadow: inset 0 1px 0 var(--pm-glass-border);
}

.modern-mail-table {
  width: 100%;
}

.modern-mail-table :deep(.arco-table-tr) {
  cursor: pointer;
  transition: all 0.2s var(--pm-ease-out);
}

.modern-mail-table :deep(.arco-table-td) {
  padding: 13px 0;
}

.status-indicator {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 4px;
}

.badge-anchor {
  display: inline-block;
  width: 6px;
  height: 6px;
}

.danger-icon {
  color: #ef4444;
}

.mail-row-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.mail-main-info {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 12px;
}

.mail-sender {
  width: 172px;
  min-width: 172px;
  font-weight: 600;
  color: var(--pm-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 14px;
}

.mail-subject {
  flex: 0 1 auto;
  max-width: 40%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 500;
}

.mail-snippet {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--pm-text-secondary);
}

.mail-meta {
  min-width: 80px;
  text-align: right;
  padding-right: 16px;
}

.mail-date {
  font-size: 12px;
  color: var(--pm-text-secondary);
  font-weight: 500;
}

.is-unread .mail-sender,
.is-unread .mail-subject {
  font-weight: 700;
  color: var(--pm-text-primary);
}

.is-unread .mail-date {
  color: var(--pm-primary-color);
  font-weight: 700;
}

.pagination-wrapper {
  flex-shrink: 0;
  padding: 10px 0;
  padding-bottom: calc(10px + env(safe-area-inset-bottom));
  margin-top: 14px;
  display: flex;
  justify-content: center;
}

@media (max-width: 768px) {
  .list-view-container {
    height: calc(100dvh - 56px);
    min-height: auto;
    padding: 12px;
  }

  .list-header {
    flex-direction: column;
    align-items: flex-start;
  }
  .header-actions {
    width: 100%;
    overflow-x: auto;
    padding-bottom: 4px;
  }
  .mail-main-info {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  .mail-sender {
    width: 100%;
  }
  .mail-subject {
    max-width: 100%;
  }
  .mail-snippet {
    display: none;
  }
  .mail-row-content {
    align-items: flex-start;
  }
  .mail-meta {
    padding-top: 2px;
  }
}
</style>
