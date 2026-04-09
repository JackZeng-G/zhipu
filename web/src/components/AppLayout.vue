<script setup lang="ts">
import { h, ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { NLayout, NLayoutSider, NMenu, NBadge, NText } from 'naive-ui'
import {
  DocumentTextOutline,
  SearchOutline,
  ChatbubblesOutline,
  SettingsOutline
} from '@vicons/ionicons5'
import { useNASStore } from '@/stores/nas'
import type { MenuOption } from 'naive-ui'

const router = useRouter()
const route = useRoute()
const nasStore = useNASStore()
const collapsed = ref(false)

function renderIcon(icon: any) {
  return () => h(icon)
}

const menuOptions: MenuOption[] = [
  {
    label: '\u7B14\u8BB0',
    key: '/',
    icon: renderIcon(DocumentTextOutline)
  },
  {
    label: 'AI \u641C\u7D22',
    key: '/search',
    icon: renderIcon(SearchOutline)
  },
  {
    label: '\u5BF9\u8BDD',
    key: '/chat',
    icon: renderIcon(ChatbubblesOutline)
  },
  {
    label: '\u8BBE\u7F6E',
    key: '/settings',
    icon: renderIcon(SettingsOutline)
  }
]

function handleMenuUpdate(key: string) {
  router.push(key)
}

onMounted(() => {
  nasStore.checkStatus()
})
</script>

<template>
  <n-layout has-sider style="height: 100vh">
    <n-layout-sider
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="200"
      :collapsed="collapsed"
      show-trigger
      @collapse="collapsed = true"
      @expand="collapsed = false"
      :native-scrollbar="false"
      style="display: flex; flex-direction: column"
    >
      <div style="padding: 16px; text-align: center">
        <n-text strong style="font-size: 18px" v-if="!collapsed">\u77E5\u8BC6\u5E93</n-text>
      </div>
      <n-menu
        :options="menuOptions"
        :value="route.path"
        @update:value="handleMenuUpdate"
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="22"
      />
      <div style="flex: 1" />
      <div style="padding: 16px; text-align: center">
        <n-badge :type="nasStore.connected ? 'success' : 'error'" :dot="!collapsed" :processing="nasStore.loading">
          <n-text depth="3" style="font-size: 12px" v-if="!collapsed">
            {{ nasStore.connected ? 'NAS \u5DF2\u8FDE\u63A5' : 'NAS \u672A\u8FDE\u63A5' }}
          </n-text>
        </n-badge>
      </div>
    </n-layout-sider>
    <n-layout>
      <div style="height: 100vh; overflow: auto">
        <router-view />
      </div>
    </n-layout>
  </n-layout>
</template>
