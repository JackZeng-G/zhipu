# 个人知识库系统设计文档

> **状态：已完成** (2026-04-13)
>
> 本设计已于 2026-04-11 全部实现，项目已可正常运行。详见实施计划验收标准。

## 概述

一个基于 Go + Vue 的个人知识库系统，直接连接 Synology NAS 的 Note Station，集成 Ollama 本地 AI，提供文档分析、总结、智能搜索和对话式问答功能。

## 技术栈

- **后端：** Go 1.22+，Gin 框架
- **前端：** Vue 3 + Vite + TypeScript + Naive UI
- **AI：** Ollama（本地部署）
- **数据存储：** SQLite（本地缓存笔记元数据、AI 对话历史）
- **外部 API：** Synology Note Station API（逆向接口）

## 架构

```
┌─────────────┐     HTTP      ┌──────────────────┐     HTTP      ┌──────────────┐
│  Vue 3 SPA  │ ◄──────────► │   Go Backend     │ ◄──────────► │ Synology NAS │
│  (前端)      │              │   (Gin)          │              │ Note Station │
└─────────────┘              │                  │              └──────────────┘
                             │  ┌────────────┐  │
                             │  │  SQLite DB  │  │     HTTP     ┌──────────────┐
                             │  │  (本地缓存)  │  │ ◄──────────► │   Ollama     │
                             │  └────────────┘  │              │  (本地 AI)    │
                             └──────────────────┘              └──────────────┘
```

## 核心模块

### 1. NAS 连接模块 (`internal/nas/`)

负责与 Synology Note Station API 通信。

**认证流程：**
1. 用户在设置页面输入 NAS 地址、端口、用户名、密码
2. 后端调用 `SYNO.API.Auth` 登录获取 session
3. session 保存在后端内存中，定期续期

**主要接口：**
- `POST /webapi/entry.cgi?api=SYNO.API.Auth&method=login` — 登录
- `POST /webapi/entry.cgi?api=SYNO.NoteStation.Note&method=list` — 获取笔记列表
- `POST /webapi/entry.cgi?api=SYNO.NoteStation.Note&method=get` — 获取笔记内容
- `POST /webapi/entry.cgi?api=SYNO.NoteStation.Notebook&method=list` — 获取笔记本列表

**数据同步策略：**
- 首次连接时拉取全部笔记（元数据 + 内容）
- 之后增量同步（基于笔记的 `modified_time`）
- 本地 SQLite 缓存笔记元数据和内容（HTML），避免每次访问 NAS

### 2. Ollama 集成模块 (`internal/ollama/`)

与本地 Ollama 服务通信，提供 AI 能力。

**接口：**
- `POST /api/generate` — 文本生成（摘要、编辑辅助）
- `POST /api/chat` — 对话式问答（支持上下文）

**功能映射：**
- 文档摘要 → `generate`，将文档内容作为 prompt 输入
- 智能搜索 → `generate`，将搜索关键词 + 所有文档标题/摘要作为上下文
- 内容编辑辅助 → `generate`，将原文 + 编辑指令作为 prompt
- 对话式问答 → `chat`，将相关文档内容作为 system context

### 3. 数据存储模块 (`internal/store/`)

SQLite 本地数据库，缓存数据。

**表结构：**

```sql
-- NAS 连接配置（加密存储密码）
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 笔记本
CREATE TABLE notebooks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    parent_id TEXT,
    created_time INTEGER,
    modified_time INTEGER
);

-- 笔记缓存
CREATE TABLE notes (
    id TEXT PRIMARY KEY,
    notebook_id TEXT,
    title TEXT NOT NULL,
    content_html TEXT,
    content_text TEXT,        -- 纯文本版本，用于 AI 处理
    tags TEXT,                -- JSON 数组
    created_time INTEGER,
    modified_time INTEGER,
    synced_at INTEGER         -- 最后同步时间
);

-- AI 对话历史
CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id TEXT,             -- 关联的笔记，可为空
    title TEXT,
    created_at INTEGER
);

CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER REFERENCES conversations(id),
    role TEXT NOT NULL,       -- system/user/assistant
    content TEXT NOT NULL,
    created_at INTEGER
);
```

### 4. 后端 API 模块 (`internal/api/`)

Gin 路由，提供给前端的 RESTful API。

**API 设计：**

```
# NAS 连接
POST   /api/nas/connect          — 连接 NAS（地址、端口、用户名、密码）
POST   /api/nas/disconnect       — 断开连接
GET    /api/nas/status           — 连接状态
POST   /api/nas/sync             — 手动触发同步

# 笔记
GET    /api/notes                — 笔记列表（支持分页、按笔记本筛选）
GET    /api/notes/:id            — 笔记详情
GET    /api/notebooks            — 笔记本树形结构

# AI 功能
POST   /api/ai/summarize/:id     — 生成笔记摘要
POST   /api/ai/search            — 智能搜索（body: { query })
POST   /api/ai/edit              — AI 编辑辅助（body: { note_id, instruction })
GET    /api/ai/conversations     — 对话列表
POST   /api/ai/conversations     — 创建对话（body: { note_id? })
POST   /api/ai/conversations/:id/messages — 发送消息（SSE 流式返回）

# 设置
GET    /api/settings             — 获取设置
PUT    /api/settings             — 更新设置（Ollama 地址、模型名等）
```

### 5. 前端模块 (`web/`)

Vue 3 SPA，主要页面：

**页面结构：**
1. **设置页** — NAS 连接配置 + Ollama 配置
2. **笔记列表页** — 左侧笔记本树 + 右侧笔记列表，支持普通搜索
3. **笔记详情页** — 笔记内容展示 + 摘要面板 + 编辑辅助面板
4. **AI 搜索页** — 智能搜索界面
5. **对话页** — AI 对话式问答，可关联笔记

**关键交互：**
- 笔记摘要：点击按钮，AI 异步生成，展示在侧边栏
- 智能搜索：输入自然语言，AI 返回相关笔记及理由
- 编辑辅助：选中笔记内容 → 输入修改指令（如"翻译成英文""优化措辞"）→ AI 返回修改后内容 → 用户确认后更新
- 对话问答：支持流式输出（SSE），可引用笔记内容作为上下文

## 配置项

通过 `settings` 表或配置文件管理：

- NAS 地址、端口、用户名、密码（加密）
- Ollama 地址（默认 `http://localhost:11434`）
- Ollama 模型名（用户自行选择）
- 同步间隔（默认 30 分钟）
- HTTP 服务端口（默认 8080）

## 项目目录结构

```
personal-kb/
├── cmd/
│   └── server/
│       └── main.go              # 入口
├── internal/
│   ├── api/                     # HTTP handler
│   │   ├── router.go
│   │   ├── nas.go
│   │   ├── notes.go
│   │   ├── ai.go
│   │   └── settings.go
│   ├── nas/                     # Synology NAS 客户端
│   │   ├── client.go
│   │   ├── auth.go
│   │   └── notes.go
│   ├── ollama/                  # Ollama 客户端
│   │   ├── client.go
│   │   └── chat.go
│   ├── store/                   # SQLite 数据层
│   │   ├── db.go
│   │   ├── notes.go
│   │   ├── conversations.go
│   │   └── settings.go
│   └── sync/                    # 同步服务
│       └── sync.go
├── web/                         # Vue 3 前端
│   ├── src/
│   │   ├── views/
│   │   ├── components/
│   │   ├── api/
│   │   ├── stores/
│   │   └── App.vue
│   ├── package.json
│   └── vite.config.ts
├── go.mod
└── go.sum
```

## 安全考虑

- NAS 密码使用 AES 加密存储在 SQLite 中
- Ollama 仅支持本地访问，不暴露到外网
- 后端与 NAS 通信支持跳过 SSL 验证（NAS 常用自签名证书）
- API 支持 CORS 配置

## 非目标（不在本期范围）

- 向量语义搜索（未来可扩展）
- 多用户支持
- 移动端适配
- 笔记创建/编辑回写 NAS（只读）
