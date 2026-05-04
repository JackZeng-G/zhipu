<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  NInput,
  NButton,
  NSpace,
  NPagination,
  NSpin,
  NEmpty,
  NTree,
  NCollapse,
  NCollapseItem,
  NTag,
  useMessage
} from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { useNotesStore, type Notebook } from '@/stores/notes'
import { useKnowledgeStore } from '@/stores/knowledge'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const notesStore = useNotesStore()
const knowledgeStore = useKnowledgeStore()

const searchQuery = ref('')
const summary = ref<string | null>(null)
const editInstruction = ref('')
const editSelectedText = ref('')
const editResult = ref<string | null>(null)
const summarizing = ref(false)
const editing = ref(false)
const detailLoading = ref(false)

const filteredNotes = computed(() => {
  if (!searchQuery.value) return notesStore.notes
  const q = searchQuery.value.toLowerCase()
  return notesStore.notes.filter((n) => n.title.toLowerCase().includes(q))
})

function notebooksToTreeOptions(notebooks: Notebook[]): TreeOption[] {
  return notebooks.map((nb) => ({
    key: nb.id,
    label: nb.title,
    children: nb.children ? notebooksToTreeOptions(nb.children) : undefined
  }))
}

const treeOptions = computed(() => notebooksToTreeOptions(notesStore.notebooks))

function handleTreeSelect(keys: string[]) {
  if (keys.length > 0) {
    notesStore.fetchNotes(keys[0], 1, notesStore.pagination.pageSize)
  }
}

async function handlePageChange(page: number) {
  await notesStore.fetchNotes(notesStore.currentNotebookId || undefined, page, notesStore.pagination.pageSize)
}

async function selectNote(id: string) {
  detailLoading.value = true
  await notesStore.fetchNote(id)
  summary.value = null
  editResult.value = null
  // Fetch knowledge index data
  knowledgeStore.clear()
  knowledgeStore.fetchAll(id)
  detailLoading.value = false
}

async function handleSummarize() {
  if (!notesStore.currentNote) return
  summarizing.value = true
  summary.value = null
  try {
    const result = await notesStore.summarize(notesStore.currentNote.id)
    if (result) {
      summary.value = result
    } else {
      message.error(notesStore.error || '生成摘要失败')
    }
  } finally {
    summarizing.value = false
  }
}

async function handleEdit() {
  if (!notesStore.currentNote || !editInstruction.value) return
  editing.value = true
  editResult.value = null
  try {
    const result = await notesStore.edit(
      notesStore.currentNote.id,
      editInstruction.value,
      editSelectedText.value || undefined
    )
    if (result) {
      editResult.value = result
    } else {
      message.error(notesStore.error || 'AI 编辑失败')
    }
  } finally {
    editing.value = false
  }
}

function startChat() {
  if (!notesStore.currentNote) return
  router.push({ path: '/chat', query: { noteId: notesStore.currentNote.id } })
}

async function handleIndexNote() {
  if (!notesStore.currentNote) return
  try {
    await knowledgeStore.doIndex(notesStore.currentNote.id)
    message.success('索引完成')
    knowledgeStore.fetchAll(notesStore.currentNote.id)
  } catch (e: any) {
    message.error('索引失败: ' + (e.response?.data?.error || e.message))
  }
}

function formatDate(ts: number) {
  return new Date(ts * 1000).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit'
  })
}

onMounted(async () => {
  await Promise.all([
    notesStore.fetchNotebooks(),
    notesStore.fetchNotes(undefined, 1, 20)
  ])

  // Handle search result link: /?note=<id>
  const noteId = route.query.note as string
  if (noteId) {
    // Find the note in the current list or fetch it
    const found = notesStore.notes.find(n => n.id === noteId)
    if (found) {
      await selectNote(noteId)
    } else {
      // Note not in current page, try to fetch directly
      await selectNote(noteId)
    }
    // Clear the query param from URL
    router.replace({ query: {} })
  }
})
</script>

<template>
  <div class="notes-layout">
    <!-- 左栏：笔记本树 -->
    <div class="panel-notebooks">
      <div class="panel-header">
        <h3 class="panel-title">笔记本</h3>
      </div>
      <div class="notebook-tree-wrapper">
        <NTree
          :data="treeOptions"
          block-line
          selectable
          :default-expand-all="true"
          @update:selected-keys="handleTreeSelect"
        />
      </div>
    </div>

    <!-- 中栏：笔记列表 -->
    <div class="panel-list">
      <div class="search-bar">
        <NInput
          v-model:value="searchQuery"
          placeholder="搜索笔记..."
          clearable
          size="medium"
        />
      </div>

      <div class="note-list-content">
        <NSpin :show="notesStore.loading">
          <TransitionGroup name="list" tag="div" class="note-list-inner">
            <div
              v-for="note in filteredNotes"
              :key="note.id"
              class="note-card"
              :class="{ 'note-card-active': notesStore.currentNote?.id === note.id }"
              @click="selectNote(note.id)"
            >
              <div class="note-card-title">{{ note.title }}</div>
              <div class="note-card-meta">{{ formatDate(note.modified_time) }}</div>
            </div>
          </TransitionGroup>
          <NEmpty v-if="filteredNotes.length === 0 && !notesStore.loading" description="暂无笔记" class="empty-state" />
        </NSpin>
      </div>

      <div class="pagination-bar">
        <NPagination
          :page="notesStore.pagination.page"
          :page-count="Math.ceil(notesStore.pagination.total / notesStore.pagination.pageSize)"
          :max-page-count="5"
          @update:page="handlePageChange"
          size="small"
        />
      </div>
    </div>

    <!-- 右栏：笔记详情 -->
    <div class="panel-detail" v-if="notesStore.currentNote">
      <div class="detail-content">
        <NSpin :show="detailLoading">
          <article class="note-article">
            <header class="note-header">
              <h1 class="note-title">{{ notesStore.currentNote.title }}</h1>
              <div class="note-meta">
                <span>创建 {{ formatDate(notesStore.currentNote.created_time) }}</span>
                <span class="meta-sep">·</span>
                <span>修改 {{ formatDate(notesStore.currentNote.modified_time) }}</span>
              </div>
            </header>

            <!-- 知识索引：实体标签 -->
            <div v-if="knowledgeStore.entities.length > 0" class="entity-tags">
              <NTag
                v-for="entity in knowledgeStore.entities"
                :key="entity.id"
                size="small"
                :type="entity.entity_type === 'technology' ? 'info' : entity.entity_type === 'tool' ? 'warning' : entity.entity_type === 'concept' ? 'success' : 'default'"
                class="entity-tag"
                @click="$router.push({ path: '/wiki', query: { entity: entity.entity_name } })"
              >
                {{ entity.entity_name }}
              </NTag>
            </div>

            <!-- 知识索引：持久化摘要 -->
            <div v-if="knowledgeStore.summary" class="indexed-summary">
              <div class="summary-label">AI 摘要</div>
              <div class="summary-text">{{ knowledgeStore.summary.summary }}</div>
            </div>

            <div class="note-body" v-html="notesStore.currentNote.content_html || notesStore.currentNote.content_text || ''" />
          </article>
        </NSpin>
      </div>

      <!-- 关联笔记面板 -->
      <div v-if="knowledgeStore.relatedNotes.length > 0" class="related-panel">
        <div class="related-title">关联笔记</div>
        <div class="related-list">
          <div
            v-for="rel in knowledgeStore.relatedNotes"
            :key="rel.note_id"
            class="related-item"
            @click="selectNote(rel.note_id)"
          >
            <div class="related-item-type">{{ rel.relation_type }}</div>
            <div class="related-item-reason">{{ rel.reason }}</div>
          </div>
        </div>
      </div>

      <!-- AI 工具面板 -->
      <div class="ai-panel">
        <NCollapse>
          <NCollapse-item title="AI 工具" name="ai">
            <NSpace vertical :size="12">
              <NButton size="small" :loading="summarizing" @click="handleSummarize" class="btn-glow">
                生成摘要
              </NButton>
              <NButton size="small" @click="handleIndexNote" class="btn-glow">
                索引此笔记
              </NButton>
              <div v-if="summary" class="ai-result-card">
                {{ summary }}
              </div>
              <NInput
                v-model:value="editSelectedText"
                type="textarea"
                placeholder="选中的文本（可选）"
                :rows="2"
              />
              <NInput
                v-model:value="editInstruction"
                placeholder="输入编辑指令"
                :rows="2"
                @keydown.enter.ctrl="handleEdit"
              />
              <NSpace>
                <NButton size="small" :loading="editing" @click="handleEdit" :disabled="!editInstruction" class="btn-glow">
                  提交编辑
                </NButton>
                <NButton size="small" @click="startChat">
                  开始对话
                </NButton>
              </NSpace>
              <div v-if="editResult" class="ai-result-card">
                {{ editResult }}
              </div>
            </NSpace>
          </NCollapse-item>
        </NCollapse>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="panel-detail panel-empty">
      <div class="empty-illustration">
        <svg width="64" height="64" viewBox="0 0 24 24" fill="none" opacity="0.2">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" stroke="currentColor" stroke-width="1.5"/>
          <polyline points="14 2 14 8 20 8" stroke="currentColor" stroke-width="1.5"/>
          <line x1="16" y1="13" x2="8" y2="13" stroke="currentColor" stroke-width="1.5"/>
          <line x1="16" y1="17" x2="8" y2="17" stroke="currentColor" stroke-width="1.5"/>
        </svg>
        <p>选择一篇笔记查看详情</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.notes-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--color-bg-primary);
}

/* === 左栏：笔记本 === */
.panel-notebooks {
  width: 220px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--color-border);
  flex-shrink: 0;
  background: rgba(15, 15, 26, 0.5);
}

.panel-header {
  padding: 20px 20px 12px;
}

.panel-title {
  font-family: var(--font-heading);
  font-size: 16px;
  font-weight: 500;
  color: var(--color-text-primary);
  letter-spacing: 0.02em;
}

.notebook-tree-wrapper {
  overflow: auto;
  padding: 0 12px 8px;
}

.category-section {
  border-top: 1px solid var(--color-border);
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.category-section .panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px 8px;
}

.category-tree-wrapper {
  flex: 1;
  overflow: auto;
  padding: 0 12px 12px;
}

.category-empty {
  padding: 8px 16px 16px;
}

.category-hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

/* === 中栏：笔记列表 === */
.panel-list {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--color-border);
  background: var(--color-bg-primary);
  overflow: hidden;
}

.search-bar {
  padding: 16px;
  border-bottom: 1px solid var(--color-border);
}

.note-list-content {
  flex: 1;
  overflow: auto;
}

.note-list-inner {
  padding: 8px;
}

.note-card {
  padding: 14px 16px;
  border-radius: 8px;
  cursor: pointer;
  transition: all var(--transition-fast);
  margin-bottom: 2px;
  border: 1px solid transparent;
}

.note-card:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: var(--color-border);
}

.note-card-active {
  background: rgba(106, 13, 173, 0.1);
  border-color: rgba(106, 13, 173, 0.25);
}

.note-card-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
  margin-bottom: 4px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.note-card-meta {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.pagination-bar {
  padding: 12px 8px;
  display: flex;
  justify-content: center;
  border-top: 1px solid var(--color-border);
  flex-shrink: 0;
  overflow: hidden;
}

.empty-state {
  padding: 64px 24px;
}

/* === 右栏：详情 === */
.panel-detail {
  flex: 1;
  min-width: 400px;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-primary);
}

.panel-empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-illustration {
  text-align: center;
  color: var(--color-text-tertiary);
  font-size: 14px;
}

.empty-illustration svg {
  margin-bottom: 12px;
  color: var(--color-text-tertiary);
}

.detail-content {
  flex: 1;
  overflow: auto;
  min-height: 0;
}

.note-article {
  padding: 24px;
}

.note-header {
  margin-bottom: 24px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--color-border);
}

.note-title {
  font-family: var(--font-heading);
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.3;
  margin-bottom: 12px;
}

.note-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.meta-sep {
  opacity: 0.4;
}

.note-body {
  line-height: 1.8;
  font-size: 15px;
  color: var(--color-text-secondary);
}

/* 强制覆盖 NAS HTML 内容中的浅色样式 */
.note-body :deep(*) {
  color: var(--color-text-secondary) !important;
  background-color: transparent !important;
}

.note-body :deep(img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 12px 0;
}

.note-body :deep(pre) {
  background: var(--color-bg-secondary) !important;
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  border: 1px solid var(--color-border);
  font-family: var(--font-mono);
}

.note-body :deep(code) {
  background: rgba(106, 13, 173, 0.12) !important;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 13px;
}

.note-body :deep(a) {
  color: #8b5cf6 !important;
  text-decoration: none;
}

.note-body :deep(a:hover) {
  text-decoration: underline;
}

.note-body :deep(blockquote) {
  border-left: 3px solid rgba(106, 13, 173, 0.4);
  padding-left: 16px;
  margin: 12px 0;
  color: var(--color-text-tertiary) !important;
}

.note-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0;
}

.note-body :deep(th),
.note-body :deep(td) {
  padding: 8px 12px;
  border: 1px solid var(--color-border);
  text-align: left;
}

.note-body :deep(th) {
  background: var(--color-bg-secondary) !important;
  font-weight: 500;
}

.note-body :deep(font) {
  color: inherit !important;
}

.note-body :deep(span[style*="color"]),
.note-body :deep(div[style*="color"]) {
  color: var(--color-text-secondary) !important;
}

.note-body :deep(h1),
.note-body :deep(h2),
.note-body :deep(h3),
.note-body :deep(h4),
.note-body :deep(h5),
.note-body :deep(h6) {
  color: var(--color-text-primary) !important;
  margin: 16px 0 8px;
}

.note-body :deep(li),
.note-body :deep(p),
.note-body :deep(ul),
.note-body :deep(ol) {
  color: var(--color-text-secondary) !important;
}

.note-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--color-border);
  margin: 16px 0;
}

/* === AI 面板 === */
.ai-panel {
  border-top: 1px solid var(--color-border);
  padding: 12px 16px;
  max-height: 300px;
  overflow: auto;
  flex-shrink: 0;
  background: rgba(15, 15, 26, 0.5);
}

.ai-result-card {
  background: rgba(106, 13, 173, 0.08);
  border: 1px solid rgba(106, 13, 173, 0.2);
  border-radius: 8px;
  padding: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
}

.btn-glow {
  position: relative;
}

.btn-glow:hover {
  box-shadow: 0 0 20px rgba(106, 13, 173, 0.3);
}

/* === 知识索引：实体标签 === */
.entity-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0 24px 16px;
  padding: 12px 16px;
  background: rgba(106, 13, 173, 0.06);
  border: 1px solid rgba(106, 13, 173, 0.15);
  border-radius: 8px;
}

.entity-tag {
  cursor: pointer;
  transition: all var(--transition-fast);
}

.entity-tag:hover {
  box-shadow: 0 0 12px rgba(106, 13, 173, 0.3);
}

/* === 知识索引：摘要 === */
.indexed-summary {
  margin: 0 24px 16px;
  padding: 16px;
  background: rgba(67, 97, 238, 0.06);
  border: 1px solid rgba(67, 97, 238, 0.15);
  border-radius: 8px;
}

.summary-label {
  font-size: 11px;
  font-weight: 500;
  color: #8b5cf6;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.summary-text {
  font-size: 14px;
  line-height: 1.6;
  color: var(--color-text-secondary);
}

/* === 知识索引：关联笔记 === */
.related-panel {
  border-top: 1px solid var(--color-border);
  padding: 16px;
  max-height: 200px;
  overflow: auto;
  flex-shrink: 0;
  background: rgba(15, 15, 26, 0.5);
}

.related-title {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-tertiary);
  margin-bottom: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.related-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.related-item {
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.related-item:hover {
  border-color: rgba(106, 13, 173, 0.4);
  background: rgba(106, 13, 173, 0.08);
}

.related-item-type {
  font-size: 11px;
  font-weight: 500;
  color: #8b5cf6;
  text-transform: uppercase;
  margin-bottom: 2px;
}

.related-item-reason {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* === 列表动画 === */
.list-enter-active {
  transition: all 0.3s ease;
}
.list-leave-active {
  transition: all 0.2s ease;
}
.list-enter-from {
  opacity: 0;
  transform: translateX(-8px);
}
.list-leave-to {
  opacity: 0;
}
</style>
