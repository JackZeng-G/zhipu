<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed, nextTick } from 'vue'
import Sigma from 'sigma'
import Graph from 'graphology'
import type { GraphNode, GraphEdge } from '@/stores/knowledge'

const props = defineProps<{
  centerSlug: string
  nodes: GraphNode[]
  edges: GraphEdge[]
}>()

const emit = defineEmits<{
  select: [slug: string]
}>()

const containerRef = ref<HTMLElement | null>(null)
let sigmaInstance: Sigma | null = null

// Compute 2-hop neighborhood
const localData = computed(() => {
  const slug = props.centerSlug
  if (!slug || props.edges.length === 0) return { nodes: [] as GraphNode[], edges: [] as GraphEdge[] }

  const nodeMap = new Map(props.nodes.map(n => [n.id, n]))

  // 1-hop neighbors
  const hop1 = new Set<string>([slug])
  for (const e of props.edges) {
    if (e.source === slug) hop1.add(e.target)
    if (e.target === slug) hop1.add(e.source)
  }
  // 2-hop neighbors
  const hop2 = new Set(hop1)
  for (const e of props.edges) {
    if (hop1.has(e.source)) hop2.add(e.target)
    if (hop1.has(e.target)) hop2.add(e.source)
  }

  const localEdges = props.edges.filter(e => hop2.has(e.source) && hop2.has(e.target))
  const localNodeIds = new Set<string>()
  for (const e of localEdges) {
    localNodeIds.add(e.source)
    localNodeIds.add(e.target)
  }
  localNodeIds.add(slug)

  const localNodes = [...localNodeIds]
    .map(id => nodeMap.get(id))
    .filter((n): n is GraphNode => !!n)

  return { nodes: localNodes, edges: localEdges }
})

function buildGraph() {
  if (sigmaInstance) {
    sigmaInstance.kill()
    sigmaInstance = null
  }

  const container = containerRef.value
  if (!container) return

  const { nodes, edges } = localData.value
  if (nodes.length === 0 || !props.centerSlug) return

  const graph = new Graph({ type: 'undirected' })

  for (const n of nodes) {
    const isCenter = n.id === props.centerSlug
    graph.addNode(n.id, {
      label: n.label,
      size: isCenter ? 12 : 6,
      color: isCenter ? '#3b82f6' : '#6a8caf',
      x: Math.random(),
      y: Math.random(),
    })
  }

  for (const e of edges) {
    if (graph.hasNode(e.source) && graph.hasNode(e.target) && !graph.hasEdge(e.source, e.target)) {
      graph.addUndirectedEdge(e.source, e.target, { weight: e.weight })
    }
  }

  // Circular layout around center
  const centerNode = props.centerSlug
  const neighbors = graph.neighbors(centerNode)
  const hop1Set = new Set(neighbors)
  const hop1Nodes = neighbors.filter(n => n !== centerNode)
  const hop2Nodes = graph.nodes().filter(n => !hop1Set.has(n) && n !== centerNode)

  // Place center in middle
  graph.setNodeAttribute(centerNode, 'x', 0.5)
  graph.setNodeAttribute(centerNode, 'y', 0.5)

  // 1-hop: inner ring
  hop1Nodes.forEach((n, i) => {
    const angle = (2 * Math.PI * i) / Math.max(hop1Nodes.length, 1) - Math.PI / 2
    graph.setNodeAttribute(n, 'x', 0.5 + 0.3 * Math.cos(angle))
    graph.setNodeAttribute(n, 'y', 0.5 + 0.3 * Math.sin(angle))
  })

  // 2-hop: outer ring
  hop2Nodes.forEach((n, i) => {
    const angle = (2 * Math.PI * i) / Math.max(hop2Nodes.length, 1) - Math.PI / 2
    graph.setNodeAttribute(n, 'x', 0.5 + 0.45 * Math.cos(angle))
    graph.setNodeAttribute(n, 'y', 0.5 + 0.45 * Math.sin(angle))
  })

  sigmaInstance = new Sigma(graph, container, {
    renderLabels: true,
    labelFont: 'system-ui, "Microsoft YaHei", sans-serif',
    labelSize: 10,
    labelWeight: '400',
    labelColor: { color: '#7fdbff' },
    minEdgeThickness: 0.5,
    stagePadding: 16,
    renderEdgeLabels: false,
    enableEdgeEvents: false,
    labelRenderedSizeThreshold: 0,
    labelDensity: 10,
    defaultEdgeType: 'arrow',
    enableCameraZooming: false,
    enableCameraPanning: false,
    enableCameraRotation: false,
    nodeReducer: (node, data) => {
      const res = { ...data }
      // Dim hop2 nodes slightly
      if (!hop1Set.has(node) && node !== centerNode) {
        res.color = `${data.color}60`
      }
      return res
    },
    edgeReducer: (edge, data) => {
      const src = graph.source(edge)
      const tgt = graph.target(edge)
      const isCenterEdge = src === centerNode || tgt === centerNode
      const res = { ...data }
      if (isCenterEdge) {
        res.color = 'rgba(100, 160, 220, 0.4)'
        res.size = 1
      }
      return res
    },
  })

  sigmaInstance.on('clickNode', ({ node }) => {
    if (node !== props.centerSlug) {
      emit('select', node)
    }
  })
}

watch(
  () => [props.centerSlug, props.nodes, props.edges],
  () => nextTick(() => buildGraph()),
  { deep: true }
)

onMounted(() => nextTick(() => buildGraph()))

onUnmounted(() => {
  if (sigmaInstance) {
    sigmaInstance.kill()
    sigmaInstance = null
  }
})
</script>

<template>
  <div ref="containerRef" class="local-sigma-container" />
</template>

<style scoped>
.local-sigma-container {
  width: 100%;
  height: 180px;
  background: #0f0f1a;
}
</style>
