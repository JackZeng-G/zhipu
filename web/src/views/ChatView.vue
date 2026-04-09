<script setup lang="ts">
import { ref, nextTick, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  NButton,
  NInput,
  NSpace,
  NText,
  NSpin,
  NEmpty,
  useMessage
} from 'naive-ui'
import { useChatStore, type Conversation } from '@/stores/chat'

const route = useRoute()
const message = useMessage()
const chatStore = useChatStore()

const inputContent = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

function scrollToBottom() {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

watch(() => chatStore.messages.length, () => {
  scrollToBottom()
})

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

onMounted(async () => {
  await chatStore.fetchConversations()

  // If navigated with a noteId, create a new conversation
  const noteId = route.query.noteId as string | undefined
  if (noteId) {
    await chatStore.create(noteId)
  } else if (chatStore.conversations.length > 0) {
    chatStore.selectConversation(chatStore.conversations[0]!)
  }
})
</script>

<template>
  <div style="height: 100vh; display: flex; overflow: hidden">
    <!-- Left column: Conversation list -->
    <div style="width: 250px; border-right: 1px solid #e0e0e0; display: flex; flex-direction: column">
      <div style="padding: 12px">
        <n-button type="primary" block @click="handleCreate">\u65B0\u5BF9\u8BDD</n-button>
      </div>
      <div style="flex: 1; overflow: auto">
        <div
          v-for="conv in chatStore.conversations"
          :key="conv.id"
          @click="handleSelect(conv)"
          :class="{ 'conv-selected': chatStore.currentConv?.id === conv.id }"
          style="padding: 12px; cursor: pointer; border-bottom: 1px solid #f0f0f0"
        >
          <n-text>{{ conv.title || '\u65B0\u5BF9\u8BDD' }}</n-text>
          <br />
          <n-text depth="3" style="font-size: 12px">{{ conv.updated_at }}</n-text>
        </div>
        <n-empty v-if="chatStore.conversations.length === 0" description="\u6682\u65E0\u5BF9\u8BDD" style="padding: 48px" />
      </div>
    </div>

    <!-- Right column: Chat interface -->
    <div v-if="chatStore.currentConv" style="flex: 1; display: flex; flex-direction: column">
      <!-- Messages area -->
      <div ref="messagesContainer" style="flex: 1; overflow: auto; padding: 16px">
        <div
          v-for="(msg, idx) in chatStore.messages"
          :key="idx"
          :class="msg.role === 'user' ? 'msg-user' : 'msg-assistant'"
        >
          <div :class="msg.role === 'user' ? 'msg-bubble-user' : 'msg-bubble-assistant'">
            <n-text>{{ msg.content }}</n-text>
            <n-spin v-if="msg.role === 'assistant' && chatStore.streaming && idx === chatStore.messages.length - 1 && !msg.content" size="small">
              <template #description>\u6B63\u5728\u8F93\u5165...</template>
            </n-spin>
          </div>
        </div>
        <div v-if="chatStore.messages.length === 0" style="text-align: center; padding: 48px">
          <n-text depth="3">\u5F00\u59CB\u4E00\u6BB5\u65B0\u5BF9\u8BDD</n-text>
        </div>
      </div>

      <!-- Input area -->
      <div style="padding: 12px; border-top: 1px solid #e0e0e0; display: flex; gap: 8px">
        <n-input
          v-model:value="inputContent"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 4 }"
          placeholder="\u8F93\u5165\u6D88\u606F..."
          @keydown.enter.exact.prevent="handleSend"
          :disabled="chatStore.streaming"
        />
        <n-button
          type="primary"
          @click="handleSend"
          :disabled="!inputContent.trim() || chatStore.streaming"
          :loading="chatStore.streaming"
        >
          \u53D1\u9001
        </n-button>
      </div>
    </div>
    <div v-else style="flex: 1; display: flex; align-items: center; justify-content: center">
      <n-empty description="\u9009\u62E9\u6216\u521B\u5EFA\u4E00\u4E2A\u5BF9\u8BDD" />
    </div>
  </div>
</template>

<style scoped>
.msg-user {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.msg-assistant {
  display: flex;
  justify-content: flex-start;
  margin-bottom: 12px;
}

.msg-bubble-user {
  background-color: #2080f0;
  color: white;
  padding: 10px 14px;
  border-radius: 12px;
  max-width: 70%;
  word-wrap: break-word;
}

.msg-bubble-user .n-text {
  color: white;
}

.msg-bubble-assistant {
  background-color: #f0f0f0;
  padding: 10px 14px;
  border-radius: 12px;
  max-width: 70%;
  word-wrap: break-word;
}

.conv-selected {
  background-color: #e8f4fd;
}
</style>
