<script setup lang="ts">
import { ref } from 'vue'
import {
  NInput,
  NButton,
  NCard,
  NSpace,
  NSpin,
  NEmpty,
  NText,
  useMessage
} from 'naive-ui'
import { aiSearch } from '@/api'

interface SearchResult {
  note_id: string
  title: string
  reason: string
}

const message = useMessage()
const query = ref('')
const results = ref<SearchResult[]>([])
const searching = ref(false)
const hasSearched = ref(false)

async function handleSearch() {
  if (!query.value.trim()) return

  searching.value = true
  hasSearched.value = true
  results.value = []

  try {
    const res = await aiSearch(query.value)
    results.value = res.data.results || []
  } catch (e: any) {
    message.error(e.response?.data?.error || e.message || '搜索失败')
  } finally {
    searching.value = false
  }
}
</script>

<template>
  <div class="search-page">
    <div class="search-hero">
      <div class="hero-content">
        <h1 class="hero-title">AI 搜索</h1>
        <p class="hero-subtitle">用自然语言搜索你的知识库</p>
        <div class="search-bar">
          <NInput
            v-model:value="query"
            placeholder="例如：'如何部署K8S集群？' 或 '最近修改的笔记'"
            size="large"
            @keydown.enter="handleSearch"
            class="search-input"
          >
            <template #prefix>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" opacity="0.5">
                <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="2"/>
                <path d="m20 20-4-4" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </template>
          </NInput>
          <NButton
            type="primary"
            size="large"
            :loading="searching"
            @click="handleSearch"
            class="search-btn"
          >
            <template #icon>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
                <path d="m9 18 6-6-6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </template>
          </NButton>
        </div>
      </div>
    </div>

    <div class="search-results">
      <NSpin :show="searching" size="large">
        <TransitionGroup name="result" tag="div" class="results-list">
          <NCard
            v-for="result in results"
            :key="result.note_id"
            class="result-card"
            hoverable
            @click="$router.push(`/?note=${result.note_id}`)"
          >
            <div class="result-header">
              <NText strong style="font-size: 16px" class="result-title">{{ result.title }}</NText>
              <div class="result-match">
                <span class="match-badge">AI匹配</span>
              </div>
            </div>
            <div class="result-reason">{{ result.reason }}</div>
          </NCard>
        </TransitionGroup>

        <NEmpty
          v-if="hasSearched && !searching && results.length === 0"
          description="未找到相关笔记"
          class="empty-results"
        />
      </NSpin>
    </div>
  </div>
</template>

<style scoped>
.search-page {
  min-height: 100vh;
  background: var(--color-bg-primary);
}

/* === Hero 区域 === */
.search-hero {
  padding: 64px 32px 48px;
  text-align: center;
  position: relative;
  overflow: hidden;
}

.search-hero::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 100%;
  background: var(--gradient-primary);
  opacity: 0.1;
  z-index: -1;
}

.hero-content {
  max-width: 800px;
  margin: 0 auto;
}

.hero-title {
  font-family: var(--font-heading);
  font-size: 3rem;
  font-weight: 600;
  background: var(--gradient-accent);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 8px;
  letter-spacing: -0.02em;
}

.hero-subtitle {
  font-size: 1.1rem;
  color: var(--color-text-secondary);
  margin-bottom: 40px;
  max-width: 500px;
  margin-left: auto;
  margin-right: auto;
}

/* === 搜索栏 === */
.search-bar {
  display: flex;
  gap: 12px;
  max-width: 700px;
  margin: 0 auto;
}

.search-input {
  flex: 1;
}

.search-btn {
  border-radius: 10px !important;
  width: 56px;
  height: 56px;
  padding: 0 !important;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* === 结果区 === */
.search-results {
  padding: 0 32px 64px;
  max-width: 900px;
  margin: 0 auto;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 24px;
}

.result-card {
  cursor: pointer;
  transition: all var(--transition-normal);
  border-radius: 12px !important;
  border: 1px solid var(--color-border);
  background: rgba(30, 30, 46, 0.4) !important;
}

.result-card:hover {
  transform: translateY(-2px);
  border-color: var(--color-text-accent);
  box-shadow: 0 6px 20px rgba(106, 13, 173, 0.15);
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.result-title {
  color: var(--color-text-primary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-right: 12px;
}

.result-match {
  flex-shrink: 0;
}

.match-badge {
  display: inline-block;
  padding: 4px 8px;
  background: var(--gradient-accent);
  color: white;
  font-size: 11px;
  font-weight: 500;
  border-radius: 6px;
  line-height: 1;
}

.result-reason {
  color: var(--color-text-secondary);
  font-size: 14px;
  line-height: 1.6;
}

.empty-results {
  margin-top: 64px;
}

/* === 动画 === */
.result-enter-active {
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.result-enter-from {
  opacity: 0;
  transform: translateY(20px);
}
</style>
