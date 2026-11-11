<template>
  <div class="sidebar-main">
    <div class="brand">
      <router-link
        to="/"
        class="brand-link"
      >
        <span class="brand-dot" />
        <span class="brand-text">PMail</span>
      </router-link>
    </div>
    <div class="search-box">
      <a-input
        v-model="searchQuery"
        :placeholder="lang.search"
        allow-clear
        class="custom-search"
      >
        <template #prefix>
          <IconSearch />
        </template>
      </a-input>
    </div>

    <div class="menu-container">
      <a-menu
        :selected-keys="[activeGroup]"
        class="sidebar-menu"
        @menu-item-click="handleMenuSelect"
      >
        <a-menu-item
          v-for="item in data"
          :key="item.tag"
        >
          <span class="menu-label">{{ item.label }}</span>
        </a-menu-item>
      </a-menu>
    </div>
    <div
      v-if="isLogin"
      class="sidebar-footer"
    >
      <div
        class="settings-btn"
        @click="openSettings"
      >
        <IconSettings />
        <span class="settings-label">{{ lang.settings }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { IconSearch, IconSettings } from '@arco-design/web-vue/es/icon'
import useGroupStore from '@/stores/group'
import lang from '@/i18n/i18n'
import { http } from '@/utils/axios'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'

interface GroupNode {
  tag: string
  label: string
  children?: GroupNode[]
}

const groupStore = useGroupStore()
const globalStatus = useGlobalStatusStore()
const isLogin = computed(() => globalStatus.isLogin)
const router = useRouter()
const data = ref<GroupNode[]>([])
const searchQuery = ref('')
const activeGroup = ref(groupStore.tag)

watch(
  () => groupStore.tag,
  (newVal) => {
    activeGroup.value = newVal
  }
)

http.get<GroupNode[]>('/api/group').then((res) => {
  if (res.data) {
    const list: GroupNode[] = []
    const traverse = (items: GroupNode[]) => {
      items.forEach((node) => {
        list.push(node)
        if (node.children) traverse(node.children)
      })
    }
    traverse(res.data)
    data.value = list
  }
})

const handleMenuSelect = (index: string) => {
  const selected = data.value.find((item) => item.tag === index)
  if (selected) {
    groupStore.name = selected.label
    groupStore.tag = selected.tag
    router.push({ name: 'list' })
  }
}

const openSettings = () => {
  if (Object.keys(globalStatus.userInfos).length === 0) {
    globalStatus.init(() => {
      globalStatus.settingsDrawerVisible = true
    })
  } else {
    globalStatus.settingsDrawerVisible = true
  }
}
</script>

<style scoped>
.sidebar-main {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: linear-gradient(180deg, var(--pm-sidebar-grad-start) 0%, var(--pm-sidebar-grad-end) 100%);
  animation: pm-rise-in 0.4s var(--pm-ease-out);
}

.brand {
  padding: 18px 20px 8px;
}

.brand-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.brand-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--pm-primary-color);
  box-shadow: 0 0 0 6px rgba(0, 113, 227, 0.14);
  animation: pm-soft-pulse 2.6s ease-in-out infinite;
}

.brand-text {
  font-size: 19px;
  font-weight: 600;
  color: var(--pm-text-primary);
  letter-spacing: -0.02em;
}

.search-box {
  padding: 12px 18px 16px;
}

.menu-container {
  flex-grow: 1;
  overflow-y: auto;
  padding: 0 10px;
}

.sidebar-menu {
  background: transparent;
}

.sidebar-menu :deep(.arco-menu-item) {
  height: 42px;
  line-height: 42px;
  border-radius: 999px;
  margin-bottom: 4px;
  font-weight: 500;
}

.menu-label {
  font-size: 14px;
}

.sidebar-footer {
  margin-top: auto;
  padding: 14px 16px 18px;
  border-top: 1px solid var(--pm-border-color);
}

.settings-btn {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  font-size: 14px;
  font-weight: 500;
  color: var(--pm-text-secondary);
  border-radius: 14px;
  background: var(--pm-surface-glass);
  border: 1px solid var(--pm-border-color);
  cursor: pointer;
  transition: all 0.2s ease;
}

.settings-btn :deep(.arco-icon) {
  font-size: 18px;
}

.settings-btn:hover {
  background-color: var(--pm-bg-hover);
  color: var(--pm-text-primary);
  transform: translateY(-1px);
}
</style>
