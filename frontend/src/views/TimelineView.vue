<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NTabs, NTabPane, NDataTable, NEmpty, NSpin, NPageHeader, NSpace,
  NButton, NInput, NCard, NTag, NList, NListItem, NThing, NModal,
  useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshOutline, PulseOutline, PlayOutline, AddOutline, TrashOutline } from '@vicons/ionicons5'
import DOMPurify from 'dompurify'
import {
  getActivities, runLint, getIndexStatus, resetIndexes, triggerIndex,
  listOutputs, getOutput, deleteOutput,
  listQuestions, createQuestion, resolveQuestion,
  runReflect, getReflectStatus,
  getMergeCandidates
} from '@/api'

interface Activity {
  id: number
  activity_type: string
  target_type: string
  target_id: string
  description: string
  metadata: string
  created_at: number
}

interface RowData {
  id: number
  noteId: string
  noteName: string
  conceptCount: number
  activityType: string
  time: string
  timestamp: number
}

const router = useRouter()
const message = useMessage()

// --- Shared state ---
const activeTab = ref('overview')
const loading = ref(false)

// --- Index status ---
const indexStatus = ref<any>({})
const resetting = ref(false)
const indexing = ref(false)

// --- Lint ---
const lintResult = ref<any>(null)
const linting = ref(false)

// --- Activities ---
const activities = ref<Activity[]>([])

// --- Outputs ---
const outputs = ref<any[]>([])
const outputsLoading = ref(false)
const outputDetail = ref<any>(null)
const showOutputModal = ref(false)

// --- Questions ---
const questions = ref<any[]>([])
const questionsLoading = ref(false)
const newQuestion = ref('')
const addingQuestion = ref(false)

// --- Reflect ---
const reflectRunning = ref(false)
const reflectPolling = ref(false)

// --- Merge ---
const mergeCandidates = ref<any[]>([])

// ==================== Computed ====================

// Filter reflect-related activities for REFLECT tab
const reflectActivities = computed(() => {
  return activities.value
    .filter((a: any) => ['reflect', 'query_persist', 'merge', 'confirm_confidence'].includes(a.activity_type))
    .sort((a: any, b: any) => b.created_at - a.created_at)
})

// Group lint issues by type for better display
const lintIssueGroups = computed(() => {
  if (!lintResult.value?.issues) return null
  const groups: Record<string, any[]> = {}
  for (const issue of lintResult.value.issues) {
    const type = issue.type || 'unknown'
    if (!groups[type]) groups[type] = []
    groups[type].push(issue)
  }
  return groups
})

// Count lint issues by type
function lintIssueCount(type: string): number {
  if (!lintResult.value?.issues) return 0
  return lintResult.value.issues.filter((i: any) => i.type === type).length
}

function lintTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    no_entities: '无实体',
    no_relations: '无关联',
    stale_summary: '过期摘要',
    popular_entity: '热门实体',
    stub_concept: '空概念',
    stale_concept: '过期概念',
    orphan_concept: '孤立概念',
    content_hash_mismatch: '内容变更'
  }
  return labels[type] || type
}

function outputTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    query: '查询',
    gap: '差距分析',
    synthesis: '综合分析',
    reflect: '反思'
  }
  return labels[type] || type
}

// ==================== Existing functions ====================

async function handleFullLint() {
  linting.value = true
  try {
    const res = await runLint()
    lintResult.value = res.data
  } catch (e: any) {
    message.error('检查失败')
  } finally {
    linting.value = false
  }
}

async function loadIndexStatus() {
  try {
    const res = await getIndexStatus()
    indexStatus.value = res.data
  } catch (e) {
    // ignore
  }
}

async function handleResetIndexes() {
  if (!confirm('确定要重置所有索引数据吗？这将清除所有实体、摘要、关联、概念图谱数据。笔记本身不受影响。')) return
  resetting.value = true
  try {
    await resetIndexes()
    message.success('索引已重置')
    indexStatus.value = {}
    await loadIndexStatus()
  } catch (e: any) {
    message.error('重置失败: ' + (e.response?.data?.error || e.message))
  } finally {
    resetting.value = false
  }
}

async function handleIndexAll() {
  indexing.value = true
  try {
    await triggerIndex()
    message.success('全量索引已启动')
    await loadIndexStatus()
  } catch (e: any) {
    message.error('索引失败: ' + (e.response?.data?.error || e.message))
  } finally {
    indexing.value = false
  }
}

const activityLabels: Record<string, string> = {
  sync: '同步',
  index: '索引',
  auto_index: '自动索引',
  categorize: '分类',
  search: '搜索',
  generate_wiki: '生成 Wiki',
  auto_wiki: '自动 Wiki',
  wiki_update: 'Wiki 更新',
  build_relations: '关联分析',
  chat_insight: '洞察',
  ingest: '知识吸收',
  lint_fix: '修复',
  reflect: '反思',
  query_persist: '查询保存',
  merge: '概念合并',
  confirm_confidence: '置信度确认',
  question_match: '问题匹配',
  add_question: '添加问题',
  resolve_question: '问题解决',
  gap_analysis: '差距分析'
}

// Build all activities into rows (index+ingest merged, others shown directly)
const tableData = computed<RowData[]>(() => {
  const merged = new Map<string, RowData>()
  const otherRows: RowData[] = []

  for (const act of activities.value) {
    const label = activityLabels[act.activity_type] || act.activity_type

    // Merge index + ingest activities by target_id
    if (act.activity_type === 'index' || act.activity_type === 'ingest' || act.activity_type === 'auto_index') {
      const key = act.target_id || act.description
      const existing = merged.get(key)

      if (act.activity_type === 'index' || act.activity_type === 'auto_index') {
        const parts = act.description.split(': ')
        const noteName = parts.length > 1 ? (parts[1] || act.description) : act.description

        if (existing) {
          existing.noteName = noteName
          existing.time = formatTime(act.created_at)
          existing.timestamp = act.created_at
        } else {
          merged.set(key, {
            id: act.id,
            noteId: act.target_id || '',
            noteName: noteName,
            conceptCount: 0,
            activityType: label,
            time: formatTime(act.created_at),
            timestamp: act.created_at
          })
        }
      } else if (act.activity_type === 'ingest') {
        const match = act.description.match(/updated (\d+) concepts/)
        const count = match && match[1] ? parseInt(match[1]) : 0

        if (existing) {
          existing.conceptCount = count
        } else {
          merged.set(key, {
            id: act.id,
            noteId: act.target_id || '',
            noteName: act.target_id || act.description,
            conceptCount: count,
            activityType: '索引',
            time: formatTime(act.created_at),
            timestamp: act.created_at
          })
        }
      }
    } else {
      // Show all other activities directly (reflect, merge, query, etc.)
      otherRows.push({
        id: act.id,
        noteId: act.target_id || '',
        noteName: act.description,
        conceptCount: 0,
        activityType: label,
        time: formatTime(act.created_at),
        timestamp: act.created_at
      })
    }
  }

  return [...otherRows, ...Array.from(merged.values())].sort((a, b) => b.timestamp - a.timestamp)
})

const columns: DataTableColumns<RowData> = [
  {
    title: '活动',
    key: 'noteName',
    ellipsis: { tooltip: true },
    render(row) {
      return row.noteName
    }
  },
  {
    title: '类型',
    key: 'activityType',
    width: 120,
    render(row) {
      return row.activityType
    }
  },
  {
    title: '概念数',
    key: 'conceptCount',
    width: 80,
    render(row) {
      if (row.conceptCount > 0) {
        return row.conceptCount
      }
      return '-'
    }
  },
  {
    title: '时间',
    key: 'time',
    width: 140,
    render(row) {
      return row.time
    }
  }
]

async function loadActivities() {
  loading.value = true
  try {
    const res = await getActivities()
    activities.value = res.data.activities || []
  } catch (e) {
    message.error('加载活动日志失败')
  } finally {
    loading.value = false
  }
}

function formatTime(ts: number) {
  return new Date(ts * 1000).toLocaleString('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function handleRowClick(row: RowData) {
  // Only navigate to note for index-related activities
  if (row.noteId && ['索引', '自动索引', '知识吸收'].includes(row.activityType)) {
    router.push({ path: '/', query: { note: row.noteId } })
  }
}

// ==================== Outputs functions ====================

async function loadOutputs() {
  outputsLoading.value = true
  try {
    const res = await listOutputs()
    outputs.value = res.data.outputs || []
  } catch (e) {
    message.error('加载输出失败')
  } finally {
    outputsLoading.value = false
  }
}

async function viewOutput(slug: string) {
  try {
    const res = await getOutput(slug)
    outputDetail.value = res.data
    showOutputModal.value = true
  } catch (e) {
    message.error('加载失败')
  }
}

async function handleDeleteOutput(slug: string) {
  try {
    await deleteOutput(slug)
    message.success('已删除')
    loadOutputs()
  } catch (e) {
    message.error('删除失败')
  }
}

// ==================== Questions functions ====================

async function loadQuestions() {
  questionsLoading.value = true
  try {
    const res = await listQuestions()
    questions.value = res.data.questions || []
  } catch (e) {
    message.error('加载问题失败')
  } finally {
    questionsLoading.value = false
  }
}

async function handleAddQuestion() {
  if (!newQuestion.value.trim()) return
  addingQuestion.value = true
  try {
    await createQuestion(newQuestion.value.trim())
    newQuestion.value = ''
    message.success('问题已添加')
    loadQuestions()
  } catch (e) {
    message.error('添加失败')
  } finally {
    addingQuestion.value = false
  }
}

async function handleResolveQuestion(id: number) {
  try {
    await resolveQuestion(id)
    message.success('问题已标记为已解决')
    loadQuestions()
  } catch (e) {
    message.error('操作失败')
  }
}

// ==================== Reflect functions ====================

async function handleReflect() {
  try {
    await runReflect()
    reflectRunning.value = true
    message.success('反思已启动')
    pollReflectStatus()
  } catch (e: any) {
    if (e.response?.status === 409) {
      message.info('反思正在运行中')
    } else {
      message.error('启动失败')
    }
  }
}

async function pollReflectStatus() {
  if (reflectPolling.value) return
  reflectPolling.value = true
  const poll = async () => {
    try {
      const res = await getReflectStatus()
      reflectRunning.value = res.data.running
      if (reflectRunning.value) {
        setTimeout(poll, 3000)
      } else {
        reflectPolling.value = false
        message.success('反思完成')
        loadOutputs()
      }
    } catch {
      reflectPolling.value = false
    }
  }
  poll()
}

// ==================== Merge functions ====================

async function loadMergeCandidates() {
  try {
    const res = await getMergeCandidates()
    mergeCandidates.value = res.data.candidates || []
  } catch (e) {
    // silently ignore - endpoint may not be ready
  }
}

// ==================== Tab data loading ====================

const loadedTabs = ref(new Set<string>())

function loadTabData(tab: string) {
  if (loadedTabs.value.has(tab)) return
  loadedTabs.value.add(tab)

  switch (tab) {
    case 'overview':
      loadIndexStatus()
      handleFullLint()
      break
    case 'activities':
      loadActivities()
      break
    case 'outputs':
      loadOutputs()
      break
    case 'questions':
      loadQuestions()
      break
    case 'reflect':
      // Check reflect status on first visit
      getReflectStatus().then(res => {
        reflectRunning.value = res.data.running
        if (reflectRunning.value) pollReflectStatus()
      }).catch(() => {})
      break
    case 'health':
      if (!lintResult.value) handleFullLint()
      break
  }
}

watch(activeTab, (tab) => {
  loadTabData(tab)
})

// ==================== Refresh all ====================

async function refreshAll() {
  loading.value = true
  loadedTabs.value.clear()
  try {
    await Promise.all([
      loadActivities(),
      handleFullLint(),
      loadIndexStatus(),
      loadOutputs(),
      loadQuestions(),
      loadMergeCandidates()
    ])
    loadedTabs.value.add('overview')
    loadedTabs.value.add('activities')
    loadedTabs.value.add('outputs')
    loadedTabs.value.add('questions')
    loadedTabs.value.add('health')
  } finally {
    loading.value = false
  }
}

// ==================== Markdown rendering ====================

function renderMarkdown(content: string): string {
  if (!content) return ''
  // Basic markdown: escape HTML, then apply simple transformations
  let html = content
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  // Italic
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')
  // Code blocks
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  // Headers
  html = html.replace(/^### (.+)$/gm, '<h4>$1</h4>')
  html = html.replace(/^## (.+)$/gm, '<h3>$1</h3>')
  html = html.replace(/^# (.+)$/gm, '<h2>$1</h2>')
  // Line breaks
  html = html.replace(/\n/g, '<br>')
  return DOMPurify.sanitize(html)
}

// ==================== Lifecycle ====================

onMounted(() => {
  loadTabData('overview')
})
</script>

<template>
  <div class="timeline-page">
    <NPageHeader title="知识中心" subtitle="知识库管理与监控">
      <template #extra>
        <NButton size="small" @click="refreshAll" :loading="loading">
          <template #icon><RefreshOutline /></template>
          刷新
        </NButton>
      </template>
    </NPageHeader>

    <NTabs v-model:value="activeTab" type="line" animated style="margin-top: 16px">
      <!-- Tab 1: Overview -->
      <NTabPane name="overview" tab="概览">
        <NSpace vertical :size="16">
          <!-- Index status cards -->
          <div class="status-cards">
            <div class="status-card" v-if="indexStatus.total_notes > 0">
              <div class="status-item">
                <span class="status-value">{{ indexStatus.indexed_notes || 0 }}</span>
                <span class="status-label">已索引 / {{ indexStatus.total_notes }}</span>
              </div>
              <div class="status-item">
                <span class="status-value">{{ indexStatus.total_entities || 0 }}</span>
                <span class="status-label">实体</span>
              </div>
              <div class="status-item">
                <span class="status-value">{{ indexStatus.total_relations || 0 }}</span>
                <span class="status-label">关联</span>
              </div>
              <div class="status-item" v-if="lintResult">
                <span class="status-value" :class="{ 'text-warning': lintIssueCount('no_relations') > 0 }">{{ lintIssueCount('no_relations') }}</span>
                <span class="status-label">无关联</span>
              </div>
              <div class="status-item" v-if="lintResult">
                <span class="status-value" :class="{ 'text-warning': lintIssueCount('stub_concept') > 0 }">{{ lintIssueCount('stub_concept') }}</span>
                <span class="status-label">空概念</span>
              </div>
              <div class="status-item" v-if="indexStatus.active_provider">
                <span class="status-value">{{ indexStatus.active_provider }}</span>
                <span class="status-label">{{ indexStatus.active_model }}</span>
              </div>
            </div>
            <div class="status-card" v-else>
              <div class="status-loading">正在加载索引状态...</div>
            </div>
          </div>

          <!-- Quick actions -->
          <div class="quick-actions">
            <NButton type="primary" @click="handleIndexAll" :loading="indexing">
              <template #icon><PlayOutline /></template>
              全量索引
            </NButton>
            <NButton @click="handleReflect" :loading="reflectRunning" :disabled="reflectRunning">
              <template #icon><PlayOutline /></template>
              REFLECT
            </NButton>
            <NButton type="error" ghost @click="handleResetIndexes" :loading="resetting">
              <template #icon><TrashOutline /></template>
              重置索引
            </NButton>
          </div>

          <!-- Quick lint summary on overview -->
          <div class="status-card" v-if="lintResult">
            <div class="lint-row">
              <div class="status-item">
                <span class="status-value">{{ lintResult.total_notes }}</span>
                <span class="status-label">笔记总数</span>
              </div>
              <div class="status-item">
                <span class="status-value">{{ lintResult.indexed_notes }}</span>
                <span class="status-label">已索引</span>
              </div>
              <div class="status-item">
                <span class="status-value" :class="{ 'text-warning': lintResult.orphaned_notes > 0 }">{{ lintResult.orphaned_notes }}</span>
                <span class="status-label">孤儿笔记</span>
              </div>
              <div class="status-item">
                <span class="status-value" :class="{ 'text-warning': lintResult.stale_summaries > 0 }">{{ lintResult.stale_summaries }}</span>
                <span class="status-label">过期摘要</span>
              </div>
            </div>
          </div>
        </NSpace>
      </NTabPane>

      <!-- Tab 2: Activity Log -->
      <NTabPane name="activities" tab="活动日志">
        <div class="table-container">
          <NSpin :show="loading">
            <NEmpty v-if="tableData.length === 0" description="暂无索引记录" class="empty-state" />
            <NDataTable
              v-else
              :columns="columns"
              :data="tableData"
              :bordered="false"
              :single-line="false"
              :row-props="(row: RowData) => ({ style: 'cursor: pointer;', onClick: () => handleRowClick(row) })"
              max-height="calc(100vh - 280px)"
              virtual-scroll
            />
          </NSpin>
        </div>
      </NTabPane>

      <!-- Tab 3: Outputs -->
      <NTabPane name="outputs" tab="输出">
        <NSpin :show="outputsLoading">
          <NList v-if="outputs.length > 0" bordered>
            <NListItem v-for="o in outputs" :key="o.slug">
              <NThing :title="o.title" :description="outputTypeLabel(o.output_type) + ' · ' + formatTime(o.created_at)">
                <template #action>
                  <NSpace>
                    <NButton size="tiny" @click="viewOutput(o.slug)">查看</NButton>
                    <NButton size="tiny" type="error" ghost @click="handleDeleteOutput(o.slug)">删除</NButton>
                  </NSpace>
                </template>
              </NThing>
            </NListItem>
          </NList>
          <NEmpty v-else description="暂无持久化输出" />
        </NSpin>
      </NTabPane>

      <!-- Tab 4: Questions -->
      <NTabPane name="questions" tab="问题">
        <NSpace vertical :size="16">
          <NSpace align="center">
            <NInput
              v-model:value="newQuestion"
              placeholder="输入开放问题..."
              style="width: 400px"
              @keydown.enter="handleAddQuestion"
            />
            <NButton @click="handleAddQuestion" :loading="addingQuestion" type="primary">
              <template #icon><AddOutline /></template>
              添加
            </NButton>
          </NSpace>
          <NSpin :show="questionsLoading">
            <NList v-if="questions.length > 0" bordered>
              <NListItem v-for="q in questions" :key="q.id">
                <NThing :title="q.content">
                  <template #header-extra>
                    <NTag :type="q.status === 'open' ? 'warning' : 'success'" size="small">
                      {{ q.status === 'open' ? '待解决' : '已解决' }}
                    </NTag>
                  </template>
                  <template #action v-if="q.status === 'open'">
                    <NButton size="tiny" @click="handleResolveQuestion(q.id)">标记已解决</NButton>
                  </template>
                </NThing>
              </NListItem>
            </NList>
            <NEmpty v-else description="暂无开放问题" />
          </NSpin>
        </NSpace>
      </NTabPane>

      <!-- Tab 5: REFLECT -->
      <NTabPane name="reflect" tab="反思">
        <NSpace vertical :size="16">
          <NButton
            type="primary"
            @click="handleReflect"
            :loading="reflectRunning"
            :disabled="reflectRunning"
            size="large"
          >
            <template #icon><PlayOutline /></template>
            {{ reflectRunning ? '正在运行...' : '启动反思' }}
          </NButton>
          <div v-if="reflectRunning" class="reflect-status">
            <NSpin size="small" /> 反思四阶段流水线正在运行，请等待...
          </div>

          <!-- REFLECT history from activity log -->
          <div v-if="reflectActivities.length > 0" class="reflect-history">
            <h4 style="margin: 0 0 8px; color: var(--color-text-primary);">反思历史</h4>
            <div v-for="act in reflectActivities" :key="act.id" class="reflect-entry">
              <span class="reflect-time">{{ formatTime(act.created_at) }}</span>
              <span class="reflect-desc">{{ act.description }}</span>
            </div>
          </div>

          <NEmpty v-if="!reflectRunning && reflectActivities.length === 0" description="点击上方按钮启动反思分析" />
        </NSpace>
      </NTabPane>

      <!-- Tab 6: Health -->
      <NTabPane name="health" tab="健康检查">
        <NSpace vertical :size="16">
          <NButton @click="handleFullLint" :loading="linting">
            <template #icon><PulseOutline /></template>
            运行检查
          </NButton>

          <div class="status-cards" v-if="lintResult">
            <div class="status-card">
              <div class="lint-row">
                <div class="status-item">
                  <span class="status-value">{{ lintResult.total_notes }}</span>
                  <span class="status-label">笔记总数</span>
                </div>
                <div class="status-item">
                  <span class="status-value">{{ lintResult.indexed_notes }}</span>
                  <span class="status-label">已索引</span>
                </div>
                <div class="status-item">
                  <span class="status-value" :class="{ 'text-warning': lintResult.orphaned_notes > 0 }">{{ lintResult.orphaned_notes }}</span>
                  <span class="status-label">孤儿笔记</span>
                </div>
                <div class="status-item">
                  <span class="status-value" :class="{ 'text-warning': lintResult.stale_summaries > 0 }">{{ lintResult.stale_summaries }}</span>
                  <span class="status-label">过期摘要</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Issues detail list -->
          <div v-if="lintResult && lintResult.issues && lintResult.issues.length > 0" class="issues-section">
            <h4 style="margin: 0 0 8px; color: var(--color-text-primary);">
              问题列表 ({{ lintResult.issues.length }})
            </h4>
            <div class="issue-groups" v-if="lintIssueGroups">
              <div v-for="(issues, type) in lintIssueGroups" :key="type" class="issue-group">
                <div class="issue-group-header">
                  <NTag :type="issues[0]?.severity === 'error' ? 'error' : issues[0]?.severity === 'warning' ? 'warning' : 'info'" size="small">
                    {{ lintTypeLabel(type) }}
                  </NTag>
                  <span class="issue-group-count">{{ issues.length }} 项</span>
                </div>
                <div v-for="(issue, idx) in issues.slice(0, 5)" :key="idx" class="issue-item">
                  <span class="issue-title">{{ issue.title }}</span>
                  <span class="issue-desc">{{ issue.description }}</span>
                  <span class="issue-suggestion" v-if="issue.suggestion">→ {{ issue.suggestion }}</span>
                </div>
                <div v-if="issues.length > 5" class="issue-more">
                  ...还有 {{ issues.length - 5 }} 项
                </div>
              </div>
            </div>
          </div>

          <NEmpty v-if="lintResult && (!lintResult.issues || lintResult.issues.length === 0) && !linting" description="所有检查通过，知识库状态良好" />
        </NSpace>
      </NTabPane>
    </NTabs>

    <!-- Output detail modal -->
    <NModal v-model:show="showOutputModal" preset="card" style="max-width: 800px" :title="outputDetail?.title || '输出详情'">
      <div v-if="outputDetail" class="output-content" v-html="renderMarkdown(outputDetail.content || '')" />
    </NModal>
  </div>
</template>

<style scoped>
.timeline-page {
  padding: 24px 32px;
  max-width: 900px;
  margin: 0 auto;
}

.status-cards {
  display: flex;
  gap: 16px;
}

.status-card {
  flex: 1;
  background: rgba(30, 30, 46, 0.3);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid var(--color-border);
  display: flex;
  gap: 24px;
  align-items: center;
  flex-wrap: wrap;
}

.status-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.status-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.status-label {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.text-warning {
  color: #f59e0b;
}

.status-loading {
  text-align: center;
  padding: 12px;
  color: var(--color-text-secondary);
  font-size: 14px;
  width: 100%;
}

.lint-row {
  display: flex;
  gap: 24px;
}

.table-container {
  background: rgba(30, 30, 46, 0.3);
  border-radius: 12px;
  padding: 16px;
}

.empty-state {
  padding: 64px 0;
}

.quick-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.reflect-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px;
  background: rgba(30, 30, 46, 0.3);
  border-radius: 8px;
  color: var(--color-text-secondary);
  font-size: 14px;
}

.reflect-history {
  margin-top: 8px;
}

.reflect-entry {
  display: flex;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.reflect-time {
  font-size: 12px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
  min-width: 100px;
}

.reflect-desc {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.issues-section {
  margin-top: 8px;
}

.issue-groups {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.issue-group {
  background: rgba(30, 30, 46, 0.3);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 12px;
}

.issue-group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.issue-group-count {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.issue-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 0;
  border-top: 1px dashed rgba(255, 255, 255, 0.05);
}

.issue-item:first-of-type {
  border-top: none;
}

.issue-title {
  font-size: 13px;
  color: var(--color-text-primary);
  font-weight: 500;
}

.issue-desc {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.issue-suggestion {
  font-size: 12px;
  color: var(--color-text-accent);
}

.issue-more {
  font-size: 11px;
  color: var(--color-text-tertiary);
  padding-top: 4px;
}

.output-content {
  max-height: 60vh;
  overflow-y: auto;
  line-height: 1.7;
  color: var(--color-text-primary);
}

.output-content :deep(pre) {
  background: rgba(30, 30, 46, 0.5);
  padding: 12px;
  border-radius: 6px;
  overflow-x: auto;
}

.output-content :deep(code) {
  background: rgba(30, 30, 46, 0.5);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

.output-content :deep(h2),
.output-content :deep(h3),
.output-content :deep(h4) {
  margin: 12px 0 6px;
  color: var(--color-text-primary);
}

:deep(.n-data-table) {
  --n-th-color: transparent;
  --n-td-color: transparent;
}

:deep(.n-data-table-th) {
  font-weight: 600;
  color: var(--color-text-primary);
}

:deep(.n-data-table-td) {
  color: var(--color-text-secondary);
}

:deep(.n-data-table-tr:hover .n-data-table-td) {
  background: rgba(255, 255, 255, 0.04) !important;
}
</style>
