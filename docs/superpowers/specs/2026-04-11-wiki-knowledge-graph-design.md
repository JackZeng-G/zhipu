# Wiki 概念知识图谱设计

> 基于 Karpathy LLM Wiki 核心原则，将现有 wiki 从"每实体一页"重构为概念知识图谱。

## Context

当前 wiki 实现为每篇笔记的每个实体生成一个独立 wiki 页面（一篇笔记 6 个实体 = 6 个重复页面）。需要重构为：以概念为中心，跨所有笔记聚合知识，概念之间有关联关系，最终呈现为可交互的知识图谱。

## 核心决策

1. **概念关联方式**：共现关系（自动）+ LLM 语义标签（补充），两者结合
2. **概念页生成策略**：索引时只记录元数据，用户查看时按需 LLM 生成（混合策略）
3. **矛盾检测**：LLM 生成概念页时检查来源分歧，显式标注在 contradictions 字段

## 三层架构

```
Raw 层（notes 表）          → NAS 同步的原始笔记，LLM 只读
Wiki 层（概念图谱）         → wiki_concepts + concept_relations，LLM 拥有
Output 层（持久化）         → activity_log + chat insights，不消失
```

## 数据模型

### 已有表（不变）

- `note_entities` — 索引时写入，每条笔记提取的概念/实体
- `note_summaries` — 索引时写入，笔记摘要
- `note_relations` — 笔记间关联
- `activity_log` — 操作日志
- `wiki_pages` — 旧表，保留兼容，不再新增

### 新增表

**wiki_concepts** — 概念聚合页，核心单元
```sql
slug, title, aliases(JSON), definition, key_points, content(聚合markdown),
note_ids(JSON), source_count, confidence(low/medium/high),
evolution_log(JSON), contradictions, created_at, updated_at
```

**concept_relations** — 概念间关联（知识图谱的边）
```sql
source_concept_slug, target_concept_slug, relation_type,
reason, co_occurrence_count, created_at
```

**questions** — 开放问题追踪
```sql
content, status(open/answered), opened_at, answered_at, answer_synthesis_slug
```

## 核心流程

### INGEST（索引笔记）

```
indexNote(noteID):
  1. LLM 提取概念 → note_entities（已有）
  2. LLM 生成摘要 → note_summaries（已有）
  3. 概念去重：查 wiki_concepts.aliases 匹配已有概念
  4. 更新每个概念的 source_count（纯计数，不调 LLM）
  5. 构建共现关系 → concept_relations（同一笔记中的概念互相关联）
  6. 记录 activity_log
```

### VIEW（查看概念页）

```
用户点击概念:
  1. 查 wiki_concepts 表
  2. 若无内容 → 聚合所有提到此概念的笔记 → LLM 生成概念页
     - Prompt 要求 LLM 输出：定义、要点、矛盾检测、来源列表
     - 保存到 wiki_concepts
  3. 若有内容但 source_count 增加 → 标记"可更新"
  4. Evolution Log 追加本次生成记录
```

### BROWSE（浏览图谱）

```
前端力导向图:
  - 节点 = 概念，大小 = source_count，颜色 = confidence
  - 边 = concept_relations，粗细 = co_occurrence_count
  - 点击节点 → 查看概念详情
```

## llmwiki.md 合规性

| 原则 | 实现 | 状态 |
|---|---|---|
| Raw 层不可变 | notes 表 NAS 同步，LLM 只读 | OK |
| Wiki 层 LLM 拥有 | wiki_concepts LLM 生成 | OK |
| 输出持久化 | activity_log + chat_insight 沉淀 | OK |
| 矛盾显式标注 | contradictions 字段 + LLM 检测 | OK |
| 操作记日志 | activity_log | OK |
| Evolution Log | evolution_log JSON 数组 | OK |
| Confidence 体系 | source_count → low/medium/high | OK |
| 概念名称对齐 | aliases 字段 + 去重逻辑 | OK |

## 前端设计

### WikiView 三标签页

1. **图谱** — 力导向图（D3.js 或 vis-network），节点可点击
2. **概念列表** — 按 source_count 排序，显示 confidence 徽章
3. **时间线** — 操作日志，可点击跳转

### 笔记详情增强

- 实体标签可点击 → 跳转到对应概念页
- 显示"此笔记贡献了 X 个概念"

## 实施步骤

1. 数据库：新增 wiki_concepts、concept_relations、questions 表
2. Store 层：新增 WikiConcept、ConceptRelation 结构体和 CRUD
3. API 层：重构 INGEST 流程（概念聚合 + 共现关系）
4. API 层：新增概念页查看/按需生成接口
5. 前端：WikiView 重构为图谱 + 概念列表 + 时间线
6. 前端：NotesView 实体标签可点击
7. 清理旧的重复 wiki_pages 数据

## 关键文件

| 文件 | 操作 |
|---|---|
| `internal/store/db.go` | 新增表 |
| `internal/store/knowledge.go` | 新增结构体和 CRUD |
| `internal/api/knowledge.go` | 重构 INGEST 流程 |
| `internal/api/wiki.go` | 更新 API 端点 |
| `internal/api/router.go` | 注册新路由 |
| `web/src/views/WikiView.vue` | 重构前端 |
| `web/src/api/index.ts` | 新增 API 调用 |
| `web/src/stores/knowledge.ts` | 新增状态管理 |
