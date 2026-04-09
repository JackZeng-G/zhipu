import axios from 'axios'

const api = axios.create({ baseURL: '/api' })

// NAS
export const connectNAS = (data: { host: string; port: number; username: string; password: string }) =>
  api.post('/nas/connect', data)
export const disconnectNAS = () => api.post('/nas/disconnect')
export const getNASStatus = () => api.get('/nas/status')
export const syncNAS = () => api.post('/nas/sync')

// Notes
export const getNotebooks = () => api.get('/notebooks')
export const getNotes = (params: { notebook_id?: string; page?: number; page_size?: number }) =>
  api.get('/notes', { params })
export const getNote = (id: string) => api.get(`/notes/${id}`)

// AI
export const summarizeNote = (id: string) => api.post(`/ai/summarize/${id}`)
export const aiSearch = (query: string) => api.post('/ai/search', { query })
export const aiEdit = (noteId: string, instruction: string, selectedText?: string) =>
  api.post('/ai/edit', { note_id: noteId, instruction, selected_text: selectedText })

// Conversations
export const getConversations = () => api.get('/ai/conversations')
export const createConversation = (noteId?: string) =>
  api.post('/ai/conversations', { note_id: noteId })
export const sendMessage = async (convId: number, content: string, onChunk: (chunk: string) => void) => {
  const response = await fetch(`/api/ai/conversations/${convId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content })
  })
  const reader = response.body!.getReader()
  const decoder = new TextDecoder()
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    const text = decoder.decode(value)
    // Parse SSE data lines
    const lines = text.split('\n')
    for (const line of lines) {
      if (line.startsWith('data: ')) {
        try {
          const data = JSON.parse(line.slice(6))
          if (data.chunk) onChunk(data.chunk)
        } catch {
          // skip malformed lines
        }
      }
    }
  }
}

// Settings
export const getSettings = () => api.get('/settings')
export const updateSettings = (data: Record<string, string>) => api.put('/settings', data)
