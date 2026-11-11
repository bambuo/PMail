<template>
  <div class="header-container">
    <div class="header-left">
      <div
        class="mobile-menu-btn"
        @click="globalStatus.mobileDrawerVisible = true"
      >
        <IconMenu />
      </div>
      <div class="mobile-logo">
        <router-link to="/">
          <h1>PMail</h1>
        </router-link>
      </div>
    </div>

    <div class="header-right" />
  </div>

  <a-modal
    v-model:visible="globalStatus.settingsDrawerVisible"
    :title="lang.settings"
    width="min(1200px, 94vw)"
    top="4vh"
    :footer="false"
    modal-class="settings-dialog"
  >
    <a-tabs
      v-model:active-key="activeTab"
      :position="isMobile ? 'top' : 'left'"
      class="settings-tabs"
    >
      <a-tab-pane key="security">
        <template #title>
          <span class="tab-title"><IconLock />{{ lang.security }}</span>
        </template>
        <SecuritySettings />
      </a-tab-pane>
      <a-tab-pane key="group">
        <template #title>
          <span class="tab-title"><IconFolder />{{ lang.group_settings }}</span>
        </template>
        <GroupSettings />
      </a-tab-pane>
      <a-tab-pane key="rule">
        <template #title>
          <span class="tab-title"><IconFilter />{{ lang.rule_setting }}</span>
        </template>
        <RuleSettings />
      </a-tab-pane>
      <a-tab-pane
        v-if="userInfos.is_admin"
        key="user"
      >
        <template #title>
          <span class="tab-title"><IconUser />{{ lang.user_management }}</span>
        </template>
        <UserManagement />
      </a-tab-pane>
      <a-tab-pane key="plugins">
        <template #title>
          <span class="tab-title"><IconApps />{{ lang.plugin_settings }}</span>
        </template>
        <PluginSettings />
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<script setup lang="ts">
import { IconApps, IconFilter, IconFolder, IconLock, IconMenu, IconUser } from '@arco-design/web-vue/es/icon'
import { onMounted, onUnmounted, ref } from 'vue'
import SecuritySettings from '@/components/SecuritySettings.vue'
import GroupSettings from '@/components/GroupSettings.vue'
import RuleSettings from '@/components/RuleSettings.vue'
import UserManagement from '@/components/UserManagement.vue'
import PluginSettings from '@/components/PluginSettings.vue'
import lang from '@/i18n/i18n'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'

const globalStatus = useGlobalStatusStore()
const userInfos = globalStatus.userInfos
const activeTab = ref('security')
const isMobile = ref(window.innerWidth <= 768)

const handleResize = () => {
  isMobile.value = window.innerWidth <= 768
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.header-container {
  display: none;
}

.header-left,
.header-right {
  display: flex;
  align-items: center;
}

.mobile-menu-btn {
  display: none;
  width: 36px;
  height: 36px;
  justify-content: center;
  align-items: center;
  font-size: 18px;
  color: var(--pm-text-secondary);
  cursor: pointer;
  border-radius: 12px;
  border: 1px solid var(--pm-border-color);
  background: var(--pm-surface-glass);
  transition: all 0.2s;
}

.mobile-menu-btn:hover {
  background-color: var(--pm-surface-solid);
  color: var(--pm-text-primary);
}

.mobile-logo {
  display: none;
  margin-left: 12px;
}

.mobile-logo h1 {
  font-size: 20px;
  font-weight: 600;
  color: var(--pm-primary-color);
  margin: 0;
  letter-spacing: -0.02em;
}

.mobile-logo a {
  text-decoration: none;
}

.settings-tabs {
  height: 100%;
}

:global(.settings-dialog .arco-modal-body) {
  padding-top: 4px;
}

:global(.settings-dialog .settings-tabs) {
  height: min(76vh, 780px);
}

:global(.settings-dialog .settings-tabs .arco-tabs-content) {
  height: 100%;
  overflow-y: auto;
}

:global(.settings-dialog .settings-tabs .tab-title) {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

@media (max-width: 768px) {
  :global(.settings-dialog .settings-tabs) {
    height: min(78vh, 640px);
  }
}

@media (max-width: 768px) {
  .header-container {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 10px 10px 0;
    padding: 12px 14px;
    border-radius: var(--pm-radius-lg);
    border: 1px solid var(--pm-border-color);
    background: var(--pm-surface-glass);
    backdrop-filter: blur(12px);
  }
  .mobile-menu-btn {
    display: flex;
  }
  .mobile-logo {
    display: block;
  }
}
</style>
