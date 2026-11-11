<template>
  <!-- eslint-disable vue/no-v-html -- 邮件 HTML 已由后端 parsemail 清洗 -->
  <div class="mail-detail-container">
    <div class="mail-detail-header">
      <a-button
        class="back-btn"
        @click="$router.back()"
      >
        <template #icon>
          <IconArrowLeft />
        </template>
      </a-button>
      <div class="action-buttons">
        <a-button @click="handleDelete">
          <template #icon>
            <IconDelete />
          </template>
        </a-button>
      </div>
    </div>

    <div class="mail-detail-content">
      <h1 class="mail-subject">
        {{ detailData.subject }}
      </h1>

      <div class="mail-meta-card">
        <div class="meta-left">
          <div class="avatar-placeholder">
            {{ getInitial(detailData.from_name || detailData.from_address) }}
          </div>
          <div class="meta-info">
            <div class="sender-line">
              <span class="sender-name">{{ detailData.from_name !== '' ? detailData.from_name : detailData.from_address }}</span>
              <span
                v-if="detailData.from_name !== ''"
                class="sender-email"
              >&lt;{{ detailData.from_address }}&gt;</span>
            </div>
            <div class="receivers-line">
              <span class="meta-label">{{ lang.to }}:</span>
              <span
                v-for="(to, index) in tos"
                :key="index"
                class="receiver-chip"
              >
                {{ to.Name !== '' ? to.Name : to.EmailAddress }}<span v-if="index < tos.length - 1">, </span>
              </span>
              <span
                v-if="showCC"
                class="cc-section"
              >
                <span class="meta-label">{{ lang.cc }}:</span>
                <span
                  v-for="(item, index) in ccs"
                  :key="'cc' + index"
                  class="receiver-chip"
                >
                  {{ item.Name !== '' ? item.Name : item.EmailAddress }}<span v-if="index < ccs.length - 1">, </span>
                </span>
              </span>
            </div>
          </div>
        </div>
        <div class="meta-right">
          <div class="mail-date">
            {{ formatDetailDate(detailData.send_date) }}
          </div>
        </div>
      </div>

      <a-divider class="custom-divider" />

      <div class="mail-body">
        <div
          v-if="detailData.html === ''"
          class="body-text"
        >
          {{ detailData.text }}
        </div>
        <div
          v-else
          class="body-html"
          v-html="detailData.html"
        />
      </div>

      <div
        v-if="detailData.attachments && detailData.attachments.length > 0"
        class="attachments-section"
      >
        <a-divider class="custom-divider" />
        <div class="attachments-title">
          {{ lang.attachment }} ({{ detailData.attachments.length }})
        </div>
        <div class="attachments-list">
          <a
            v-for="item in detailData.attachments"
            :key="item.Index"
            class="attachment-card"
            :href="'/attachments/download/' + detailData.id + '/' + item.Index"
          >
            <div class="att-icon"><IconFile /></div>
            <div class="att-name">{{ item.Filename }}</div>
            <div class="att-download"><IconDownload /></div>
          </a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import { IconArrowLeft, IconDelete, IconDownload, IconFile } from '@arco-design/web-vue/es/icon'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'
import useGroupStore from '@/stores/group'

interface MailAddress {
  Name: string
  EmailAddress: string
}

interface Attachment {
  Index: number
  Filename: string
}

interface EmailDetail {
  id: number
  subject: string
  from_name: string
  from_address: string
  to: string
  cc: string
  send_date: string
  html: string
  text: string
  attachments: Attachment[]
}

const route = useRoute()
const router = useRouter()
const groupStore = useGroupStore()
const detailData = ref<Partial<EmailDetail>>({ attachments: [] })
const tos = ref<MailAddress[]>([])
const ccs = ref<MailAddress[]>([])
const showCC = ref(false)

const emailId = Number(route.params.id)

http.post<EmailDetail>('/api/email/detail', { id: emailId }).then((res) => {
  detailData.value = res.data ?? { attachments: [] }
  detailData.value.attachments = res.data?.attachments ?? []
  if (res.data?.to && res.data.to !== '') {
    try {
      tos.value = JSON.parse(res.data.to) as MailAddress[]
    } catch {
      tos.value = []
    }
  }
  if (res.data?.cc && res.data.cc !== '') {
    try {
      ccs.value = JSON.parse(res.data.cc) as MailAddress[]
    } catch {
      ccs.value = []
    }
  }
  showCC.value = ccs.value.length > 0
})

const getInitial = (name?: string) => {
  if (!name) return '?'
  return name.charAt(0).toUpperCase()
}

const formatDetailDate = (dateStr?: string) => {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString([], {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const handleDelete = () => {
  const id = detailData.value.id || emailId
  if (!id) return

  let tag = groupStore.tag
  if (!tag) {
    tag = '{"type":0,"status":-1}'
  }

  let forcedDel = false
  try {
    const groupTag = JSON.parse(tag) as { status?: number }
    forcedDel = groupTag.status === 3
  } catch {
    forcedDel = false
  }

  Modal.confirm({
    title: 'Warning',
    content: lang.del_email_confirm,
    okText: 'OK',
    cancelText: 'Cancel',
    onOk: () => {
      http.post('/api/email/del', { ids: [id], forcedDel }).then((res) => {
        if (res.errorNo === 0) {
          Message.success('Deleted successfully')
          router.push({ name: 'list' })
        } else {
          Message.error(res.errorMsg)
        }
      })
    }
  })
}
</script>

<style scoped>
.mail-detail-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--pm-surface-glass);
  border-radius: var(--pm-radius-xl);
  box-shadow: var(--pm-shadow-md);
  border: 1px solid var(--pm-glass-border);
  backdrop-filter: blur(18px);
  overflow: hidden;
  animation: pm-rise-in 0.34s var(--pm-ease-out);
}

.mail-detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid var(--pm-border-color);
  background: var(--pm-surface-glass-soft);
}

.mail-detail-content {
  flex-grow: 1;
  overflow-y: auto;
  padding: 28px 30px;
}

.mail-subject {
  font-size: 36px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin: 0 0 22px 0;
  line-height: 1.3;
  letter-spacing: -0.02em;
}

.mail-meta-card {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 18px;
  padding: 16px 18px;
  border-radius: var(--pm-radius-lg);
  border: 1px solid var(--pm-border-color);
  background: var(--pm-surface-glass);
}

.meta-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.avatar-placeholder {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--pm-primary-light-3), var(--pm-primary-color));
  color: white;
  display: flex;
  justify-content: center;
  align-items: center;
  font-size: 20px;
  font-weight: 600;
  flex-shrink: 0;
}

.sender-line {
  margin-bottom: 4px;
}

.sender-name {
  font-weight: 600;
  font-size: 16px;
  color: var(--pm-text-primary);
  margin-right: 8px;
}

.sender-email {
  color: var(--pm-text-secondary);
  font-size: 14px;
}

.receivers-line {
  font-size: 13px;
  color: var(--pm-text-secondary);
}

.meta-label {
  color: var(--pm-text-muted);
  margin-right: 4px;
}

.cc-section {
  margin-left: 12px;
}

.meta-right {
  color: var(--pm-text-secondary);
  font-size: 14px;
}

.custom-divider {
  margin: 10px 0 26px 0;
}

.mail-body {
  font-size: 16px;
  line-height: 1.7;
  color: var(--pm-text-primary);
  min-height: 200px;
}

.body-html :deep(img) {
  max-width: 100%;
  height: auto;
}

.body-text {
  white-space: pre-wrap;
}

.attachments-section {
  margin-top: 32px;
}

.attachments-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 16px;
  color: var(--pm-text-primary);
}

.attachments-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.attachment-card {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border: 1px solid var(--pm-border-color);
  border-radius: 14px;
  background-color: var(--pm-surface-muted);
  transition:
    transform 0.2s var(--pm-ease-out),
    box-shadow 0.2s var(--pm-ease-out),
    border-color 0.2s;
  min-width: 200px;
  max-width: 300px;
}

.attachment-card:hover {
  border-color: var(--pm-primary-color);
  background-color: var(--pm-bg-hover);
  box-shadow: var(--pm-shadow-sm);
  transform: translateY(-1px);
}

.att-icon {
  font-size: 24px;
  color: var(--pm-text-secondary);
  margin-right: 12px;
}

.att-name {
  flex-grow: 1;
  font-size: 14px;
  color: var(--pm-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-right: 12px;
}

.att-download {
  color: var(--pm-primary-color);
  font-size: 18px;
}

@media (max-width: 768px) {
  .mail-detail-header {
    padding: 12px 16px;
  }
  .mail-detail-content {
    padding: 20px 16px;
  }
  .mail-subject {
    font-size: 22px;
  }
  .mail-meta-card {
    flex-direction: column;
    gap: 12px;
  }
  .meta-right {
    padding-left: 64px;
  }
  .avatar-placeholder {
    width: 40px;
    height: 40px;
    font-size: 16px;
  }
}
</style>
