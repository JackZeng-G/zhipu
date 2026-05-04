<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick, defineAsyncComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NCard,
  NButton,
  NSpace,
  NTag,
  NEmpty,
  NSpin,
  NList,
  NListItem,
  NThing,
  NPageHeader,
  NTabs,
  NTabPane,
  NModal,
  NSelect,
  NInput,
  NBadge,
  NAvatar,
  useMessage
} from 'naive-ui'
import { BookOutline, TrashOutline, RefreshOutline, GitNetworkOutline, ListOutline } from '@vicons/ionicons5'
import type { SelectOption } from 'naive-ui'
import { useKnowledgeStore } from '@/stores/knowledge'
import type { WikiConcept, GraphNode, GraphEdge } from '@/stores/knowledge'
import DOMPurify from 'dompurify'
import { deleteWikiConcept, confirmConceptConfidence } from '@/api'
import Sigma2DGraph from '@/components/Sigma2DGraph.vue'
import LocalSigmaGraph from '@/components/LocalSigmaGraph.vue'

const Graph3DExplorer = defineAsyncComponent(() =>
  import('@/components/Graph3DExplorer.vue')
)

const route = useRoute()
const router = useRouter()
const message = useMessage()
const store = useKnowledgeStore()

const activeTab = ref('graph')
const selectedSlug = ref<string | null>(null)
const showDetail = ref(false)
const initialLoading = ref(true)
const graphMode = ref<'2d' | '3d'>('2d')  // 默认 2D，可切换 3D 探索

// Ref to graph component for accessing community info
const graphRef = ref<InstanceType<typeof Sigma2DGraph> | InstanceType<typeof Graph3DExplorer> | null>(null)

const confidenceLabels: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高'
}

const sortedConcepts = computed(() => {
  return [...store.concepts].sort((a, b) => b.source_count - a.source_count)
})

function selectConcept(slug: string) {
  selectedSlug.value = slug
  showDetail.value = true
  store.fetchConcept(slug)
}

async function handleDelete(slug: string) {
  if (!confirm('确定要删除这个概念页面吗？')) return
  try {
    await deleteWikiConcept(slug)
    message.success('已删除')
    if (selectedSlug.value === slug) {
      selectedSlug.value = null
      showDetail.value = false
    }
    await store.fetchConcepts()
    await store.fetchGraph()
  } catch {
    message.error('删除失败')
  }
}

async function handleRefreshGraph() {
  try {
    await store.fetchGraph()
    message.success('图谱已刷新')
  } catch {
    message.error('刷新失败')
  }
}

async function handleConfirmConfidence(slug: string) {
  try {
    await confirmConceptConfidence(slug)
    message.success('已确认为高置信度')
    await store.fetchConcept(slug)
    await store.fetchConcepts()
  } catch (e: any) {
    message.error('确认失败: ' + (e.response?.data?.error || e.message))
  }
}

function formatDate(ts: number) {
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

function renderMarkdown(content: string): string {
  const html = content
    .replace(/^### (.+)$/gm, '<h3 class="wiki-h3">$1</h3>')
    .replace(/^## (.+)$/gm, '<h2 class="wiki-h2">$1</h2>')
    .replace(/^# (.+)$/gm, '<h1 class="wiki-h1">$1</h1>')
    .replace(/```[\s\S]*?```/g, (match) => {
      const code = match.slice(3, -3).trim()
      return `<pre class="wiki-pre"><code>${code}</code></pre>`
    })
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`(.+?)`/g, '<code class="wiki-code">$1</code>')
    .replace(/^- (.+)$/gm, '<li class="wiki-li">$1</li>')
    .replace(/(<li.*<\/li>\n?)+/g, '<ul class="wiki-ul">$&</ul>')
    .replace(/\n/g, '<br>')
  return DOMPurify.sanitize(html)
}

function parseEvolutionLog(log: string): Array<{ action: string; detail: string; [key: string]: any }> {
  try {
    return JSON.parse(log)
  } catch {
    return []
  }
}

function evolutionActionLabel(action: string): string {
  const labels: Record<string, string> = {
    source_added: '新增来源',
    reinforce: '强化',
    revise: '修正',
    contradict: '分歧',
    created: '创建'
  }
  return labels[action] || action
}

// Summarize evolution log: group source_added into one line, keep semantic entries
const evolutionSummary = computed(() => {
  const entries = parseEvolutionLog(store.currentConcept?.evolution_log || '[]')
  if (entries.length === 0) return null

  const sourceAddedCount = entries.filter(e => e.action === 'source_added').length
  const semanticEntries = entries.filter(e => e.action && e.action !== 'source_added')

  return { sourceAddedCount, semanticEntries }
})

// === Filter state ===
const minSourceCount = ref(2)
const minEdgeWeight = ref(1)
const graphSearch = ref('')
const listSearch = ref('')
const hideIsolated = ref(true)

const nodeCountInfo = computed(() => {
  const nodes = store.graph.nodes
  const total = nodes.length
  const filtered = nodes.filter(n => n.source_count >= minSourceCount.value).length
  const edgeTotal = store.graph.edges.length
  const edgeFiltered = store.graph.edges.filter(e => e.weight >= minEdgeWeight.value).length
  return { total, filtered, edgeTotal, edgeFiltered }
})

const filteredConcepts = computed(() => {
  const search = listSearch.value.trim().toLowerCase()
  let list = sortedConcepts.value
  if (search) {
    list = list.filter(c =>
      c.title.toLowerCase().includes(search) ||
      (c.definition && c.definition.toLowerCase().includes(search))
    )
  }
  return list
})

// Event handlers from SigmaGraph
function onGraphSelect(slug: string) {
  selectedSlug.value = slug
  showDetail.value = true
  store.fetchConcept(slug)
}

function onGraphFocus(_slug: string) {
  // 聚焦只由图谱组件内部处理
}

function onGraphUnfocus() {
  // 取消聚焦
}

function onLocalSelect(slug: string) {
  selectConcept(slug)
}

// Keyboard shortcuts
function onKeyDown(e: KeyboardEvent) {
  if ((e.target as HTMLElement).tagName === 'INPUT' || (e.target as HTMLElement).tagName === 'TEXTAREA') return
  if (e.key === 'Escape' && showDetail.value) {
    showDetail.value = false
  }
}

onMounted(async () => {
  await Promise.all([store.fetchConcepts(), store.fetchGraph()])
  initialLoading.value = false

  // Handle direct navigation: /wiki?entity=X
  const entityQuery = route.query.entity as string
  if (entityQuery) {
    const slug = entityQuery.toLowerCase().replace(/[^a-z0-9一-龥]+/g, '-').replace(/^-|-$/g, '')
    if (slug) selectConcept(slug)
  }

  // Handle direct URL: /wiki/:slug
  const slugParam = route.params.slug as string
  if (slugParam) selectConcept(slugParam)

  window.addEventListener('keydown', onKeyDown)
})

watch(() => route.params.slug, (slug) => {
  if (slug) selectConcept(slug as string)
})
</script>

<template>
  <div class="wiki-layout">
    <!-- Loading -->
    <div v-if="initialLoading" class="initial-loading">
      <NSpin size="large" />
      <div class="loading-text">正在加载知识图谱...</div>
    </div>

    <!-- Main content -->
    <div class="wiki-main" v-show="!initialLoading">
      <NTabs v-model:value="activeTab" type="segment" class="wiki-tabs">
        <NTabPane name="graph" tab="图谱">
          <template #tab>
            <GitNetworkOutline style="margin-right: 4px" /> 图谱
          </template>
          <div class="graph-container">
            <NEmpty v-if="store.graph.nodes.length === 0 && !store.graphLoading" description="暂无图谱数据。请先索引笔记生成概念。" />
            <!-- 2D 主视图 -->
            <Sigma2DGraph
              v-if="graphMode === '2d' && store.graph.nodes.length > 0"
              ref="graphRef"
              :nodes="store.graph.nodes"
              :edges="store.graph.edges"
              :selected-slug="selectedSlug"
              :min-source-count="minSourceCount"
              :min-edge-weight="minEdgeWeight"
              :search-query="graphSearch"
              :hide-isolated="hideIsolated"
              @select="onGraphSelect"
              @focus="onGraphFocus"
              @unfocus="onGraphUnfocus"
            />
            <!-- 3D 探索模式 -->
            <Graph3DExplorer
              v-if="graphMode === '3d' && store.graph.nodes.length > 0"
              ref="graphRef"
              :nodes="store.graph.nodes"
              :edges="store.graph.edges"
              :selected-slug="selectedSlug"
              :min-source-count="minSourceCount"
              :min-edge-weight="minEdgeWeight"
              :search-query="graphSearch"
              :hide-isolated="hideIsolated"
              @select="onGraphSelect"
              @focus="onGraphFocus"
              @unfocus="onGraphUnfocus"
            />
            <!-- Loading overlay -->
            <div v-if="store.graphLoading" class="graph-loading">
              <NSpin size="large" />
            </div>
            <!-- Mode switch -->
            <div class="mode-switch" v-if="store.graph.nodes.length > 0">
              <button
                class="mode-btn"
                :class="{ 'mode-active': graphMode === '2d' }"
                @click="graphMode = '2d'"
                title="2D 图谱"
              >2D</button>
              <button
                class="mode-btn"
                :class="{ 'mode-active': graphMode === '3d' }"
                @click="graphMode = '3d'"
                title="3D 探索"
              >3D</button>
            </div>
            <!-- Legend -->
            <div class="graph-legend" v-if="store.graph.nodes.length > 0">
              <div class="legend-communities">
                <div v-for="c in Math.min(graphRef?.communityCount ?? 0, 8)" :key="c" class="legend-community">
                  <span class="legend-dot" :style="{ background: graphRef?.getCommunityColor(c - 1) }" />
                  <span class="legend-label">{{ graphRef?.communityLabels?.get(c - 1) || '' }}</span>
                </div>
              </div>
              <div class="legend-item legend-hint">
                {{ graphMode === '2d' ? '单击聚焦 · 双击详情 · 悬停高亮邻居' : '单击聚焦 · 再次单击取消 · 右键详情' }}
              </div>
            </div>
            <!-- Refresh control -->
            <div class="zoom-controls" v-if="store.graph.nodes.length > 0">
              <button class="zoom-btn refresh-btn" :class="{ 'refreshing': store.graphLoading }" @click="handleRefreshGraph" :disabled="store.graphLoading" title="刷新图谱">
                <RefreshOutline style="width: 16px; height: 16px" />
              </button>
            </div>
            <!-- Filter controls -->
            <div class="graph-filter" v-if="store.graph.nodes.length > 0">
              <div class="filter-row">
                <span class="filter-label">最小来源数</span>
                <div class="filter-buttons">
                  <button
                    v-for="v in [1, 2, 3, 5]" :key="v"
                    class="filter-btn"
                    :class="{ 'filter-active': minSourceCount === v }"
                    @click="minSourceCount = v"
                  >{{ v }}</button>
                </div>
              </div>
              <div class="filter-row">
                <span class="filter-label">最小关联数</span>
                <div class="filter-buttons">
                  <button
                    v-for="v in [1, 2, 3, 5]" :key="v"
                    class="filter-btn"
                    :class="{ 'filter-active': minEdgeWeight === v }"
                    @click="minEdgeWeight = v"
                  >{{ v }}</button>
                </div>
              </div>
              <div class="filter-row">
                <input
                  class="filter-search"
                  v-model="graphSearch"
                  placeholder="搜索概念..."
                />
              </div>
              <div class="filter-row">
                <label class="filter-check">
                  <input type="checkbox" v-model="hideIsolated" />
                  <span class="filter-check-label">隐藏孤立节点</span>
                </label>
              </div>
              <div class="filter-info">
                {{ nodeCountInfo.filtered }} / {{ nodeCountInfo.total }} 概念 · {{ nodeCountInfo.edgeFiltered }} / {{ nodeCountInfo.edgeTotal }} 关联
              </div>
            </div>
          </div>
        </NTabPane>

        <NTabPane name="list" tab="概念列表">
          <template #tab>
            <ListOutline style="margin-right: 4px" /> 列表
          </template>
          <div class="list-container">
            <div class="list-header">
              <input
                class="list-search"
                v-model="listSearch"
                placeholder="搜索概念名称或定义..."
              />
              <span class="list-count">{{ filteredConcepts.length }} / {{ store.concepts.length }} 个概念</span>
            </div>
            <NEmpty v-if="filteredConcepts.length === 0" description="暂无匹配的概念" />
            <div class="concept-cards">
              <div
                v-for="concept in filteredConcepts"
                :key="concept.slug"
                class="concept-card"
                @click="selectConcept(concept.slug)"
              >
                <div class="card-header">
                  <h3 class="card-title">{{ concept.title }}</h3>
                  <NTag
                    size="small"
                    :type="concept.confidence === 'high' ? 'success' : concept.confidence === 'medium' ? 'info' : 'warning'"
                    round
                  >
                    {{ confidenceLabels[concept.confidence] || '低' }}
                  </NTag>
                  <NTag v-if="concept.confidence_pending" size="small" type="warning" round style="margin-left: 4px">
                    待确认
                  </NTag>
                </div>
                <p class="card-definition" v-if="concept.definition">{{ concept.definition }}</p>
                <p class="card-definition" v-else>点击查看或生成概念详情</p>
                <div class="card-footer">
                  <span class="card-sources">{{ concept.source_count }} 个来源笔记</span>
                  <span class="card-date" v-if="concept.updated_at">{{ formatDate(concept.updated_at) }}</span>
                </div>
              </div>
            </div>
          </div>
        </NTabPane>
      </NTabs>

      <!-- Detail panel -->
      <Transition name="slide">
        <div v-if="showDetail" class="concept-detail-panel">
          <div class="detail-header">
            <div>
              <h2 class="detail-title">{{ store.currentConcept?.title || store.graph.nodes.find(n => n.id === selectedSlug)?.label || selectedSlug }}</h2>
              <div class="detail-meta" v-if="store.currentConcept">
                <NTag
                  size="small"
                  :type="store.currentConcept.confidence === 'high' ? 'success' : store.currentConcept.confidence === 'medium' ? 'info' : 'warning'"
                  round
                >
                  {{ confidenceLabels[store.currentConcept.confidence] || '低' }}
                </NTag>
                <NTag v-if="store.currentConcept?.confidence_pending" size="small" type="warning" round>
                  待确认
                </NTag>
                <NButton
                  v-if="store.currentConcept?.confidence_pending"
                  size="tiny"
                  type="success"
                  @click="handleConfirmConfidence(store.currentConcept!.slug)"
                >
                  确认高置信度
                </NButton>
                <span class="detail-sources">{{ store.currentConcept.source_count }} 个来源笔记</span>
              </div>
            </div>
            <NSpace>
              <NButton size="small" quaternary @click="showDetail = false">关闭</NButton>
              <NButton v-if="store.currentConcept" size="small" quaternary type="error" @click="handleDelete(store.currentConcept!.slug)">
                <template #icon><TrashOutline /></template>
              </NButton>
            </NSpace>
          </div>

          <div class="detail-body">
            <NSpin :show="store.conceptsLoading">
              <template v-if="store.currentConcept?.content">
                <!-- Local graph -->
                <div class="local-graph-wrap">
                  <div class="local-graph-title">关联图谱</div>
                  <LocalSigmaGraph
                    :center-slug="selectedSlug || ''"
                    :nodes="store.graph.nodes"
                    :edges="store.graph.edges"
                    @select="onLocalSelect"
                  />
                </div>
                <!-- Contradictions -->
                <div v-if="store.currentConcept.contradictions" class="contradiction-banner">
                  <strong>发现分歧：</strong>{{ store.currentConcept.contradictions }}
                </div>
                <!-- Evolution Log -->
                <div v-if="evolutionSummary && (evolutionSummary.sourceAddedCount > 0 || evolutionSummary.semanticEntries.length > 0)" class="evolution-section">
                  <h4 class="evolution-title">演变记录</h4>
                  <div class="evolution-entries">
                    <div v-if="evolutionSummary.sourceAddedCount > 0" class="evolution-entry evolution-source_added">
                      <span class="evolution-action">{{ evolutionSummary.sourceAddedCount }} 个来源</span>
                      <span class="evolution-detail">已关联笔记</span>
                    </div>
                    <div v-for="(entry, idx) in evolutionSummary.semanticEntries" :key="idx"
                      class="evolution-entry" :class="'evolution-' + (entry.action || 'unknown')">
                      <span class="evolution-action">{{ evolutionActionLabel(entry.action) }}</span>
                      <span class="evolution-detail">{{ entry.detail || '' }}</span>
                    </div>
                  </div>
                </div>
                <div class="markdown-content" v-html="renderMarkdown(store.currentConcept.content)" />
              </template>
              <template v-else>
                <NEmpty description="正在加载概念内容..." v-if="store.conceptsLoading" />
                <NEmpty description="暂无内容，请稍后重试" v-else />
              </template>
            </NSpin>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.wiki-layout {
  height: 100vh;
  background: var(--color-bg-primary);
}

.initial-loading {
  height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.loading-text {
  color: var(--color-text-tertiary);
  font-size: 14px;
}

/* === Main area === */
.wiki-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}

.wiki-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 16px 24px;
  overflow: hidden;
}

.wiki-tabs :deep(.n-tabs-pane-wrapper) {
  flex: 1;
  overflow: hidden;
}

.wiki-tabs :deep(.n-tab-pane) {
  height: 100%;
}

/* === Graph === */
.graph-container {
  position: relative;
  width: 100%;
  height: calc(100vh - 120px);
  min-height: 400px;
  overflow: hidden;
}

.graph-loading {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 15, 26, 0.5);
  z-index: 5;
}

.graph-legend {
  position: absolute;
  bottom: 16px;
  left: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 14px;
  background: rgba(15, 15, 26, 0.9);
  border-radius: 8px;
  border: 1px solid var(--color-border);
  font-size: 12px;
  color: var(--color-text-tertiary);
  backdrop-filter: blur(8px);
}

.legend-communities {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.legend-community {
  display: flex;
  align-items: center;
  gap: 4px;
}

.legend-label {
  font-size: 11px;
  color: var(--color-text-secondary);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.legend-hint {
  color: var(--color-text-tertiary);
  opacity: 0.7;
}

/* === Mode switch === */
.mode-switch {
  position: absolute;
  top: 16px;
  right: 60px;
  display: flex;
  gap: 0;
  border: 1px solid rgba(60, 80, 120, 0.4);
  border-radius: 6px;
  overflow: hidden;
  z-index: 5;
  backdrop-filter: blur(8px);
}

.mode-btn {
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 600;
  border: none;
  background: rgba(15, 15, 26, 0.85);
  color: #7080a0;
  cursor: pointer;
  transition: all 0.15s;
}

.mode-btn:first-child {
  border-right: 1px solid rgba(60, 80, 120, 0.3);
}

.mode-btn:hover {
  color: #a0c0e8;
}

.mode-btn.mode-active {
  background: rgba(59, 130, 246, 0.25);
  color: #fff;
}

.zoom-controls {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  z-index: 5;
}

.zoom-btn {
  width: 36px;
  height: 36px;
  border: 1px solid var(--color-border);
  background: rgba(15, 15, 26, 0.85);
  color: var(--color-text-primary);
  border-radius: 8px;
  font-size: 18px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
  backdrop-filter: blur(8px);
}

.zoom-btn:hover {
  background: rgba(106, 13, 173, 0.3);
  border-color: rgba(106, 13, 173, 0.5);
}

.refresh-btn.refreshing {
  pointer-events: none;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.graph-filter {
  position: absolute;
  top: 16px;
  left: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 14px;
  background: rgba(15, 15, 26, 0.85);
  border-radius: 8px;
  border: 1px solid var(--color-border);
  backdrop-filter: blur(8px);
  z-index: 5;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 11px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
}

.filter-buttons {
  display: flex;
  gap: 3px;
}

.filter-btn {
  padding: 2px 8px;
  font-size: 11px;
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-secondary);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
}

.filter-btn:hover {
  border-color: rgba(106, 13, 173, 0.5);
  color: var(--color-text-primary);
}

.filter-btn.filter-active {
  background: rgba(106, 13, 173, 0.3);
  border-color: rgba(106, 13, 173, 0.6);
  color: #fff;
}

.filter-search {
  width: 100%;
  padding: 4px 8px;
  font-size: 12px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.05);
  color: var(--color-text-primary);
  border-radius: 4px;
  outline: none;
  transition: border-color 0.15s;
}

.filter-search:focus {
  border-color: rgba(106, 13, 173, 0.5);
}

.filter-search::placeholder {
  color: var(--color-text-tertiary);
}

.filter-check {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 11px;
}

.filter-check input[type="checkbox"] {
  accent-color: rgba(106, 13, 173, 0.8);
  width: 14px;
  height: 14px;
  cursor: pointer;
}

.filter-check-label {
  color: var(--color-text-secondary);
  user-select: none;
}

.filter-info {
  font-size: 10px;
  color: var(--color-text-tertiary);
  text-align: right;
}

/* === Concept list === */
.list-container {
  height: calc(100vh - 120px);
  overflow-y: auto;
  padding: 0;
}

.list-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  position: sticky;
  top: 0;
  background: var(--color-bg-primary);
  z-index: 2;
}

.list-search {
  flex: 1;
  padding: 8px 12px;
  font-size: 13px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.05);
  color: var(--color-text-primary);
  border-radius: 8px;
  outline: none;
  transition: border-color 0.15s;
}

.list-search:focus {
  border-color: rgba(106, 13, 173, 0.5);
}

.list-search::placeholder {
  color: var(--color-text-tertiary);
}

.list-count {
  font-size: 12px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
}

.concept-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.concept-card {
  padding: 16px;
  background: var(--glass-bg);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.concept-card:hover {
  border-color: rgba(106, 13, 173, 0.3);
  background: rgba(106, 13, 173, 0.05);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
}

.card-title {
  font-family: var(--font-heading);
  font-size: 16px;
  color: var(--color-text-primary);
  margin: 0;
}

.card-definition {
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.5;
  margin: 0 0 8px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-sources {
  font-size: 11px;
  color: var(--color-text-tertiary);
}

.card-date {
  font-size: 11px;
  color: var(--color-text-tertiary);
}

/* === Detail panel === */
.concept-detail-panel {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 480px;
  background: var(--color-bg-primary);
  border-left: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  z-index: 10;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.3);
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 24px;
  border-bottom: 1px solid var(--color-border);
}

.detail-title {
  font-family: var(--font-heading);
  font-size: 22px;
  color: var(--color-text-primary);
  margin: 0 0 8px;
}

.detail-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-sources {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.detail-body {
  flex: 1;
  overflow: auto;
  padding: 24px;
}

.local-graph-wrap {
  margin-bottom: 20px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  overflow: hidden;
  background: #0f0f1a;
}

.local-graph-title {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  border-bottom: 1px solid var(--color-border);
}

.contradiction-banner {
  padding: 12px 16px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  border-radius: 8px;
  color: #f59e0b;
  font-size: 13px;
  margin-bottom: 16px;
  line-height: 1.5;
}

/* === Markdown === */
:deep(.wiki-h1) {
  font-family: var(--font-heading);
  font-size: 24px;
  color: var(--color-text-primary);
  margin: 20px 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--color-border);
}

:deep(.wiki-h2) {
  font-family: var(--font-heading);
  font-size: 20px;
  color: var(--color-text-primary);
  margin: 20px 0 10px;
}

:deep(.wiki-h3) {
  font-size: 16px;
  color: var(--color-text-primary);
  margin: 14px 0 6px;
}

:deep(.wiki-code) {
  background: rgba(106, 13, 173, 0.12);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 13px;
}

:deep(.wiki-pre) {
  background: var(--color-bg-secondary);
  padding: 14px;
  border-radius: 8px;
  overflow-x: auto;
  border: 1px solid var(--color-border);
  margin: 14px 0;
}

:deep(.wiki-pre code) {
  background: transparent;
  padding: 0;
}

:deep(.wiki-ul) {
  margin: 10px 0;
  padding-left: 24px;
}

:deep(.wiki-li) {
  margin: 4px 0;
  color: var(--color-text-secondary);
}

/* === Transitions === */
.slide-enter-active {
  transition: transform 0.3s ease;
}
.slide-leave-active {
  transition: transform 0.2s ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
}

/* === Evolution Log === */
.evolution-section {
  margin-bottom: 16px;
  padding: 12px 16px;
  background: rgba(30, 30, 46, 0.3);
  border-radius: 8px;
  border: 1px solid var(--color-border);
}

.evolution-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px;
}

.evolution-entries {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.evolution-entry {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
}

.evolution-action {
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
}

.evolution-reinforce .evolution-action {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.evolution-revise .evolution-action {
  background: rgba(59, 130, 246, 0.15);
  color: #3b82f6;
}

.evolution-contradict .evolution-action {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.evolution-source_added .evolution-action {
  background: rgba(156, 163, 175, 0.15);
  color: #9ca3af;
}

.evolution-detail {
  color: var(--color-text-secondary);
}
</style>
