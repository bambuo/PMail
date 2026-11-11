<template>
  <div class="settings-card">
    <div class="settings-header">
      <h3>{{ lang.email_folders }}</h3>
      <p class="settings-desc">
        {{ lang.email_folders_desc }}
      </p>
    </div>

    <div class="tree-container">
      <a-tree
        :data="treeData"
        :field-names="{ key: 'key', title: 'label', children: 'children' }"
        default-expand-all
        class="custom-tree"
        @select="handleSelect"
      />
    </div>

    <div class="form-actions">
      <a-space>
        <a-button
          type="primary"
          @click="openAddRoot"
        >
          <template #icon>
            <IconPlus />
          </template>
          {{ lang.add_group }}
        </a-button>
        <a-button
          :disabled="!selectedNode || selectedNode.id <= 0"
          @click="openAddChild"
        >
          <template #icon>
            <IconPlus />
          </template>
          {{ lang.add_sub_group }}
        </a-button>
        <a-button
          status="danger"
          :disabled="!selectedNode || selectedNode.id <= 0"
          @click="removeSelected"
        >
          <template #icon>
            <IconDelete />
          </template>
          {{ lang.delete }}
        </a-button>
      </a-space>
      <p class="group-hint">
        {{ lang.group_delete_hint }}
      </p>
    </div>

    <a-modal
      v-model:visible="addVisible"
      :title="lang.add_group"
      :ok-text="lang.submit"
      :cancel-text="lang.cancel"
      @ok="confirmAdd"
    >
      <a-input
        v-model="addLabel"
        :placeholder="lang.folder_name"
        @press-enter="confirmAdd"
      />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { Message, Modal } from '@arco-design/web-vue'
import { IconDelete, IconPlus } from '@arco-design/web-vue/es/icon'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'

interface GroupNode {
  id: number
  label: string
  parent_id?: number
  tag?: string
  children?: GroupNode[]
  key?: string | number
}

interface GroupTreeNode extends GroupNode {
  key: string
  children: GroupTreeNode[]
}

const data = ref<GroupNode[]>([])
const selectedKey = ref<string | null>(null)
const addVisible = ref(false)
const addLabel = ref('')
const addParentId = ref(0)

const withKeys = (items: GroupNode[]): GroupTreeNode[] =>
  items.map((item) => ({
    ...item,
    key: String(item.id > 0 ? item.id : item.tag ?? item.label),
    children: withKeys(item.children ?? [])
  }))

const treeData = computed<GroupTreeNode[]>(() => withKeys(data.value))

const findNode = (items: GroupTreeNode[], key: string): GroupTreeNode | undefined => {
  for (const item of items) {
    if (item.key === key) return item
    const child = findNode(item.children, key)
    if (child) return child
  }
  return undefined
}

const selectedNode = computed(() => {
  if (selectedKey.value === null) return undefined
  return findNode(treeData.value, selectedKey.value)
})

const loadGroups = () => {
  http.get<GroupNode[]>('/api/group').then((res) => {
    if (res.errorNo === 0) {
      data.value = res.data ?? []
    }
  })
}

loadGroups()

const handleSelect = (keys: Array<string | number>) => {
  selectedKey.value = keys[0] === undefined ? null : String(keys[0])
}

const openAddRoot = () => {
  addParentId.value = 0
  addLabel.value = ''
  addVisible.value = true
}

const openAddChild = () => {
  if (!selectedNode.value) return
  if (selectedNode.value.id <= 0) return
  addParentId.value = selectedNode.value.id
  addLabel.value = ''
  addVisible.value = true
}

const confirmAdd = () => {
  if (!addLabel.value.trim()) return
  http.post('/api/group/add', { name: addLabel.value.trim(), parent_id: addParentId.value }).then((res) => {
    if (res.errorNo !== 0) {
      Message.error(res.errorMsg)
      return
    }
    addVisible.value = false
    loadGroups()
  })
}

const removeSelected = () => {
  const target = selectedNode.value
  if (!target || target.id <= 0) return
  Modal.confirm({
    title: lang.delete,
    content: lang.delete_group_confirm,
    okText: lang.delete,
    cancelText: lang.cancel,
    onOk: () => {
      http.post('/api/group/del', { id: target.id }).then((res) => {
        if (res.errorNo !== 0) {
          Message.error(res.errorMsg)
          return
        }
        Message.success(lang.group_delete_success)
        selectedKey.value = null
        loadGroups()
      })
    }
  })
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

.tree-container {
  background: var(--pm-bg-secondary);
  border: 1px solid var(--pm-border-color);
  border-radius: var(--pm-radius-sm);
  padding: 16px;
  margin-bottom: 24px;
}

.custom-tree {
  background: transparent;
}

.form-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.group-hint {
  margin: 0;
  color: var(--pm-text-secondary);
  font-size: 13px;
}
</style>
