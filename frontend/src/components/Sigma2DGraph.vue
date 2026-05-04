<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick, computed } from 'vue'
import Sigma from 'sigma'
import Graph from 'graphology'
import forceAtlas2 from 'graphology-layout-forceatlas2'
import type { GraphNode, GraphEdge } from '@/stores/knowledge'

const props = defineProps<{
  nodes: GraphNode[]
  edges: GraphEdge[]
  selectedSlug: string | null
  minSourceCount: number
  minEdgeWeight: number
  searchQuery: string
  hideIsolated: boolean
}>()

const emit = defineEmits<{
  select: [slug: string]
  focus: [slug: string]
  unfocus: []
  'hover-node': [data: { label: string; degree: number; sourceCount: number; x: number; y: number } | null]
}>()

const containerRef = ref<HTMLElement | null>(null)
let sigmaInstance: Sigma | null = null
let graphInstance: Graph | null = null

const communityCount = ref(0)
const communityLabels = ref<Map<number, string>>(new Map())

const communityPalette = [
  '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#06b6d4',
  '#f97316', '#14b8a6', '#eab308', '#22c55e', '#0ea5e9',
]

function getCommunityColor(community: number): string {
  return communityPalette[community % communityPalette.length] ?? '#888'
}

defineExpose({
  communityCount,
  communityLabels,
  getCommunityColor,
  locateNode,
})

// Louvain community detection
function detectCommunities(nodeList: { id: string }[], edgeList: { source: string; target: string; weight: number }[]): Map<string, number> {
  if (nodeList.length === 0) return new Map()
  const nodeComm = new Map<string, number>()
  nodeList.forEach((n, i) => nodeComm.set(n.id, i))

  const adj = new Map<string, Map<string, number>>()
  for (const n of nodeList) adj.set(n.id, new Map())
  for (const e of edgeList) {
    if (!adj.has(e.source) || !adj.has(e.target)) continue
    adj.get(e.source)!.set(e.target, (adj.get(e.source)!.get(e.target) ?? 0) + e.weight)
    adj.get(e.target)!.set(e.source, (adj.get(e.target)!.get(e.source) ?? 0) + e.weight)
  }

  let m = 0
  for (const e of edgeList) m += e.weight
  if (m === 0) return nodeComm

  const degree = new Map<string, number>()
  for (const [id, neighbors] of adj) {
    let sum = 0
    for (const w of neighbors.values()) sum += w
    degree.set(id, sum)
  }

  let improved = true
  let iterations = 0
  while (improved && iterations < 20) {
    improved = false
    iterations++
    for (const node of nodeList) {
      const nid = node.id
      const currentComm = nodeComm.get(nid)!
      const neighbors = adj.get(nid)
      if (!neighbors || neighbors.size === 0) continue

      const commWeights = new Map<number, number>()
      for (const [neighbor, w] of neighbors) {
        const nc = nodeComm.get(neighbor)!
        commWeights.set(nc, (commWeights.get(nc) ?? 0) + w)
      }

      const commDegree = new Map<number, number>()
      for (const n of nodeList) {
        const c = nodeComm.get(n.id)!
        commDegree.set(c, (commDegree.get(c) ?? 0) + (degree.get(n.id) ?? 0))
      }

      const ki = degree.get(nid) ?? 0
      const ki_in_current = commWeights.get(currentComm) ?? 0
      const sigma_current = commDegree.get(currentComm) ?? 0
      const removeFromCurrent = ki_in_current / m - (sigma_current * ki) / (2 * m * m)

      let bestComm = currentComm
      let bestDelta = 0

      for (const [comm, ki_in] of commWeights) {
        if (comm === currentComm) continue
        const sigma_comm = commDegree.get(comm) ?? 0
        const addToComm = ki_in / m - (sigma_comm * ki) / (2 * m * m)
        if (addToComm - removeFromCurrent > bestDelta) {
          bestDelta = addToComm - removeFromCurrent
          bestComm = comm
        }
      }

      if (bestComm !== currentComm) {
        nodeComm.set(nid, bestComm)
        improved = true
      }
    }
  }

  const uniqueComms = new Set(nodeComm.values())
  const commRemap = new Map<number, number>()
  let idx = 0
  for (const c of uniqueComms) commRemap.set(c, idx++)
  const result = new Map<string, number>()
  for (const [nid, c] of nodeComm) result.set(nid, commRemap.get(c) ?? 0)
  return result
}

let highlightNodes = new Set<string>()
let highlightEdges = new Set<string>()
let focusedNode: string | null = null

function buildGraph() {
  if (sigmaInstance) {
    sigmaInstance.kill()
    sigmaInstance = null
  }
  graphInstance = null

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    requestAnimationFrame(() => buildGraph())
    return
  }

  // Filter nodes
  let filteredNodes = props.nodes.filter(n => n.source_count >= props.minSourceCount)
  const search = props.searchQuery.trim().toLowerCase()
  if (search) filteredNodes = filteredNodes.filter(n => n.label.toLowerCase().includes(search))
  const nodeIds = new Set(filteredNodes.map(n => n.id))

  const filteredEdges = props.edges.filter(e =>
    nodeIds.has(e.source) && nodeIds.has(e.target) && e.weight >= props.minEdgeWeight
  )

  let finalNodes = filteredNodes
  if (props.hideIsolated) {
    const connectedIds = new Set<string>()
    for (const e of filteredEdges) { connectedIds.add(e.source); connectedIds.add(e.target) }
    finalNodes = filteredNodes.filter(n => connectedIds.has(n.id))
  }

  if (finalNodes.length === 0) return

  // Community detection
  const communityMap = detectCommunities(finalNodes, filteredEdges)
  const commSet = new Set(communityMap.values())
  communityCount.value = commSet.size

  // Community labels
  const commBest = new Map<number, { label: string; score: number }>()
  for (const n of finalNodes) {
    const c = communityMap.get(n.id) ?? 0
    const best = commBest.get(c)
    if (!best || n.source_count > best.score) commBest.set(c, { label: n.label, score: n.source_count })
  }
  const labels = new Map<number, string>()
  commBest.forEach((v, k) => labels.set(k, v.label))
  communityLabels.value = labels

  // Compute degree
  const degreeMap = new Map<string, number>()
  for (const n of finalNodes) degreeMap.set(n.id, 0)
  const finalNodeIds = new Set(finalNodes.map(n => n.id))
  for (const e of filteredEdges) {
    if (finalNodeIds.has(e.source) && finalNodeIds.has(e.target)) {
      degreeMap.set(e.source, (degreeMap.get(e.source) ?? 0) + 1)
      degreeMap.set(e.target, (degreeMap.get(e.target) ?? 0) + 1)
    }
  }

  const maxSource = Math.max(...finalNodes.map(n => n.source_count))
  const maxDegree = Math.max(...finalNodes.map(n => degreeMap.get(n.id) ?? 0))

  // Build graphology graph
  const graph = new Graph({ type: 'undirected' })
  graphInstance = graph

  // 计算边的最大权重用于样式缩放
  const maxWeight = Math.max(...filteredEdges.map(e => e.weight), 1)

  for (const n of finalNodes) {
    const sourceRatio = maxSource > 0 ? n.source_count / maxSource : 0.5
    const degreeRatio = maxDegree > 0 ? (degreeMap.get(n.id) ?? 0) / maxDegree : 0.5
    // 节点大小：核心节点更大更醒目
    const size = 6 + sourceRatio * 16 + degreeRatio * 8
    const community = communityMap.get(n.id) ?? 0
    const isCore = n.source_count >= 5 || (degreeMap.get(n.id) ?? 0) >= 5

    graph.addNode(n.id, {
      label: n.label,
      size,
      color: getCommunityColor(community),
      // 核心节点有边框
      borderColor: isCore ? '#2563eb' : undefined,
      borderSize: isCore ? 1.5 : 0,
      x: Math.random() * 1000 - 500,
      y: Math.random() * 1000 - 500,
      sourceCount: n.source_count,
      degree: degreeMap.get(n.id) ?? 0,
      community,
      isCore,
    })
  }

  for (const e of filteredEdges) {
    if (graph.hasNode(e.source) && graph.hasNode(e.target) && !graph.hasEdge(e.source, e.target)) {
      const weightRatio = e.weight / maxWeight
      // 边的宽度和颜色基于权重
      const edgeSize = 0.5 + weightRatio * 2
      const edgeColor = getCommunityColor(communityMap.get(e.source) ?? 0)
      graph.addUndirectedEdge(e.source, e.target, {
        weight: e.weight,
        color: edgeColor,
        size: edgeSize,
      })
    }
  }

  // Apply ForceAtlas2 layout - 更好的聚类
  forceAtlas2.assign(graph, {
    iterations: 200,
    settings: {
      gravity: 0.8,
      scalingRatio: 25,
      strongGravityMode: false,
      linLogMode: true,
      adjustSizes: true,
    },
  })

  // 将孤立节点放到外围环形位置
  const isolatedNodes: string[] = []
  graph.forEachNode((node, attrs) => {
    if (attrs.degree === 0) {
      isolatedNodes.push(node)
    }
  })

  if (isolatedNodes.length > 0) {
    // 计算现有布局的边界
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
    graph.forEachNode((node, attrs) => {
      if (!isolatedNodes.includes(node)) {
        minX = Math.min(minX, attrs.x)
        maxX = Math.max(maxX, attrs.x)
        minY = Math.min(minY, attrs.y)
        maxY = Math.max(maxY, attrs.y)
      }
    })

    // 计算中心和外围半径
    const centerX = (minX + maxX) / 2 || 0
    const centerY = (minY + maxY) / 2 || 0
    const rangeX = maxX - minX || 100
    const rangeY = maxY - minY || 100
    const outerRadius = Math.max(rangeX, rangeY) * 0.7 + 150 // 外围半径比现有范围大70%

    // 将孤立节点放在外围环形
    isolatedNodes.forEach((node, i) => {
      const angle = (2 * Math.PI * i) / isolatedNodes.length - Math.PI / 2
      graph.setNodeAttribute(node, 'x', centerX + outerRadius * Math.cos(angle))
      graph.setNodeAttribute(node, 'y', centerY + outerRadius * Math.sin(angle))
      // 孤立节点稍微缩小，用不同样式
      graph.setNodeAttribute(node, 'size', 5)
      graph.setNodeAttribute(node, 'isolated', true)
    })
  }

  highlightNodes = new Set()
  highlightEdges = new Set()

  sigmaInstance = new Sigma(graph, container, {
    renderLabels: true,
    labelFont: 'system-ui, "Microsoft YaHei", sans-serif',
    labelSize: 14,
    labelWeight: '700',
    labelColor: { color: '#a0aab4' },
    labelRenderedSizeThreshold: 10,
    labelDensity: 10,
    labelGridCellSize: 80,
    minEdgeThickness: 0.3,
    stagePadding: 40,
    renderEdgeLabels: false,
    enableEdgeEvents: false,
    defaultEdgeType: 'line',
    // 节点渲染增强
    nodeReducer: (node, data) => {
      const res = { ...data }
      const isCore = graph.getNodeAttributes(node).isCore as boolean
      const isIsolated = graph.getNodeAttributes(node).isolated as boolean

      // 孤立节点用虚化样式
      if (isIsolated && highlightNodes.size === 0) {
        res.color = `${data.color}60`
        res.label = ''
      }

      if (highlightNodes.size > 0) {
        if (highlightNodes.has(node)) {
          res.zIndex = 10
          res.borderColor = '#2563eb'
          res.borderSize = 2
        } else {
          res.hidden = true
        }
      } else {
        if (isCore) {
          res.borderColor = 'rgba(37,99,235,0.4)'
          res.borderSize = 1.5
        }
      }
      return res
    },
    edgeReducer: (edge, data) => {
      const res = { ...data }
      const edgeAttrs = graph.getEdgeAttributes(edge)
      const weightRatio = edgeAttrs.weight / maxWeight

      if (highlightNodes.size > 0) {
        const src = graph.source(edge)
        const tgt = graph.target(edge)
        if (highlightNodes.has(src) && highlightNodes.has(tgt)) {
          res.color = `rgba(180, 220, 255, ${0.3 + weightRatio * 0.5})`
          res.size = 2 + weightRatio * 2
        } else {
          res.hidden = true
        }
      } else {
        const opacity = 0.08 + weightRatio * 0.18
        res.color = `rgba(100, 140, 200, ${opacity})`
        res.size = 0.3 + weightRatio * 1.5
      }
      return res
    },
  })

  sigmaInstance.on('enterNode', ({ node }) => {
    if (focusedNode) return  // 聚焦状态下跳过悬停高亮
    highlightNodes = new Set([node])
    graph.forEachEdge(node, (edge, _attrs, source, target) => {
      highlightNodes.add(source)
      highlightNodes.add(target)
    })
    sigmaInstance!.refresh()

    const attrs = graph.getNodeAttributes(node)
    emit('hover-node', { label: attrs.label, degree: attrs.degree, sourceCount: attrs.sourceCount, x: 0, y: 0 })
  })

  sigmaInstance.on('leaveNode', () => {
    if (focusedNode) return  // 聚焦状态下跳过悬停清除
    highlightNodes = new Set()
    highlightEdges = new Set()
    sigmaInstance!.refresh()
    emit('hover-node', null)
  })

  sigmaInstance.on('clickNode', ({ node }) => {
    if (focusedNode === node) {
      // 单击已聚焦的节点 → 取消聚焦
      focusedNode = null
      highlightNodes = new Set()
      highlightEdges = new Set()
      sigmaInstance!.refresh()
      emit('unfocus')
    } else {
      // 单击新节点 → 聚焦（只显示该节点和关联节点）
      focusedNode = node
      highlightNodes = new Set([node])
      highlightEdges = new Set()
      graph.forEachEdge(node, (edge, _attrs, source, target) => {
        highlightNodes.add(source)
        highlightNodes.add(target)
      })
      sigmaInstance!.refresh()
      emit('focus', node)
    }
  })

  // 双击 → 打开详情
  sigmaInstance.on('doubleClickNode', ({ node }) => {
    emit('select', node)
  })

  sigmaInstance.on('clickStage', () => {
    if (focusedNode) {
      focusedNode = null
      highlightNodes = new Set()
      highlightEdges = new Set()
      sigmaInstance!.refresh()
      emit('unfocus')
    }
  })

  // Initial zoom to fit
  setTimeout(() => {
    if (sigmaInstance) {
      sigmaInstance.getCamera().animatedReset({ duration: 400 })
    }
  }, 200)
}

function locateNode(slug: string) {
  if (!sigmaInstance || !graphInstance || !graphInstance.hasNode(slug)) return
  const attrs = graphInstance.getNodeAttributes(slug)
  sigmaInstance.getCamera().animate({ x: attrs.x, y: attrs.y, ratio: 0.3 }, { duration: 400 })

  highlightNodes = new Set([slug])
  graphInstance.forEachEdge(slug, (edge, _attrs, source, target) => {
    highlightNodes.add(source)
    highlightNodes.add(target)
  })
  sigmaInstance.refresh()
}

watch(
  () => [props.nodes, props.edges, props.minSourceCount, props.minEdgeWeight, props.searchQuery, props.hideIsolated],
  () => nextTick(() => requestAnimationFrame(() => buildGraph())),
  { deep: true }
)

watch(() => props.searchQuery, (search) => {
  if (search && props.nodes.length > 0) {
    const found = props.nodes.find(n => n.label.toLowerCase().includes(search.toLowerCase()) && n.source_count >= props.minSourceCount)
    if (found) nextTick(() => locateNode(found.id))
  }
})

onMounted(() => nextTick(() => requestAnimationFrame(() => buildGraph())))

onUnmounted(() => {
  if (sigmaInstance) {
    sigmaInstance.kill()
    sigmaInstance = null
  }
})
</script>

<template>
  <div ref="containerRef" class="sigma-2d-container" />
</template>

<style scoped>
.sigma-2d-container {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: #080814;
}
</style>