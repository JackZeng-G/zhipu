<script setup lang="ts">
import { h, ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  DocumentTextOutline,
  ChatbubblesOutline,
  SettingsOutline,
  BookOutline,
  TimeOutline
} from '@vicons/ionicons5'
import { useNASStore } from '@/stores/nas'

const router = useRouter()
const route = useRoute()
const nasStore = useNASStore()
const collapsed = ref(false)

const navItems = [
  { label: '笔记', key: '/', icon: DocumentTextOutline },
  { label: 'AI 助手', key: '/chat', icon: ChatbubblesOutline },
  { label: 'Wiki', key: '/wiki', icon: BookOutline },
  { label: '活动日志', key: '/timeline', icon: TimeOutline },
  { label: '设置', key: '/settings', icon: SettingsOutline },
]

const navItemsReversed = computed(() => {
  // Settings goes to bottom
  return navItems.slice(0, 4)
})

const bottomNav = computed(() => navItems.slice(4))

function renderIcon(icon: any) {
  return () => h(icon)
}

function handleNav(key: string) {
  router.push(key)
}

const statusColor = computed(() => nasStore.connected ? '#4ade80' : '#f87171')
const statusPulse = computed(() => nasStore.loading)

onMounted(() => {
  nasStore.checkStatus()
})
</script>

<template>
  <div class="app-shell">
    <!-- 侧边栏 -->
    <aside class="sidebar" :class="{ 'sidebar-collapsed': collapsed }">
      <!-- 品牌区 -->
      <div class="sidebar-brand" @click="collapsed = !collapsed">
        <div class="brand-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <path d="M4 4h6v6H4V4zm10 0h6v6h-6V4zM4 14h6v6H4v-6zm10 0h6v6h-6v-6z" fill="url(#brandGrad)" opacity="0.8"/>
            <defs><linearGradient id="brandGrad" x1="4" y1="4" x2="20" y2="20"><stop stop-color="#6a0dad"/><stop offset="1" stop-color="#4361ee"/></linearGradient></defs>
          </svg>
        </div>
        <Transition name="fade">
          <span v-if="!collapsed" class="brand-text gradient-text">知识库</span>
        </Transition>
      </div>

      <!-- 导航项 -->
      <nav class="sidebar-nav">
        <div class="nav-section">
          <button
            v-for="item in navItemsReversed"
            :key="item.key"
            class="nav-item"
            :class="{ 'nav-active': route.path === item.key || (item.key !== '/' && route.path.startsWith(item.key)) }"
            @click="handleNav(item.key)"
            :title="item.label"
          >
            <component :is="item.icon" class="nav-icon" />
            <Transition name="fade">
              <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </Transition>
          </button>
        </div>

        <div class="nav-spacer" />

        <div class="nav-section">
          <button
            v-for="item in bottomNav"
            :key="item.key"
            class="nav-item"
            :class="{ 'nav-active': route.path === item.key || (item.key !== '/' && route.path.startsWith(item.key)) }"
            @click="handleNav(item.key)"
            :title="item.label"
          >
            <component :is="item.icon" class="nav-icon" />
            <Transition name="fade">
              <span v-if="!collapsed" class="nav-label">{{ item.label }}</span>
            </Transition>
          </button>

          <!-- NAS 状态 -->
          <div class="nas-status" :title="nasStore.connected ? 'NAS 已连接' : 'NAS 未连接'">
            <span class="status-dot" :class="{ 'status-pulse': statusPulse }" :style="{ background: statusColor }" />
            <Transition name="fade">
              <span v-if="!collapsed" class="status-text">
                {{ nasStore.connected ? 'NAS 已连接' : '未连接' }}
              </span>
            </Transition>
          </div>
        </div>
      </nav>
    </aside>

    <!-- 主内容区 -->
    <main class="main-content">
      <router-view v-slot="{ Component }">
        <Transition name="page" mode="out-in">
          <component :is="Component" />
        </Transition>
      </router-view>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--color-bg-primary);
}

/* === 侧边栏 === */
.sidebar {
  width: 220px;
  display: flex;
  flex-direction: column;
  background: var(--glass-bg);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  border-right: 1px solid var(--glass-border);
  transition: width var(--transition-normal);
  flex-shrink: 0;
  position: relative;
  z-index: 10;
}

.sidebar::before {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  width: 1px;
  height: 100%;
  background: linear-gradient(
    to bottom,
    transparent,
    rgba(106, 13, 173, 0.3) 30%,
    rgba(67, 97, 238, 0.3) 70%,
    transparent
  );
}

.sidebar-collapsed {
  width: 68px;
}

/* === 品牌区 === */
.sidebar-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 20px 16px;
  cursor: pointer;
  user-select: none;
  min-height: 60px;
}

.brand-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 10px;
  background: rgba(106, 13, 173, 0.15);
}

.brand-text {
  font-family: var(--font-heading);
  font-size: 20px;
  font-weight: 600;
  white-space: nowrap;
}

/* === 导航 === */
.sidebar-nav {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 8px;
  overflow: hidden;
}

.nav-section {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-spacer {
  flex: 1;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  border-radius: 10px;
  cursor: pointer;
  font-size: 14px;
  font-family: var(--font-body);
  transition: all var(--transition-fast);
  width: 100%;
  text-align: left;
  position: relative;
  overflow: hidden;
}

.nav-item:hover {
  background: rgba(255, 255, 255, 0.06);
  color: var(--color-text-primary);
}

.nav-item.nav-active {
  background: rgba(106, 13, 173, 0.15);
  color: var(--color-text-primary);
}

.nav-item.nav-active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 60%;
  background: var(--gradient-accent);
  border-radius: 0 3px 3px 0;
}

.nav-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.nav-label {
  white-space: nowrap;
  overflow: hidden;
}

/* === NAS 状态 === */
.nas-status {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  margin-top: 4px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  transition: background var(--transition-fast);
}

.status-dot.status-pulse {
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(74, 222, 128, 0.4); }
  50% { opacity: 0.8; box-shadow: 0 0 0 6px rgba(74, 222, 128, 0); }
}

.status-text {
  font-size: 12px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
}

/* === 主内容 === */
.main-content {
  flex: 1;
  overflow: auto;
  position: relative;
}

/* === 页面过渡 === */
.page-enter-active {
  transition: all 0.25s ease-out;
}
.page-leave-active {
  transition: all 0.2s ease-in;
}
.page-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

/* === Fade 过渡 === */
.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-fast);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
