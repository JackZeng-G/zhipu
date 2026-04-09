import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getConversations, createConversation, sendMessage } from '@/api'

export interface Conversation {
  id: number
  title: string
  note_id?: string
  created_at: string
  updated_at: string
  messages?: Message[]
}

export interface Message {
  id?: number
  role: 'user' | 'assistant'
  content: string
  created_at?: string
}

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const currentConv = ref<Conversation | null>(null)
  const messages = ref<Message[]>([])
  const loading = ref(false)
  const streaming = ref(false)
  const error = ref('')

  async function fetchConversations() {
    try {
      const res = await getConversations()
      conversations.value = res.data || []
    } catch (e: any) {
      error.value = e.message || 'Failed to fetch conversations'
    }
  }

  async function create(noteId?: string) {
    try {
      const res = await createConversation(noteId)
      const newConv = res.data
      conversations.value.unshift(newConv)
      currentConv.value = newConv
      messages.value = []
      return newConv
    } catch (e: any) {
      error.value = e.message || 'Failed to create conversation'
      return null
    }
  }

  function selectConversation(conv: Conversation) {
    currentConv.value = conv
    messages.value = conv.messages || []
  }

  async function send(content: string) {
    if (!currentConv.value) return

    messages.value.push({
      role: 'user',
      content,
      created_at: new Date().toISOString()
    })

    streaming.value = true
    const assistantMessage: Message = {
      role: 'assistant',
      content: ''
    }
    messages.value.push(assistantMessage)

    try {
      await sendMessage(currentConv.value.id, content, (chunk) => {
        assistantMessage.content += chunk
      })
    } catch (e: any) {
      error.value = e.message || 'Failed to send message'
    } finally {
      streaming.value = false
    }
  }

  return {
    conversations,
    currentConv,
    messages,
    loading,
    streaming,
    error,
    fetchConversations,
    create,
    selectConversation,
    send
  }
})
