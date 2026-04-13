# 个人知识库系统实施计划

> **状态：已完成** (2026-04-13)
>
> Phase 1-8 全部完成。验收标准全部通过。

## 项目信息

- **需求来源：** 用户请求
- **实施日期：** 2026-04-09
- **技术栈：** Go 1.22+ + Vue 3 + SQLite + Ollama
- **预估工期：** 2-3 天

---

## 任务清单

### Phase 1: 项目脚手架 (1-2h)

**1.1 创建 Go 后端项目**
- 初始化 Go module: `go mod init personal-kb`
- 创建目录结构: `cmd/server`, `internal/api`, `internal/nas`, `internal/ollama`, `internal/store`, `internal/sync`
- 添加依赖: `gin`, `gorm` (或 `sqlite3` 原生), `gorilla/websocket`
- 创建 `main.go` 入口，初始化 Gin 路由框架

**1.2 创建 Vue 前端项目**
- `npm create vue@latest web`
- 安装依赖: `naive-ui`, `vue-router`, `pinia`, `axios`
- 配置 Vite 代理（开发时代理到 Go 后端）
- 创建基础布局: 侧边栏导航 + 主内容区

**1.3 配置开发环境**
- 创建 `Makefile` 或 `taskfile.yml` 方便启动
- 配置热重载（air for Go, vite HMR for Vue）

---

### Phase 2: 数据层 (2-3h)

**2.1 SQLite 数据库初始化**
- 创建 `internal/store/db.go`
- 实现数据库连接和迁移
- 表结构: `settings`, `notebooks`, `notes`, `conversations`, `messages`

**2.2 Settings 存储**
- 实现 CRUD: `GetSetting`, `SetSetting`
- 密码加密: AES-GCM 加密存储 NAS 密码

**2.3 Notes 存储**
- 实现: `SaveNote`, `GetNote`, `ListNotes`, `ListNotesByNotebook`
- 实现: `SaveNotebook`, `ListNotebooks`

**2.4 Conversations 存储**
- 实现: `CreateConversation`, `GetConversation`, `ListConversations`
- 实现: `AddMessage`, `GetMessages`

---

### Phase 3: NAS 客户端 (3-4h)

**3.1 认证模块**
- 创建 `internal/nas/auth.go`
- 实现 `Login(username, password) (sessionID string, err error)`
- 实现 `Logout()` 和 `KeepAlive()`
- session 管理：内存存储，带过期时间

**3.2 Note Station API 封装**
- 创建 `internal/nas/client.go`
- 实现 `ListNotes(offset, limit)` → 调用 `SYNO.NoteStation.Note/list`
- 实现 `GetNote(id)` → 调用 `SYNO.NoteStation.Note/get`
- 实现 `ListNotebooks()` → 调用 `SYNO.NoteStation.Notebook/list`
- 处理 API 版本差异（DSM 6/7）

**3.3 内容处理**
- HTML 内容转纯文本（用于 AI 处理）
- 处理附件引用（图片等）

---

### Phase 4: Ollama 客户端 (2h)

**4.1 基础客户端**
- 创建 `internal/ollama/client.go`
- 配置: 地址、模型名（从 settings 读取）
- 实现 HTTP 调用封装

**4.2 生成功能**
- 实现 `Generate(prompt string) (string, error)`
- 调用 `POST /api/generate`

**4.3 对话功能**
- 实现 `Chat(messages []Message) (chan string, error)`
- 调用 `POST /api/chat`，SSE 流式返回
- 支持取消请求

---

### Phase 5: 同步服务 (2h)

**5.1 初始同步**
- 创建 `internal/sync/sync.go`
- 实现 `FullSync()`：拉取所有笔记本和笔记
- 保存到 SQLite 缓存

**5.2 增量同步**
- 实现 `IncrementalSync()`：基于 `modified_time` 只拉取变更
- 后台定时任务（默认 30 分钟）

**5.3 同步状态暴露**
- 同步进度（可选）
- 最后同步时间

---

### Phase 6: 后端 API 实现 (3-4h)

**6.1 设置 API**
- `POST /api/nas/connect` - 连接 NAS，触发首次同步
- `POST /api/nas/disconnect` - 断开连接
- `GET /api/nas/status` - 返回连接状态和最后同步时间
- `POST /api/nas/sync` - 手动触发同步

**6.2 笔记 API**
- `GET /api/notebooks` - 笔记本树
- `GET /api/notes?notebook_id=&page=&page_size=` - 笔记列表
- `GET /api/notes/:id` - 笔记详情（优先从缓存读取）

**6.3 AI API**
- `POST /api/ai/summarize/:id` - 生成摘要（异步，可缓存）
- `POST /api/ai/search` - 智能搜索
  - 实现：将所有笔记标题+摘要作为上下文发给 Ollama
  - 返回相关笔记列表和理由
- `POST /api/ai/edit` - 编辑辅助
  - body: `{ note_id, selected_text?, instruction }`
  - 返回修改后的内容
- `GET /api/ai/conversations` - 对话列表
- `POST /api/ai/conversations` - 创建对话
- `POST /api/ai/conversations/:id/messages` - 发送消息（SSE 流式）
  - 如果 conversation 有关联 note_id，将笔记内容作为 system context

**6.4 设置 API**
- `GET /api/settings` - 获取设置（不含密码）
- `PUT /api/settings` - 更新设置（Ollama 地址、模型等）

---

### Phase 7: 前端页面实现 (4-5h)

**7.1 设置页面**
- NAS 连接表单：地址、端口、用户名、密码
- Ollama 配置：地址、模型选择下拉框
- 连接测试按钮
- 保存后自动触发首次同步

**7.2 笔记列表页面**
- 左侧：笔记本树形菜单
- 右侧：笔记列表（标题、更新时间）
- 顶部：搜索框（本地搜索标题）
- 点击笔记进入详情页

**7.3 笔记详情页面**
- 左侧：笔记内容展示（HTML 渲染）
- 右侧：AI 面板
  - 摘要按钮：生成/展示摘要
  - 编辑辅助：输入指令，展示修改建议，支持"应用"
- 底部："开始对话"按钮（关联当前笔记）

**7.4 AI 搜索页面**
- 搜索输入框（自然语言）
- 结果列表：相关笔记 + AI 推荐理由
- 点击可跳转到笔记详情

**7.5 对话页面**
- 左侧：对话历史列表
- 右侧：聊天界面
  - 消息气泡（用户/AI）
  - 输入框 + 发送按钮
  - 流式输出展示
  - 可选择关联笔记（作为上下文）

**7.6 全局组件**
- 导航侧边栏
- 连接状态指示器
- 同步状态指示器

---

### Phase 8: 集成与测试 (2-3h)

**8.1 端到端测试**
- 完整流程：连接 NAS → 同步 → 查看笔记 → 生成摘要 → 对话问答
- 错误处理：NAS 断开、Ollama 不可用等

**8.2 性能优化**
- 笔记列表分页
- 大笔记内容懒加载
- AI 请求超时处理

**8.3 构建配置**
- Go: `go build` 生成可执行文件
- Vue: `npm run build` 生成静态文件
- 静态文件嵌入 Go 二进制（使用 `embed`）

---

## 技术决策

### 1. 数据库选择
- **SQLite**：单文件、零配置、足够支撑 <100 篇笔记
- **GORM vs 原生**：选择原生 `database/sql` + `modernc.org/sqlite`，减少依赖

### 2. NAS API 调用
- 直接 HTTP POST 到 `/webapi/entry.cgi`
- 参数使用 `application/x-www-form-urlencoded`
- 跳过 SSL 验证（NAS 常用自签名证书）

### 3. AI 上下文策略
- **摘要**：直接发送笔记全文
- **搜索**：发送所有笔记的标题+前 200 字摘要作为上下文
- **对话**：关联笔记时发送笔记全文作为 system message

### 4. 前端状态管理
- Pinia 管理：NAS 连接状态、笔记列表、当前笔记、对话历史

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Synology API 变更 | 高 | 记录当前使用的 API 版本，预留版本检测逻辑 |
| Ollama 模型不支持中文 | 中 | 允许用户选择模型，提供模型推荐 |
| 大笔记内容超过 AI 上下文限制 | 中 | 实现内容分块，超长笔记分段处理 |
| NAS 连接不稳定 | 低 | 本地缓存优先，失败时展示缓存数据 |

---

## 验收标准

- [x] 能成功连接 NAS 并拉取笔记列表
- [x] 笔记内容能正常展示（HTML 渲染）
- [x] AI 摘要功能正常工作
- [x] AI 智能搜索返回相关笔记
- [x] AI 编辑辅助能修改内容
- [x] 对话式问答支持流式输出
- [x] 所有配置持久化保存
- [x] 构建为单一可执行文件（含前端静态资源）

---

## 后续扩展（不在本期）

- 向量语义搜索（引入 embedding 模型 + 向量数据库）
- 笔记编辑和回写 NAS
- 多 NAS 支持
- 移动端适配
