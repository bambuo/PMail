<template>
  <div class="composer-container">
    <div class="composer-header">
      <h2 class="composer-title">
        {{ lang.compose }}
      </h2>
      <div class="composer-actions">
        <a-button
          class="action-btn"
          @click="upload"
        >
          <template #icon>
            <IconAttachment />
          </template>
          {{ lang.add_att }}
        </a-button>
        <a-button
          type="primary"
          class="send-btn"
          @click="send"
        >
          <template #icon>
            <IconSend />
          </template>
          {{ lang.send }}
        </a-button>
        <input
          v-show="false"
          ref="fileRef"
          type="file"
          multiple
          @change="fileChange"
        >
      </div>
    </div>

    <div class="composer-body">
      <a-form
        ref="ruleFormRef"
        :rules="rules"
        :model="ruleForm"
        class="composer-form"
      >
        <div class="compose-row">
          <div class="field-label">
            {{ lang.sender }}
          </div>
          <a-form-item
            field="sender"
            class="flex-grow-item m-0"
          >
            <a-popover
              trigger="click"
              :width="400"
              position="bl"
            >
              <div class="sender-selector">
                <span class="sender-name">{{ ruleForm.nickName }}</span>
                <span class="sender-email">&lt;{{ ruleForm.sender }}@{{ ruleForm.pickDomain }}&gt;</span>
                <IconDown class="arrow-down" />
              </div>
              <template #content>
                <div class="sender-edit-card">
                  <div class="edit-row">
                    <span class="edit-label">Prefix</span>
                    <a-input
                      v-model="ruleForm.sender"
                      :disabled="!globalStatus.userInfos.is_admin"
                      :placeholder="lang.sender_desc"
                    />
                  </div>
                  <div class="edit-row">
                    <span class="edit-label">Domain</span>
                    <a-select
                      v-model="ruleForm.pickDomain"
                      class="w-full"
                    >
                      <a-option
                        v-for="item in ruleForm.domains"
                        :key="item"
                        :value="item"
                      >
                        {{ item }}
                      </a-option>
                    </a-select>
                  </div>
                  <div class="edit-row">
                    <span class="edit-label">{{ lang.nick_name }}</span>
                    <a-input v-model="ruleForm.nickName" />
                  </div>
                </div>
              </template>
            </a-popover>
          </a-form-item>
        </div>

        <div class="compose-row">
          <div class="field-label">
            {{ lang.to }}
          </div>
          <a-form-item
            field="receivers"
            class="flex-grow-item m-0 border-less-select"
          >
            <a-select
              v-model="ruleForm.receivers"
              multiple
              allow-search
              allow-create
              placeholder="Recipients..."
            />
          </a-form-item>
        </div>

        <div
          v-if="ruleForm.cc.length > 0 || showCcBcc"
          class="compose-row"
        >
          <div class="field-label">
            {{ lang.cc }}
          </div>
          <a-form-item
            field="cc"
            class="flex-grow-item m-0 border-less-select"
          >
            <a-select
              v-model="ruleForm.cc"
              multiple
              allow-search
              allow-create
              placeholder="Cc..."
            />
          </a-form-item>
        </div>

        <div
          v-if="ruleForm.bcc.length > 0 || showCcBcc"
          class="compose-row"
        >
          <div class="field-label">
            {{ lang.bcc }}
          </div>
          <a-form-item
            field="bcc"
            class="flex-grow-item m-0 border-less-select"
          >
            <a-select
              v-model="ruleForm.bcc"
              multiple
              allow-search
              allow-create
              placeholder="Bcc..."
            />
          </a-form-item>
        </div>

        <div class="compose-row subject-row">
          <a-form-item
            field="subject"
            class="w-full m-0 border-less-input"
          >
            <a-input
              v-model="ruleForm.subject"
              placeholder="Subject"
            />
            <div
              v-if="!showCcBcc"
              class="cc-bcc-toggle"
              @click="showCcBcc = !showCcBcc"
            >
              Cc/Bcc
            </div>
          </a-form-item>
        </div>

        <div class="editor-wrapper">
          <Toolbar
            class="editor-toolbar"
            :editor="editorRef"
            :default-config="toolbarConfig"
            :mode="mode"
          />
          <Editor
            v-model="valueHtml"
            class="editor-content"
            :default-config="editorConfig"
            :mode="mode"
            @on-created="handleCreated"
          />
        </div>

        <div
          v-if="fileList.length > 0"
          class="attachments-preview"
        >
          <div
            v-for="(item, index) in fileList"
            :key="index"
            class="att-chip"
          >
            <IconFile class="att-icon" />
            <span class="att-name">{{ item.name }}</span>
            <IconClose
              class="att-remove"
              @click="delFile(index)"
            />
          </div>
        </div>
      </a-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@wangeditor-next/editor/dist/css/style.css'
import { onBeforeUnmount, reactive, ref, shallowRef } from 'vue'
import { Message, type FormInstance } from '@arco-design/web-vue'
import { IconAttachment, IconClose, IconDown, IconFile, IconSend } from '@arco-design/web-vue/es/icon'
import lang from '@/i18n/i18n'
import { Editor, Toolbar } from '@wangeditor-next/editor-for-vue'
import { i18nChangeLanguage, type IDomEditor } from '@wangeditor-next/editor'
import { useRouter } from 'vue-router'
import { http } from '@/utils/axios'
import useGroupStore from '@/stores/group'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'

interface MailAddress {
  name: string
  email: string
}

interface AttachmentFile {
  name: string
  data: string | ArrayBuffer | null
}

const router = useRouter()
const groupStore = useGroupStore()
const globalStatus = useGlobalStatusStore()
const showCcBcc = ref(false)

if (lang.lang === 'zhCn') {
  i18nChangeLanguage('zh-CN')
} else {
  i18nChangeLanguage('en')
}

const valueHtml = ref('<p><br></p>')
const toolbarConfig = {}
const editorConfig = {
  MENU_CONF: {
    uploadImage: {
      base64LimitSize: 100 * 1024 * 1024 * 1024
    }
  },
  placeholder: 'Write your message...'
}
const mode = ref<'default' | 'simple'>('default')
const fileRef = ref<HTMLInputElement>()
const ruleFormRef = ref<FormInstance>()
const ruleForm = reactive({
  nickName: '',
  sender: '',
  receivers: [] as string[],
  cc: [] as string[],
  bcc: [] as string[],
  subject: '',
  domains: [] as string[],
  pickDomain: ''
})
const fileList = reactive<AttachmentFile[]>([])

const init = () => {
  if (!globalStatus.userInfos.account) {
    globalStatus.init(() => {
      ruleForm.sender = globalStatus.userInfos.account ?? ''
      ruleForm.domains = globalStatus.userInfos.domains ?? []
      ruleForm.pickDomain = ruleForm.domains[0] ?? ''
      ruleForm.nickName = globalStatus.userInfos.name ?? ''
    })
  } else {
    ruleForm.sender = globalStatus.userInfos.account ?? ''
    ruleForm.domains = globalStatus.userInfos.domains ?? []
    ruleForm.pickDomain = ruleForm.domains[0] ?? ''
    ruleForm.nickName = globalStatus.userInfos.name ?? ''
  }
}
init()

const validateSender = (_value: unknown, callback: (error?: string) => void) => {
  if (ruleForm.sender.trim() === '') {
    callback(lang.err_sender_must)
  } else if (ruleForm.sender.includes('@')) {
    callback(lang.only_prefix)
  } else {
    callback()
  }
}

const checkEmail = (str: string) => /.+@.+\..+/.test(str)

const validateAddresses = (list: string[], callback: (error?: string) => void) => {
  for (const item of list) {
    if (!checkEmail(item)) {
      callback(lang.err_email_format)
      return
    }
  }
  callback()
}

const rules = reactive({
  sender: [{ validator: validateSender }],
  receivers: [{ validator: (_v: unknown, cb: (error?: string) => void) => validateAddresses(ruleForm.receivers, cb) }],
  cc: [{ validator: (_v: unknown, cb: (error?: string) => void) => validateAddresses(ruleForm.cc, cb) }],
  bcc: [{ validator: (_v: unknown, cb: (error?: string) => void) => validateAddresses(ruleForm.bcc, cb) }],
  subject: [{ required: true, message: lang.err_title_must }]
})

const editorRef = shallowRef<IDomEditor>()

onBeforeUnmount(() => {
  editorRef.value?.destroy()
})

const handleCreated = (editor: IDomEditor) => {
  editorRef.value = editor
}

const send = async () => {
  if (!ruleFormRef.value) return
  const errors = await ruleFormRef.value.validate()
  if (errors) return
  if (ruleForm.receivers.length === 0 && ruleForm.cc.length === 0 && ruleForm.bcc.length === 0) {
    Message.warning('Please specify at least one recipient')
    return
  }

  const tos: MailAddress[] = ruleForm.receivers.map((email) => ({ name: '', email }))
  const ccs: MailAddress[] = ruleForm.cc.map((email) => ({ name: '', email }))
  const bccs: MailAddress[] = ruleForm.bcc.map((email) => ({ name: '', email }))
  const text = editorRef.value?.getText() ?? ''

  http
    .post<string>('/api/email/send', {
      from: { name: ruleForm.nickName, email: `${ruleForm.sender}@${ruleForm.pickDomain}` },
      to: tos,
      cc: ccs,
      bcc: bccs,
      subject: ruleForm.subject,
      text,
      html: valueHtml.value,
      attrs: fileList
    })
    .then((res) => {
      if (res.errorNo === 0) {
        Message.success(lang.succ_send)
        groupStore.name = lang.outbox
        groupStore.tag = '{"type":1,"status":-1}'
        router.replace({ name: 'list' })
      } else {
        Message.error(String(res.data))
      }
    })
}

const upload = () => {
  fileRef.value?.click()
}

const fileChange = (event: Event) => {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (!files?.length) return
  for (const file of Array.from(files)) {
    const reader = new FileReader()
    reader.onload = () => {
      fileList.push({ name: file.name, data: reader.result })
    }
    reader.readAsDataURL(file)
  }
}

const delFile = (index: number) => {
  fileList.splice(index, 1)
}
</script>

<style scoped>
.composer-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--pm-surface-glass);
  border-radius: var(--pm-radius-xl);
  box-shadow: var(--pm-shadow-md);
  border: 1px solid var(--pm-glass-border);
  backdrop-filter: blur(16px);
  overflow: hidden;
  animation: pm-rise-in 0.34s var(--pm-ease-out);
}

.composer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--pm-border-color);
  background: var(--pm-surface-glass-soft);
}

.composer-title {
  font-size: 28px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin: 0;
  letter-spacing: -0.02em;
}

.composer-actions {
  display: flex;
  gap: 12px;
}

.composer-body {
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.composer-form {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.composer-form :deep(.arco-form-item-label-col) {
  display: none;
}

.composer-form :deep(.arco-form-item-wrapper-col) {
  flex: 1;
  max-width: 100%;
}

.compose-row {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--pm-border-subtle);
  padding: 6px 20px;
}

.field-label {
  width: 96px;
  min-width: 96px;
  white-space: nowrap;
  color: var(--pm-text-secondary);
  font-weight: 600;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.flex-grow-item {
  flex-grow: 1;
}

.m-0 {
  margin: 0;
}

.w-full {
  width: 100%;
}

.sender-selector {
  display: inline-flex;
  align-items: center;
  padding: 8px 12px;
  border-radius: 12px;
  cursor: pointer;
  background-color: var(--pm-surface-muted);
  border: 1px solid var(--pm-border-color);
  transition: background 0.2s;
}

.sender-selector:hover {
  background-color: var(--pm-surface-solid);
  transform: translateY(-1px);
}

.sender-name {
  font-weight: 600;
  margin-right: 6px;
  color: var(--pm-text-primary);
}

.sender-email {
  color: var(--pm-text-secondary);
  margin-right: 8px;
}

.arrow-down {
  font-size: 12px;
  color: var(--pm-text-muted);
}

.sender-edit-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.edit-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.edit-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--pm-text-secondary);
}

.border-less-select :deep(.arco-select-view) {
  background: transparent;
  border: none;
  box-shadow: none;
}

.border-less-input :deep(.arco-input-wrapper) {
  background: transparent;
  border: none;
  box-shadow: none;
  padding-left: 0;
}

.subject-row {
  position: relative;
}

.cc-bcc-toggle {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  font-size: 13px;
  color: var(--pm-primary-color);
  cursor: pointer;
  font-weight: 500;
}

.cc-bcc-toggle:hover {
  text-decoration: underline;
}

.editor-wrapper {
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  min-height: 300px;
  --w-e-toolbar-bg-color: var(--pm-surface-glass-soft);
  --w-e-toolbar-color: var(--pm-text-primary);
  --w-e-toolbar-active-color: var(--pm-primary-color);
  --w-e-toolbar-active-bg-color: var(--pm-row-hover);
  --w-e-textarea-bg-color: transparent;
  --w-e-textarea-color: var(--pm-text-primary);
  --w-e-textarea-slight-bg-color: transparent;
  --w-e-textarea-slight-color: var(--pm-text-muted);
  --w-e-border-color: var(--pm-border-subtle);
}

.editor-toolbar {
  border-bottom: 1px solid var(--pm-border-subtle);
  background-color: var(--pm-surface-glass-soft) !important;
}

.editor-content {
  flex-grow: 1;
  padding: 14px 20px;
  overflow-y: hidden;
}

.editor-content :deep(.w-e-text-container),
.editor-content :deep(.w-e-scroll),
.editor-content :deep(.w-e-text) {
  background: transparent !important;
  color: var(--pm-text-primary) !important;
}

.editor-content :deep(.w-e-text-placeholder) {
  color: var(--pm-text-muted) !important;
}

.attachments-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding: 14px 20px;
  border-top: 1px solid var(--pm-border-color);
  background: var(--pm-surface-glass-soft);
}

.att-chip {
  display: flex;
  align-items: center;
  background: var(--pm-bg-secondary);
  border: 1px solid var(--pm-border-color);
  padding: 6px 12px;
  border-radius: 999px;
  font-size: 13px;
  box-shadow: var(--pm-shadow-sm);
  transition:
    transform 0.2s var(--pm-ease-out),
    box-shadow 0.2s var(--pm-ease-out);
}

.att-chip:hover {
  transform: translateY(-1px);
  box-shadow: var(--pm-shadow-md);
}

.att-icon {
  margin-right: 6px;
  color: var(--pm-text-secondary);
}

.att-name {
  max-width: 150px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-right: 8px;
}

.att-remove {
  cursor: pointer;
  color: var(--pm-text-muted);
}

.att-remove:hover {
  color: #ef4444;
}

@media (max-width: 768px) {
  .sender-selector {
    padding: 6px;
    max-width: 100%;
    overflow: hidden;
  }
  .sender-email {
    display: none;
  }
  .composer-header {
    padding: 12px 16px;
  }
}
</style>
