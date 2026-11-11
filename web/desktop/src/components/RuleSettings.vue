<template>
  <div class="settings-card">
    <div class="settings-header">
      <h3>{{ lang.auto_rules }}</h3>
      <p class="settings-desc">
        {{ lang.auto_rules_desc }}
      </p>
    </div>

    <div class="table-container">
      <a-table
        :data="data"
        :columns="columns"
        :pagination="false"
        class="modern-table"
      >
        <template #action="{ record }">
          <a-tag size="small">
            {{ actionName[record.action] }}
          </a-tag>
        </template>
        <template #actions="{ record }">
          <a-space>
            <a-button
              size="small"
              type="text"
              @click="editRule(record)"
            >
              <template #icon>
                <IconEdit />
              </template>
            </a-button>
            <a-popconfirm
              :content="lang.del_rule_confirm"
              @ok="delRule(record.id)"
            >
              <a-button
                size="small"
                status="danger"
                type="text"
              >
                <template #icon>
                  <IconDelete />
                </template>
              </a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </a-table>
    </div>

    <div class="form-actions">
      <a-button
        type="primary"
        class="add-btn"
        @click="dialogVisible = true"
      >
        <template #icon>
          <IconPlus />
        </template>
        {{ lang.new_rule }}
      </a-button>
    </div>

    <a-modal
      v-model:visible="dialogVisible"
      :title="addRuleForm.id > 0 ? 'Edit Rule' : lang.new_rule"
      :width="760"
      :ok-text="lang.submit"
      :cancel-text="lang.cancel"
      @ok="submitRule"
    >
      <div class="dialog-content">
        <a-form
          :model="addRuleForm"
          layout="vertical"
        >
          <div class="form-row">
            <a-form-item
              :label="lang.rule_name"
              class="flex-grow"
            >
              <a-input
                v-model="addRuleForm.name"
                placeholder="Rule Name"
              />
            </a-form-item>
            <a-form-item
              :label="lang.rule_priority"
              class="w-32"
            >
              <a-input-number
                v-model="addRuleForm.sort"
                :min="0"
                class="w-full"
              />
            </a-form-item>
          </div>

          <a-divider class="custom-divider" />
          <div class="section-title">
            {{ lang.rule_desc }} (Conditions)
          </div>

          <div class="rules-list">
            <div
              v-for="(rule, index) in addRuleForm.rules"
              :key="index"
              class="rule-line"
            >
              <a-select
                v-model="rule.field"
                placeholder="Field"
                class="w-32"
              >
                <a-option
                  value="From"
                  :label="lang.from"
                />
                <a-option
                  value="Subject"
                  :label="lang.subject"
                />
                <a-option
                  value="To"
                  :label="lang.to"
                />
                <a-option
                  value="Cc"
                  :label="lang.cc"
                />
                <a-option
                  value="Content"
                  :label="lang.content"
                />
              </a-select>

              <a-select
                v-model="rule.type"
                placeholder="Type"
                class="w-32"
              >
                <a-option
                  value="equal"
                  :label="lang.equal"
                />
                <a-option
                  value="contains"
                  :label="lang.contains"
                />
                <a-option
                  value="regex"
                  :label="lang.regex"
                />
              </a-select>

              <a-input
                v-model="rule.rule"
                placeholder="Value"
                class="flex-grow"
              />
              <a-button
                type="text"
                status="danger"
                @click="removeRuleLine(index)"
              >
                <template #icon>
                  <IconDelete />
                </template>
              </a-button>
            </div>
          </div>
          <div class="add-condition-action">
            <a-button
              size="small"
              type="primary"
              @click="addRule"
            >
              <template #icon>
                <IconPlus />
              </template>
              Add Condition
            </a-button>
          </div>

          <a-divider class="custom-divider" />
          <div class="section-title">
            {{ lang.rule_do }} (Action)
          </div>

          <div class="action-row">
            <a-select
              v-model="addRuleForm.action"
              placeholder="Select Action"
              class="w-48"
              @change="ruleTypeChange"
            >
              <a-option
                :value="READ"
                :label="lang.mark_read"
              />
              <a-option
                :value="MOVE"
                :label="lang.move"
              />
              <a-option
                :value="DELETE"
                :label="lang.delete"
              />
              <a-option
                :value="FORWARD"
                :label="lang.forward"
              />
            </a-select>

            <a-select
              v-if="addRuleForm.action === MOVE"
              v-model="addRuleForm.params"
              placeholder="Select Folder"
              class="flex-grow"
            >
              <a-option
                v-for="gp in groupData.list"
                :key="gp.id"
                :label="gp.name"
                :value="gp.id"
              />
            </a-select>

            <a-input
              v-if="addRuleForm.action === FORWARD"
              v-model="addRuleForm.params"
              class="flex-grow"
              placeholder="Forward Email Address"
            />
          </div>
        </a-form>
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Notification } from '@arco-design/web-vue'
import { IconDelete, IconEdit, IconPlus } from '@arco-design/web-vue/es/icon'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'

interface RuleCondition {
  field: string
  type: string
  rule: string
}

interface RuleItem {
  id: number
  name: string
  sort: number
  rules: RuleCondition[]
  action: number
  params: string
}

interface GroupOption {
  id: string
  name: string
}

interface RuleForm {
  id: number
  name: string
  sort: number
  rules: RuleCondition[]
  action: number | ''
  params: string
}

const data = ref<RuleItem[]>([])
const dialogVisible = ref(false)
const READ = 1
const FORWARD = 2
const DELETE = 3
const MOVE = 4

const actionName: Record<number, string> = {
  [READ]: lang.mark_read,
  [FORWARD]: lang.forward,
  [DELETE]: lang.delete,
  [MOVE]: lang.move
}

const columns = [
  { title: lang.rule_name, dataIndex: 'name', ellipsis: true, tooltip: true },
  { title: lang.rule_do, dataIndex: 'action', slotName: 'action', width: 120 },
  { title: lang.rule_params, dataIndex: 'params', ellipsis: true, tooltip: true },
  { title: lang.rule_priority, dataIndex: 'sort', width: 160 },
  { title: '', slotName: 'actions', width: 100, align: 'right' as const }
]

const groupData = reactive<{ list: GroupOption[] }>({ list: [] })

const addRuleForm = reactive<RuleForm>({
  id: 0,
  name: '',
  sort: 0,
  rules: [{ field: '', type: '', rule: '' }],
  action: '',
  params: ''
})

const init = () => {
  http.post<RuleItem[]>('/api/rule/get').then((res) => {
    data.value = res.data ?? []
  })
}

init()

const reflushGroupInfos = () => {
  http.get<GroupOption[]>('/api/group/list').then((res) => {
    if (res.data != null) {
      groupData.list = res.data.map((item) => ({ ...item, id: String(item.id) }))
    }
  })
}

reflushGroupInfos()

const delRule = (id: number) => {
  http.post<string>('/api/rule/del', { id }).then((res) => {
    Notification[res.errorNo === 0 ? 'success' : 'error']({
      title: res.errorNo === 0 ? lang.succ : lang.fail,
      content: res.data
    })
    init()
  })
}

const editRule = (ruleInfo: RuleItem) => {
  addRuleForm.id = ruleInfo.id
  addRuleForm.name = ruleInfo.name
  addRuleForm.rules = ruleInfo.rules ?? []
  addRuleForm.action = ruleInfo.action
  addRuleForm.params = ruleInfo.params
  addRuleForm.sort = ruleInfo.sort
  dialogVisible.value = true
}

const removeRuleLine = (index: number) => {
  addRuleForm.rules.splice(index, 1)
}

const addRule = () => {
  addRuleForm.rules.push({ field: '', type: '', rule: '' })
}

const resetForm = () => {
  addRuleForm.id = 0
  addRuleForm.name = ''
  addRuleForm.sort = 0
  addRuleForm.rules = [{ field: '', type: '', rule: '' }]
  addRuleForm.action = ''
  addRuleForm.params = ''
}

const submitRule = () => {
  const api = addRuleForm.id > 0 ? '/api/rule/update' : '/api/rule/add'
  http.post<string>(api, addRuleForm).then((res) => {
    if (res.errorNo !== 0) {
      Notification.error({ title: lang.fail, content: res.data })
      return
    }
    init()
    dialogVisible.value = false
    resetForm()
  })
}

const ruleTypeChange = () => {
  addRuleForm.params = ''
}
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

.table-container {
  border: 1px solid var(--pm-border-color);
  border-radius: var(--pm-radius-sm);
  overflow: hidden;
  margin-bottom: 24px;
}

.form-actions {
  display: flex;
  justify-content: flex-start;
}

.dialog-content {
  padding: 0 8px;
}

.form-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.flex-grow {
  flex-grow: 1;
}

.w-32 {
  width: 170px;
  flex-shrink: 0;
}

.w-48 {
  width: 200px;
  flex-shrink: 0;
}

.w-full {
  width: 100%;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--pm-text-primary);
  margin-bottom: 16px;
}

.rules-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.rule-line {
  display: flex;
  gap: 8px;
  align-items: center;
}

.add-condition-action {
  margin-top: 12px;
}

.action-row {
  display: flex;
  gap: 12px;
  align-items: center;
}

.custom-divider {
  margin: 24px 0;
}
</style>
