# 个人知识库 (Personal Knowledge Base)

一个基于 AI 的个人知识管理系统，从 Synology NAS Note Station 同步笔记，构建知识索引，提供语义搜索、自动生成 Wiki 页面和交互式知识图谱。

## 功能特性

### NAS 笔记同步
- 支持从 Synology Note Station 全量和增量同步
- 每 30 分钟自动后台同步
- 笔记本层级结构保留
- 内容哈希追踪变更检测

### AI 知识索引
- **实体提取**: 从笔记中提取概念、技术、工具、人物等实体
- **自动摘要**: 为每条笔记生成 2-3 句摘要
- **关系发现**: 发现笔记间的关联关系
- **分类体系**: 自动生成层级分类树

### 语义搜索
- 基于实体的搜索（查找提及特定概念的笔记）
- 分类筛选搜索
- LLM 优化搜索结果并提供相关性分析

### Wiki 页面生成
- **概念页面**: 自动聚合多条笔记的知识生成概念百科页
- **主题页面**: 手动选择笔记合成专题内容
- **知识图谱**: 2D/3D 可视化概念关系网络
- **共现关系**: 通过笔记共享关联概念

### AI 对话助手
- 流式响应的对话式 AI 助手
- 笔记上下文感知讨论
- 聊天历史持久化
- AI 辅助编辑（选中文本后指令修改）

### 待解问题追踪
- 记录知识探索中的未解问题
- 问题与合成答案关联

## 技术栈

### 后端
- **语言**: Go 1.22+
- **Web 框架**: Gin
- **数据库**: SQLite (modernc.org/sqlite, 纯 Go 实现) with WAL 模式

### 前端
- **框架**: Vue 3 + TypeScript
- **构建工具**: Vite
- **UI 库**: Naive UI
- **状态管理**: Pinia
- **可视化**: 3d-force-graph (3D), sigma + graphology (2D)

### AI 集成
- **支持提供商**: Claude API、Ollama (本地)
- **能力**: 文本生成、流式响应、Embedding

## 快速开始

### 系统要求
- Go 1.22+
- Node.js 20.19+ 或 22.12+
- Synology NAS with Note Station (可选)
- AI 提供商: Claude API 密钥 或 Ollama 服务

### 构建

```bash
# 构建后端
make build

# 构建前端
make frontend

# 构建全部 (前端嵌入后端)
make build-all

# 运行测试
make test

# 清理
make clean
```

### 开发模式

```bash
# 后端开发 (支持 air 热重载)
make dev-backend

# 前端开发
make dev-frontend
```

### 生产运行

服务器是单个二进制文件，前端已嵌入:

```bash
make build-all
./bin/server
```

默认端口: 8080

## 配置

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `KB_PORT` | `8080` | 服务监听端口 |
| `KB_DB_PATH` | `knowledge.db` | SQLite 数据库路径 |
| `KB_NAS_HOST` | - | NAS 主机名 (备用) |

### 设置 (通过 UI 或 API)

- **NAS 连接**: 主机、端口、用户名、密码
- **AI 提供商**: 类型 (claude/ollama)、URL、模型、API 密钥
- **同步间隔**: 默认 30 分钟

## 目录结构

```
个人知识库/
├── cmd/server/main.go      # 入口点
├── internal/
│   ├── api/                # HTTP handlers 和路由
│   ├── ai/                 # AI 提供商抽象层
│   ├── nas/                # Synology NAS 客户端
│   ├── ollama/             # Ollama 集成
│   ├── store/              # 数据层 (SQLite)
│   ├── sync/               # 同步逻辑和调度器
│   └ util/                 # 工具函数
├── web/                    # Vue 前端
│   ├── src/views/          # 页面组件
│   ├── src/stores/         # Pinia 状态
│   ├── src/api/            # API 客户端
│   ├── dist/               # 构建产物 (嵌入)
├── data/                   # 运行时数据
├── knowledge.db            # SQLite 数据库
├── Makefile                # 构建脚本
└── go.mod / go.sum         # Go 依赖
```

## API 端点概览

### NAS & 同步
- `POST /api/nas/connect` - 连接 NAS
- `POST /api/nas/sync` - 手动触发同步
- `GET /api/nas/status` - 连接状态

### 笔记
- `GET /api/notebooks` - 列出笔记本
- `GET /api/notes` - 列出笔记 (分页)
- `GET /api/notes/:id` - 获取单条笔记

### AI 功能
- `POST /api/ai/search` - 语义搜索
- `POST /api/ai/index` - 触发知识索引
- `GET /api/ai/index/status` - 索引统计
- `GET /api/ai/notes/:id/related` - 获取关联笔记

### 对话
- `GET /api/ai/conversations` - 列出对话
- `POST /api/ai/conversations/:id/messages` - 发送消息 (SSE 流)

### Wiki & 知识图谱
- `GET /api/wiki/pages` - 列出 Wiki 页面
- `GET /api/wiki/graph` - 知识图谱数据
- `POST /api/wiki/auto` - 自动生成 Wiki

### 设置
- `GET /api/settings` - 获取设置
- `PUT /api/settings` - 更新设置
- `POST /api/ai/configs` - 创建 AI 配置

## 数据库结构

### 核心表
- `notes` - 笔记内容、元数据、内容哈希
- `notebooks` - 笔记本层级
- `settings` - 键值配置存储
- `conversations` / `messages` - 聊天历史
- `ai_configs` - AI 提供商配置 (密钥加密)

### 知识索引表
- `note_entities` - 提取的实体
- `note_summaries` - AI 摘要
- `note_relations` - 笔记关系
- `wiki_concepts` - 概念页面
- `concept_relations` - 概念关系边
- `categories` / `note_categories` - 分类体系

## 许可证

本项目采用 GNU General Public License v3.0 许可证。

```
GNU GENERAL PUBLIC LICENSE
Version 3, 29 June 2007

Copyright (C) 2026

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
```

完整的许可证文本请见 [LICENSE](LICENSE) 文件或访问 <https://www.gnu.org/licenses/gpl-3.0.html>。

## 贡献

欢迎贡献代码、报告问题或提出功能建议。

根据 GPLv3 许可证，任何对本项目的修改和分发都必须：
- 保持相同的 GPLv3 许可证
- 提供源代码访问
- 说明代码的修改部分

## 作者

Jack Zeng

## 致谢

- [Claude API](https://www.anthropic.com) - Anthropic
- [Ollama](https://ollama.ai) - 本地 LLM 运行
- [Naive UI](https://www.naiveui.com) - Vue 3 UI 组件库
- [3d-force-graph](https://github.com/vasturiano/3d-force-graph) - 3D 图可视化
- [Sigma.js](https://www.sigmajs.org) - 2D 图渲染