<script setup lang="ts">
import { RouterView, useRoute, type RouteRecordName } from 'vue-router'
import HomeHeader from '@/components/HomeHeader.vue'
import HomeAside from '@/components/HomeAside.vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'
import enUS from '@arco-design/web-vue/es/locale/lang/en-us'
import lang from '@/i18n/i18n'
import { useGlobalStatusStore } from '@/stores/useGlobalStatusStore'
import { useSystemArcoTheme } from '@/utils/theme'

const route = useRoute()
const pageName = ref<RouteRecordName | null | undefined>(route.name)
const globalStatus = useGlobalStatusStore()
const arcoLocale = computed(() => (lang.lang === 'zhCn' ? zhCN : enUS))

let disposeTheme: (() => void) | undefined

onMounted(() => {
  disposeTheme = useSystemArcoTheme()
  if (pageName.value !== 'login' && pageName.value !== 'setup') {
    globalStatus.init(() => {})
  }
})

onUnmounted(() => {
  disposeTheme?.()
})

watch(
  () => route.fullPath,
  () => {
    pageName.value = route.name
  }
)
</script>

<template>
  <a-config-provider :locale="arcoLocale">
    <div id="main">
      <HomeHeader v-if="pageName !== 'login' && pageName !== 'setup'" />
      <div id="content">
        <div
          v-if="pageName !== 'login' && pageName !== 'setup'"
          id="aside"
        >
          <HomeAside />
        </div>
        <a-drawer
          v-if="pageName !== 'login' && pageName !== 'setup'"
          v-model:visible="globalStatus.mobileDrawerVisible"
          placement="left"
          :width="243"
          :header="false"
          :footer="false"
          class="mobile-aside-drawer"
        >
          <HomeAside />
        </a-drawer>
        <div
          id="body"
          :class="{ 'full-bleed': pageName === 'login' || pageName === 'setup' }"
        >
          <RouterView v-slot="{ Component }">
            <transition
              name="page-fade"
              mode="out-in"
            >
              <component :is="Component" />
            </transition>
          </RouterView>
        </div>
      </div>
    </div>
  </a-config-provider>
</template>

<style scoped>
#aside {
  width: 264px;
  min-width: 264px;
  max-width: 264px;
  margin: 16px 0 16px 16px;
  background: var(--pm-bg-sidebar);
  border: 1px solid var(--pm-border-color);
  border-radius: var(--pm-radius-xl);
  box-shadow: var(--pm-shadow-sm);
  overflow: hidden;
}

#body {
  width: 100%;
  height: 100%;
  padding: 16px;
  box-sizing: border-box;
  overflow: hidden;
}

#body.full-bleed {
  padding: 0;
}

#content {
  display: flex;
  flex-grow: 1;
  overflow: hidden;
  min-height: 0;
}

#main {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.mobile-aside-drawer :deep(.arco-drawer-body) {
  padding: 0;
}

@media (max-width: 768px) {
  #aside {
    display: none !important;
  }
  #body {
    padding: 10px;
  }
  #body.full-bleed {
    padding: 0;
  }
}
</style>
