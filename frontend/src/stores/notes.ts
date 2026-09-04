import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getNotebooks, getNotes, getNote, summarizeNote, aiEdit,
  createNotebook, moveNotebookToStack, renameStack, deleteStack, getStacks
} from '@/api'

export interface Notebook {
  id: string
  title: string
  stack: string | null
  parent_id: string | null
  children?: Notebook[]
}

export interface Note {
  id: string
  title: string
  notebook_id: string
  modified_time: number
  created_time: number
}

export interface NoteDetail extends Note {
  content_html: string
  content_text: string | null
}

export const useNotesStore = defineStore('notes', () => {
  const notebooks = ref<Notebook[]>([])
  const notes = ref<Note[]>([])
  const currentNote = ref<NoteDetail | null>(null)
  const currentNotebookId = ref<string | null>(null)
  const loading = ref(false)
  const error = ref('')
  const pagination = ref({
    page: 1,
    pageSize: 20,
    total: 0
  })

  async function fetchNotebooks() {
    try {
      const res = await getNotebooks()
      notebooks.value = res.data || []
    } catch (e: any) {
      error.value = e.message || 'Failed to fetch notebooks'
    }
  }

  async function fetchNotes(notebookId?: string, page = 1, pageSize = 20) {
    loading.value = true
    currentNotebookId.value = notebookId || null
    try {
      const res = await getNotes({
        notebook_id: notebookId,
        page,
        page_size: pageSize
      })
      notes.value = res.data.items || []
      pagination.value = {
        page,
        pageSize,
        total: res.data.total || 0
      }
    } catch (e: any) {
      error.value = e.message || 'Failed to fetch notes'
    } finally {
      loading.value = false
    }
  }

  async function fetchNote(id: string) {
    loading.value = true
    try {
      const res = await getNote(id)
      currentNote.value = res.data
    } catch (e: any) {
      error.value = e.message || 'Failed to fetch note'
    } finally {
      loading.value = false
    }
  }

  async function summarize(id: string) {
    try {
      const res = await summarizeNote(id)
      return res.data.summary
    } catch (e: any) {
      error.value = e.message || 'Failed to summarize'
      return null
    }
  }

  async function edit(noteId: string, instruction: string, selectedText?: string) {
    try {
      const res = await aiEdit(noteId, instruction, selectedText)
      return res.data.edited_text
    } catch (e: any) {
      error.value = e.message || 'Failed to edit'
      return null
    }
  }

  async function doCreateNotebook(title: string, stack?: string) {
    try {
      await createNotebook({ title, stack })
      await fetchNotebooks()
      return true
    } catch (e: any) {
      error.value = e.response?.data?.error || e.message || 'Failed to create notebook'
      return false
    }
  }

  async function doMoveNotebookToStack(notebookId: string, stack: string) {
    try {
      await moveNotebookToStack(notebookId, stack)
      await fetchNotebooks()
      return true
    } catch (e: any) {
      error.value = e.response?.data?.error || e.message || 'Failed to move notebook'
      return false
    }
  }

  async function doRenameStack(oldName: string, newName: string) {
    try {
      await renameStack(oldName, newName)
      await fetchNotebooks()
      return true
    } catch (e: any) {
      error.value = e.response?.data?.error || e.message || 'Failed to rename stack'
      return false
    }
  }

  async function doDeleteStack(name: string) {
    try {
      await deleteStack(name)
      await fetchNotebooks()
      return true
    } catch (e: any) {
      error.value = e.response?.data?.error || e.message || 'Failed to delete stack'
      return false
    }
  }

  async function fetchStackList() {
    try {
      const res = await getStacks()
      return (res.data || []) as string[]
    } catch {
      return []
    }
  }

  return {
    notebooks,
    notes,
    currentNote,
    currentNotebookId,
    loading,
    error,
    pagination,
    fetchNotebooks,
    fetchNotes,
    fetchNote,
    summarize,
    edit,
    doCreateNotebook,
    doMoveNotebookToStack,
    doRenameStack,
    doDeleteStack,
    fetchStackList
  }
})
