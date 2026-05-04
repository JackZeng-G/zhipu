<script setup lang="ts">
import { ref, nextTick, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  NButton,
  NInput,
  NSpin,
  NEmpty,
  NCard,
  NText,
  useMessage
} from 'naive-ui'
import { useChatStore, type Conversation } from '@/stores/chat'
import { saveInsight, aiSearch, runQuery } from '@/api'

interface SearchResult {
  note_id: string
  title: string
  reason: string
}

const route = useRoute()
const message = useMessage()
const chatStore = useChatStore()

const inputContent = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

// Search state
const searchQuery = ref('')
const searchResults = ref<SearchResult[]>([])
const searching = ref(false)
const showSearch = ref(false)

async function handleSearch() {
  if (!searchQuery.value.trim()) return
  searching.value = true
  searchResults.value = []
  try {
    const res = await aiSearch(searchQuery.value)
    searchResults.value = res.data.results || []
  } catch (e: any) {
    message.error(e.response?.data?.error || e.message || '搜索失败')
  } finally {
    searching.value = false
  }
}

function toggleSearch() {
  showSearch.value = !showSearch.value
  if (!showSearch.value) {
    searchResults.value = []
    searchQuery.value = ''
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTo({
        top: messagesContainer.value.scrollHeight,
        behavior: 'smooth'
      })
    }
  })
}

watch(() => chatStore.messages[chatStore.messages.length - 1]?.content, () => {
  scrollToBottom()
})

async function handleCreate() {
  await chatStore.create()
  scrollToBottom()
}

function handleSelect(conv: Conversation) {
  chatStore.selectConversation(conv)
  scrollToBottom()
}

async function handleSend() {
  if (!inputContent.value.trim() || !chatStore.currentConv) return

  const content = inputContent.value
  inputContent.value = ''
  scrollToBottom()

  await chatStore.send(content)
  scrollToBottom()

  if (chatStore.error) {
    message.error(chatStore.error)
  }
}

async function handleSaveInsight() {
  if (!chatStore.currentConv || chatStore.messages.length === 0) return

  const lastAssistantMsg = [...chatStore.messages].reverse().find(m => m.role === 'assistant')
  if (!lastAssistantMsg) {
    message.warning('没有可保存的 AI 回复')
    return
  }

  try {
    await saveInsight({
      content: lastAssistantMsg.content,
      note_id: chatStore.currentConv.note_id || undefined
    })
    message.success('洞察已保存到活动日志')
  } catch (e: any) {
    message.error('保存失败: ' + (e.response?.data?.error || e.message))
  }
}

async function handleSaveAsOutput() {
  if (!chatStore.currentConv || chatStore.messages.length === 0) return

  // Find the last user message as the query
  const lastUserMsg = [...chatStore.messages].reverse().find(m => m.role === 'user')
  if (!lastUserMsg) {
    message.warning('没有找到可保存的对话')
    return
  }

  try {
    await runQuery(lastUserMsg.content, true)
    message.success('已保存为 Output')
  } catch (e: any) {
    message.error('保存失败: ' + (e.response?.data?.error || e.message))
  }
}

async function handleDelete(conv: Conversation) {
  try {
    await chatStore.remove(conv.id)
    message.success('对话已删除')
  } catch (e: any) {
    message.error('删除失败: ' + (e.response?.data?.error || e.message))
  }
}

onMounted(async () => {
  await chatStore.fetchConversations()

  const noteId = route.query.noteId as string | undefined
  if (noteId) {
    await chatStore.create(noteId)
  } else if (chatStore.conversations.length > 0) {
    chatStore.selectConversation(chatStore.conversations[0]!)
  }
})
</script>

<template>
  <div class="chat-layout">
    <!-- 左栏：对话列表 -->
    <div class="conv-sidebar">
      <div class="conv-header">
        <NButton type="primary" block @click="handleCreate" class="btn-new-conv">
          新对话
        </NButton>
      </div>
      <div class="conv-list">
        <div
          v-for="conv in chatStore.conversations"
          :key="conv.id"
          class="conv-item"
          :class="{ 'conv-active': chatStore.currentConv?.id === conv.id }"
          @click="handleSelect(conv)"
        >
          <div class="conv-item-row">
            <div class="conv-item-title">{{ conv.title || '新对话' }}</div>
            <button
              class="conv-delete-btn"
              @click.stop="handleDelete(conv)"
              title="删除对话"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          <div class="conv-item-time">{{ conv.updated_at }}</div>
        </div>
        <NEmpty v-if="chatStore.conversations.length === 0" description="暂无对话" class="empty-conv" />
      </div>
    </div>

    <!-- 右栏：搜索 + 聊天 -->
    <div class="chat-main" v-if="chatStore.currentConv">
      <!-- 搜索栏 -->
      <div class="search-strip">
        <NInput
          v-model:value="searchQuery"
          placeholder="搜索笔记..."
          size="small"
          @keydown.enter="handleSearch"
          class="search-input"
        >
          <template #prefix>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" opacity="0.5">
              <circle cx="11" cy="11" r="7" stroke="currentColor" stroke-width="2"/>
              <path d="m20 20-4-4" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </template>
        </NInput>
        <NButton size="small" :loading="searching" @click="handleSearch">搜索</NButton>
      </div>

      <!-- 搜索结果 -->
      <div v-if="searchResults.length > 0" class="search-results">
        <NCard
          v-for="result in searchResults"
          :key="result.note_id"
          class="result-card"
          hoverable
          size="small"
          @click="$router.push(`/?note=${result.note_id}`)"
        >
          <div class="result-header">
            <NText strong>{{ result.title }}</NText>
            <span class="match-badge">AI匹配</span>
          </div>
          <div class="result-reason">{{ result.reason }}</div>
        </NCard>
      </div>

      <!-- 消息区 -->
      <div ref="messagesContainer" class="messages-area">
        <TransitionGroup name="msg" tag="div" class="messages-inner">
          <div
            v-for="(msg, idx) in chatStore.messages"
            :key="idx"
            class="msg-row"
            :class="msg.role === 'user' ? 'msg-row-user' : 'msg-row-assistant'"
          >
            <div class="msg-avatar" :class="msg.role === 'user' ? 'avatar-user' : 'avatar-ai'">
              <template v-if="msg.role === 'assistant'">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </template>
              <template v-else>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                  <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                  <circle cx="12" cy="7" r="4" stroke="currentColor" stroke-width="2"/>
                </svg>
              </template>
            </div>
            <div class="msg-bubble" :class="msg.role === 'user' ? 'bubble-user' : 'bubble-assistant'">
              <div class="msg-content">{{ msg.content }}</div>
              <NSpin v-if="msg.role === 'assistant' && chatStore.streaming && idx === chatStore.messages.length - 1 && !msg.content" size="small">
                <template #description><span style="color: var(--color-text-tertiary)">正在输入...</span></template>
              </NSpin>
              <span v-if="msg.role === 'assistant' && chatStore.streaming && idx === chatStore.messages.length - 1 && msg.content" class="typing-cursor" />
            </div>
          </div>
        </TransitionGroup>

        <div v-if="chatStore.messages.length === 0" class="chat-empty">
          <div class="chat-empty-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" opacity="0.15">
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" stroke="currentColor" stroke-width="1.5"/>
            </svg>
          </div>
          <p>开始一段新对话</p>
        </div>
      </div>

      <!-- 输入区 -->
      <div class="chat-input-area">
        <div class="chat-input-wrapper">
          <NInput
            v-model:value="inputContent"
            type="textarea"
            :autosize="{ minRows: 1, maxRows: 4 }"
            placeholder="输入消息... (Enter 发送)"
            @keydown.enter.exact.prevent="handleSend"
            :disabled="chatStore.streaming"
            class="chat-input"
          />
          <NButton
            type="primary"
            @click="handleSend"
            :disabled="!inputContent.trim() || chatStore.streaming"
            :loading="chatStore.streaming"
            class="btn-send"
          >
            <template #icon>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </template>
          </NButton>
          <NButton
            size="small"
            quaternary
            @click="handleSaveInsight"
            :disabled="chatStore.messages.length === 0"
            title="保存最后一条AI回复为洞察"
          >
            <template #icon>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                <path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </template>
          </NButton>
          <NButton
            size="small"
            quaternary
            @click="handleSaveAsOutput"
            :disabled="chatStore.messages.length === 0"
            title="保存对话为 Output"
          >
            <template #icon>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                <polyline points="14 2 14 8 20 8" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                <line x1="16" y1="13" x2="8" y2="13" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                <line x1="16" y1="17" x2="8" y2="17" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </template>
          </NButton>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="chat-main chat-empty-main">
      <NEmpty description="选择或创建一个对话" />
    </div>
  </div>
</template>

<style scoped>
.chat-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--color-bg-primary);
}

/* === 左栏 === */
.conv-sidebar {
  width: 260px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--color-border);
  flex-shrink: 0;
  background: rgba(15, 15, 26, 0.5);
}

.conv-header {
  padding: 16px;
  border-bottom: 1px solid var(--color-border);
}

.btn-new-conv {
  background: var(--button-primary-bg) !important;
  border: none !important;
  border-radius: 8px !important;
  font-weight: 500;
}

.conv-list {
  flex: 1;
  overflow: auto;
  padding: 8px;
}

.conv-item {
  padding: 12px 14px;
  border-radius: 8px;
  cursor: pointer;
  transition: all var(--transition-fast);
  margin-bottom: 2px;
  border: 1px solid transparent;
}

.conv-item:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: var(--color-border);
}

.conv-item-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.conv-delete-btn {
  opacity: 0;
  padding: 2px;
  background: none;
  border: none;
  color: var(--color-text-tertiary);
  cursor: pointer;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.conv-item:hover .conv-delete-btn {
  opacity: 1;
}

.conv-delete-btn:hover {
  background: rgba(239, 68, 68, 0.2);
  color: #ef4444;
}

.conv-active {
  background: rgba(106, 13, 173, 0.1);
  border-color: rgba(106, 13, 173, 0.25);
}

.conv-item-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.conv-item-time {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.empty-conv {
  padding: 48px 24px;
}

/* === 右栏 === */
.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.chat-empty-main {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* === 搜索栏 === */
.search-strip {
  display: flex;
  gap: 8px;
  padding: 12px 24px;
  border-bottom: 1px solid var(--color-border);
  background: rgba(15, 15, 26, 0.3);
}

.search-strip .search-input {
  flex: 1;
}

/* === 搜索结果 === */
.search-results {
  padding: 12px 24px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-bottom: 1px solid var(--color-border);
  max-height: 300px;
  overflow-y: auto;
}

.result-card {
  cursor: pointer;
  transition: all 0.2s;
  border-radius: 8px !important;
  border: 1px solid var(--color-border) !important;
  background: rgba(30, 30, 46, 0.3) !important;
}

.result-card:hover {
  border-color: var(--color-text-accent) !important;
  transform: translateY(-1px);
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.match-badge {
  display: inline-block;
  padding: 2px 8px;
  background: var(--gradient-accent);
  color: white;
  font-size: 11px;
  font-weight: 500;
  border-radius: 4px;
  flex-shrink: 0;
}

.result-reason {
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

/* === 消息区 === */
.messages-area {
  flex: 1;
  overflow: auto;
  padding: 24px;
}

.messages-inner {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.msg-row {
  display: flex;
  gap: 12px;
  max-width: 80%;
}

.msg-row-user {
  flex-direction: row-reverse;
  align-self: flex-end;
}

.msg-row-assistant {
  align-self: flex-start;
}

.msg-avatar {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 2px;
}

.avatar-user {
  background: var(--gradient-accent);
  color: white;
}

.avatar-ai {
  background: rgba(255, 255, 255, 0.08);
  color: var(--color-text-tertiary);
  border: 1px solid var(--color-border);
}

.msg-bubble {
  padding: 12px 16px;
  border-radius: 12px;
  line-height: 1.6;
  font-size: 14px;
}

.bubble-user {
  background: var(--gradient-accent);
  color: white;
  border-bottom-right-radius: 4px;
}

.bubble-assistant {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--color-border);
  color: var(--color-text-primary);
  border-bottom-left-radius: 4px;
}

.msg-content {
  word-wrap: break-word;
  white-space: pre-wrap;
}

.typing-cursor {
  display: inline-block;
  width: 2px;
  height: 16px;
  background: var(--color-text-accent);
  margin-left: 2px;
  vertical-align: text-bottom;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  50% { opacity: 0; }
}

.chat-empty {
  text-align: center;
  padding: 64px 24px;
  color: var(--color-text-tertiary);
}

.chat-empty-icon {
  margin-bottom: 12px;
}

/* === 输入区 === */
.chat-input-area {
  padding: 16px 24px;
  border-top: 1px solid var(--color-border);
}

.chat-input-wrapper {
  display: flex;
  gap: 12px;
  max-width: 800px;
  margin: 0 auto;
  align-items: flex-end;
}

.chat-input {
  flex: 1;
}

.btn-send {
  border-radius: 10px !important;
  width: 40px;
  height: 40px;
  padding: 0 !important;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* === 消息动画 === */
.msg-enter-active {
  transition: all 0.3s ease;
}
.msg-enter-from {
  opacity: 0;
  transform: translateY(12px);
}
</style>
