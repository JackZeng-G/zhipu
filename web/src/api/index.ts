import axios from 'axios'

const api = axios.create({ baseURL: '/api' })

// NAS
export const connectNAS = (data: { host: string; port: number; username: string; password: string; otp_code?: string }) =>
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
export const deleteConversation = (id: number) =>
  api.delete(`/ai/conversations/${id}`)
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

// AI Config
export const getAIConfigs = () => api.get('/ai/configs')
export const createAIConfig = (data: { provider: string; name: string; base_url?: string; model: string; api_key?: string }) =>
  api.post('/ai/configs', data)
export const updateAIConfig = (id: number, data: { provider: string; name: string; base_url?: string; model: string; api_key?: string }) =>
  api.put(`/ai/configs/${id}`, data)
export const activateAIConfig = (id: number) => api.put(`/ai/configs/${id}/activate`)
export const deleteAIConfig = (id: number) => api.delete(`/ai/configs/${id}`)
export const testAIConfig = (id: number) => api.post(`/ai/configs/${id}/test`)

// Knowledge
export const triggerIndex = (noteId?: string) => api.post('/ai/index', { note_id: noteId || '' })
export const resetIndexes = () => api.delete('/ai/index')
export const getIndexStatus = () => api.get('/ai/index/status')
export const getNoteSummary = (id: string) => api.get(`/notes/${id}/summary`)
export const getNoteEntities = (id: string) => api.get(`/ai/notes/${id}/entities`)
export const getRelatedNotes = (id: string) => api.get(`/ai/notes/${id}/related`)

// Wiki
export const getWikiPages = () => api.get('/wiki/pages')
export const getWikiPage = (slug: string) => api.get(`/wiki/pages/${slug}`)
export const getWikiCatalog = () => api.get('/wiki/catalog')
export const generateWikiPage = (data: { note_ids?: string[]; entity?: string; page_type?: string }) =>
  api.post('/wiki/generate', data)
export const deleteWikiPage = (slug: string) => api.delete(`/wiki/pages/${slug}`)
export const getWikiConcepts = () => api.get('/wiki/concepts')
export const getWikiConcept = (slug: string) => api.get(`/wiki/concepts/${slug}`)
export const deleteWikiConcept = (slug: string) => api.delete(`/wiki/concepts/${slug}`)
export const getConceptGraph = () => api.get('/wiki/graph')
export const refreshConceptGraph = () => api.post('/wiki/graph/refresh')
export const getWikiEntities = () => api.get('/wiki/entities')
export const autoGenerateWiki = () => api.post('/wiki/auto')

// Lint & Timeline
export const runLint = () => api.get('/ai/lint')
export const fixLintIssues = (issues: any[]) => api.post('/ai/lint/fix', { issues })
export const getActivities = () => api.get('/ai/activities')
export const saveInsight = (data: { content: string; note_id?: string; related_ids?: string[] }) =>
  api.post('/ai/insights', data)
export const analyzeChatForInsights = (messages: any[]) => api.post('/ai/insights/analyze', { messages })

// === Questions ===
export const createQuestion = (question: string) =>
  api.post('/questions', { question })

export const listQuestions = (status?: string) =>
  api.get('/questions', { params: { status } })

export const resolveQuestion = (id: number, outputSlug?: string) =>
  api.put(`/questions/${id}/resolve`, { output_slug: outputSlug })

// === Outputs ===
export const runQuery = (query: string, save = false) =>
  api.post('/ai/query', { query, save })

export const listOutputs = (type?: string) =>
  api.get('/ai/outputs', { params: { type } })

export const getOutput = (slug: string) =>
  api.get(`/ai/outputs/${slug}`)

export const deleteOutput = (slug: string) =>
  api.delete(`/ai/outputs/${slug}`)

// === REFLECT ===
export const runReflect = () =>
  api.post('/ai/reflect')

export const getReflectStatus = () =>
  api.get('/ai/reflect/status')

// === Confidence ===
export const confirmConceptConfidence = (slug: string) =>
  api.put(`/wiki/concepts/${slug}/confirm-confidence`)

// === MERGE ===
export const getMergeCandidates = () =>
  api.get('/wiki/merge/candidates')

export const executeMerge = (sourceSlug: string, targetSlug: string) =>
  api.post('/wiki/merge', { source_slug: sourceSlug, target_slug: targetSlug })
