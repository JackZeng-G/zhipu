<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import ForceGraph3D from '3d-force-graph'
import type { ForceGraph3DInstance } from '3d-force-graph'
import * as THREE from 'three'
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
}>()

const containerRef = ref<HTMLElement | null>(null)
let graph3d: ForceGraph3DInstance | null = null

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
let highlightLinks = new Set<number>()
let focusedNode: string | null = null

function buildGraph() {
  if (graph3d) {
    graph3d._destructor()
    graph3d = null
  }

  const container = containerRef.value
  if (!container) return

  const rect = container.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    requestAnimationFrame(() => buildGraph())
    return
  }

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

  const communityMap = detectCommunities(finalNodes, filteredEdges)
  communityCount.value = new Set(communityMap.values()).size

  const commBest = new Map<number, { label: string; score: number }>()
  for (const n of finalNodes) {
    const c = communityMap.get(n.id) ?? 0
    const best = commBest.get(c)
    if (!best || n.source_count > best.score) commBest.set(c, { label: n.label, score: n.source_count })
  }
  const labels = new Map<number, string>()
  commBest.forEach((v, k) => labels.set(k, v.label))
  communityLabels.value = labels

  const degreeMap = new Map<string, number>()
  for (const n of finalNodes) degreeMap.set(n.id, 0)
  for (const e of filteredEdges) {
    degreeMap.set(e.source, (degreeMap.get(e.source) ?? 0) + 1)
    degreeMap.set(e.target, (degreeMap.get(e.target) ?? 0) + 1)
  }

  const maxSource = Math.max(...finalNodes.map(n => n.source_count))
  const maxDegree = Math.max(...finalNodes.map(n => degreeMap.get(n.id) ?? 0))

  const graphNodes = finalNodes.map(n => {
    const sourceRatio = maxSource > 0 ? n.source_count / maxSource : 0.5
    const degreeRatio = maxDegree > 0 ? (degreeMap.get(n.id) ?? 0) / maxDegree : 0.5
    return {
      id: n.id,
      label: n.label,
      sourceCount: n.source_count,
      degree: degreeMap.get(n.id) ?? 0,
      community: communityMap.get(n.id) ?? 0,
      color: getCommunityColor(communityMap.get(n.id) ?? 0),
      size: 3 + sourceRatio * 8 + degreeRatio * 4,
    }
  })

  const graphLinks = filteredEdges.map((e, i) => ({
    source: e.source,
    target: e.target,
    weight: e.weight,
    index: i,
    color: getCommunityColor(communityMap.get(e.source) ?? 0),
  }))

  highlightNodes = new Set()
  highlightLinks = new Set()

  const Graph = new ForceGraph3D(container, { controlType: 'orbit' })

  graph3d = Graph
    .graphData({ nodes: graphNodes, links: graphLinks })
    .backgroundColor('#080814')
    .nodeLabel((node: any) => `${node.label}\n${node.sourceCount} 篇文章 · ${node.degree} 个关联`)
    .nodeVal((node: any) => node.size)
    .nodeColor((node: any) => highlightNodes.size > 0 && !highlightNodes.has(node.id) ? '#333' : node.color)
    .nodeOpacity(1)
    .nodeThreeObject((node: any) => {
      const size = node.size ?? 3
      const isDimmed = highlightNodes.size > 0 && !highlightNodes.has(node.id)
      const geometry = new THREE.SphereGeometry(size, 16, 16)
      const material = new THREE.MeshLambertMaterial({
        color: new THREE.Color(node.color),
        transparent: isDimmed,
        opacity: isDimmed ? 0.15 : 1,
      })
      const mesh = new THREE.Mesh(geometry, material)

      // 标签
      if (!isDimmed) {
        const canvas = document.createElement('canvas')
        const ctx = canvas.getContext('2d')!
        canvas.width = 512
        canvas.height = 48
        ctx.font = 'bold 18px system-ui, "Microsoft YaHei"'
        const label = node.label.length > 14 ? node.label.slice(0, 13) + '…' : node.label
        const textWidth = ctx.measureText(label).width
        // 深色背景
        const px = 256 - textWidth / 2 - 10
        ctx.fillStyle = 'rgba(0,0,0,0.65)'
        ctx.fillRect(px, 4, textWidth + 20, 40)
        // 文字
        ctx.fillStyle = '#a0aab4'
        ctx.textAlign = 'center'
        ctx.textBaseline = 'middle'
        ctx.fillText(label, 256, 24)
        const texture = new THREE.CanvasTexture(canvas)
        const spriteMat = new THREE.SpriteMaterial({ map: texture, transparent: true })
        const sprite = new THREE.Sprite(spriteMat)
        sprite.scale.set(24, 6, 1)
        sprite.position.y = size + 5
        mesh.add(sprite)
      }

      return mesh
    })
    .nodeThreeObjectExtend(false)
    .linkColor((link: any) => {
      if (highlightLinks.size > 0) {
        if (highlightLinks.has(link.index)) {
          return link.color  // 高亮线使用社区颜色
        }
        return 'rgba(60,80,120,0.05)'
      }
      return 'rgba(100,140,200,0.25)'
    })
    .linkWidth((link: any) => highlightLinks.has(link.index) ? 1.5 : 0.8)
    .linkOpacity(0.35)
    .linkDirectionalParticles(3)
    .linkDirectionalParticleWidth(1.8)
    .linkDirectionalParticleColor((link: any) => link.color)
    .linkDirectionalParticleSpeed(0.006)
    .enableNodeDrag(true)
    .enableNavigationControls(true)
    .showNavInfo(false)
    .cooldownTicks(100)
    .d3AlphaDecay(0.01)
    .d3VelocityDecay(0.3)
    .onNodeClick((node: any) => {
      if (focusedNode === node.id) {
        // 再次点击已聚焦节点 → 取消聚焦并恢复初始视图
        focusedNode = null
        highlightNodes = new Set()
        highlightLinks = new Set()
        Graph.refresh()
        setTimeout(() => Graph.zoomToFit(400, 20), 100)
        emit('unfocus')
        return
      }

      // 聚焦：相机飞向节点 + 只显示关联节点
      focusedNode = node.id
      const distance = 80
      const distRatio = 1 + distance / Math.hypot(node.x, node.y, node.z)
      Graph.cameraPosition(
        { x: node.x * distRatio, y: node.y * distRatio, z: node.z * distRatio },
        { x: node.x, y: node.y, z: node.z },
        600
      )

      highlightNodes = new Set([node.id])
      highlightLinks = new Set()
      Graph.graphData().links.forEach((link: any, i: number) => {
        if (link.source.id === node.id || link.target.id === node.id) {
          highlightNodes.add(link.source.id)
          highlightNodes.add(link.target.id)
          highlightLinks.add(i)
        }
      })
      Graph.refresh()
      emit('focus', node.id)
    })
    .onNodeRightClick((node: any) => {
      // 双击用右键模拟，打开详情
      emit('select', node.id)
    })
    .onBackgroundClick(() => {
      if (focusedNode) {
        focusedNode = null
        highlightNodes = new Set()
        highlightLinks = new Set()
        Graph.refresh()
        setTimeout(() => Graph.zoomToFit(400, 20), 100)
        emit('unfocus')
      }
    })
    .width(rect.width)
    .height(rect.height)

  // 灯光
  const scene = Graph.scene()
  scene.add(new THREE.AmbientLight('#8888aa', 0.7))
  const light = new THREE.PointLight('#ffffff', 0.8, 500)
  light.position.set(0, 50, 100)
  scene.add(light)

  Graph.cameraPosition({ x: 0, y: 0, z: 200 })
  setTimeout(() => Graph.zoomToFit(400, 20), 1200)
}

function locateNode(slug: string) {
  if (!graph3d) return
  const node = graph3d.graphData().nodes.find((n: any) => n.id === slug) as any
  if (!node || node.x === undefined || node.y === undefined || node.z === undefined) return

  const distRatio = 1 + 80 / Math.hypot(node.x as number, node.y as number, node.z as number)
  graph3d.cameraPosition(
    { x: node.x * distRatio, y: node.y * distRatio, z: node.z * distRatio },
    { x: node.x, y: node.y, z: node.z },
    600
  )

  highlightNodes = new Set([slug])
  highlightLinks = new Set()
  graph3d.graphData().links.forEach((link: any, i: number) => {
    if (link.source.id === slug || link.target.id === slug) {
      highlightNodes.add(link.source.id)
      highlightNodes.add(link.target.id)
      highlightLinks.add(i)
    }
  })
  graph3d.refresh()
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
  if (graph3d) {
    graph3d._destructor()
    graph3d = null
  }
})
</script>

<template>
  <div ref="containerRef" class="graph-3d-container" />
</template>

<style scoped>
.graph-3d-container {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: #080814;
}
</style>