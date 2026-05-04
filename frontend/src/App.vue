<script setup lang="ts">
import { NConfigProvider, NMessageProvider, NGlobalStyle, darkTheme } from 'naive-ui'
import AppLayout from '@/components/AppLayout.vue'
</script>

<template>
  <n-config-provider :theme="darkTheme">
    <n-global-style />
    <n-message-provider>
      <AppLayout />
    </n-message-provider>
  </n-config-provider>
</template>

<style>
/* 全局样式补充 */
.glass-panel {
  background: var(--glass-bg);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid var(--glass-border);
  border-radius: 12px;
}

.gradient-text {
  background: var(--gradient-accent);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-normal);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
