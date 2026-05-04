import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getNoteSummary, getNoteEntities, getRelatedNotes,
  triggerIndex, getIndexStatus,
  getWikiConcepts, getWikiConcept, getConceptGraph, refreshConceptGraph
} from '@/api'

export interface Entity {
  id: number
  note_id: string
  entity_type: string
  entity_name: string
  description: string
}

export interface NoteSummary {
  id: number
  note_id: string
  summary: string
  key_points: string
  generated_at: number
}

export interface RelatedNote {
  note_id: string
  relation_type: string
  reason: string
  confidence: number
}

export interface IndexStatus {
  total_notes: number
  indexed_notes: number
  total_entities: number
  total_relations: number
  active_provider: string
  active_model: string
}

export interface WikiConcept {
  id: number
  slug: string
  title: string
  aliases: string
  definition: string
  key_points: string
  content: string
  note_ids: string
  source_count: number
  confidence: string
  confidence_pending?: boolean
  evolution_log: string
  contradictions: string
  created_at: number
  updated_at: number
}

export interface GraphNode {
  id: string
  label: string
  source_count: number
}

export interface GraphEdge {
  source: string
  target: string
  weight: number
}

export interface ConceptGraph {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export const useKnowledgeStore = defineStore('knowledge', () => {
  const entities = ref<Entity[]>([])
  const summary = ref<NoteSummary | null>(null)
  const relatedNotes = ref<RelatedNote[]>([])
  const indexStatus = ref<IndexStatus | null>(null)
  const loading = ref(false)

  // Wiki concept state
  const concepts = ref<WikiConcept[]>([])
  const currentConcept = ref<WikiConcept | null>(null)
  const graph = ref<ConceptGraph>({ nodes: [], edges: [] })
  const conceptsLoading = ref(false)
  const graphLoading = ref(false)

  async function fetchEntities(noteId: string) {
    try {
      const res = await getNoteEntities(noteId)
      entities.value = res.data.entities || []
    } catch {
      entities.value = []
    }
  }

  async function fetchSummary(noteId: string) {
    try {
      const res = await getNoteSummary(noteId)
      summary.value = res.data
    } catch {
      summary.value = null
    }
  }

  async function fetchRelated(noteId: string) {
    try {
      const res = await getRelatedNotes(noteId)
      relatedNotes.value = res.data.related || []
    } catch {
      relatedNotes.value = []
    }
  }

  async function fetchAll(noteId: string) {
    loading.value = true
    await Promise.all([
      fetchEntities(noteId),
      fetchSummary(noteId),
      fetchRelated(noteId)
    ])
    loading.value = false
  }

  async function fetchIndexStatus() {
    try {
      const res = await getIndexStatus()
      indexStatus.value = res.data
    } catch {
      // ignore
    }
  }

  async function doIndex(noteId?: string) {
    try {
      await triggerIndex(noteId)
    } catch (e: any) {
      throw e
    }
  }

  // Wiki concept methods
  async function fetchConcepts() {
    conceptsLoading.value = true
    try {
      const res = await getWikiConcepts()
      concepts.value = res.data.concepts || []
    } catch {
      concepts.value = []
    } finally {
      conceptsLoading.value = false
    }
  }

  async function fetchConcept(slug: string) {
    currentConcept.value = null
    conceptsLoading.value = true
    try {
      const res = await getWikiConcept(slug)
      currentConcept.value = res.data
    } catch {
      currentConcept.value = null
    } finally {
      conceptsLoading.value = false
    }
  }

  async function fetchGraph() {
    graphLoading.value = true
    try {
      const res = await getConceptGraph()
      graph.value = res.data
    } catch {
      graph.value = { nodes: [], edges: [] }
    } finally {
      graphLoading.value = false
    }
  }

  async function doRefreshGraph() {
    graphLoading.value = true
    try {
      await refreshConceptGraph()
      await fetchGraph()
    } catch (e: any) {
      graphLoading.value = false
      throw e
    }
  }

  function clear() {
    entities.value = []
    summary.value = null
    relatedNotes.value = []
  }

  return {
    entities, summary, relatedNotes, indexStatus, loading,
    fetchEntities, fetchSummary, fetchRelated, fetchAll,
    fetchIndexStatus, doIndex, clear,
    // Wiki concepts
    concepts, currentConcept, graph, conceptsLoading, graphLoading,
    fetchConcepts, fetchConcept, fetchGraph, doRefreshGraph
  }
})
