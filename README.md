# 知谱 (ZhiPu) - 个人知识库

[![License](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js)](https://vuejs.org)

基于 AI 的个人知识管理系统。从 Synology NAS Note Station 同步笔记，构建知识索引，提供语义搜索、自动 Wiki 生成和交互式知识图谱。

## 功能

- **笔记同步** — Synology Note Station 全量/增量同步，30 分钟自动后台调度
- **AI 知识索引** — 实体提取、自动摘要、关系发现、分类体系生成
- **语义搜索** — 基于实体的检索，LLM 优化结果相关性
- **Wiki 生成** — 自动聚合知识生成概念页面，2D/3D 知识图谱可视化
- **AI 对话** — 流式响应，笔记上下文感知，AI 辅助编辑
- **问题追踪** — 记录知识探索中的未解问题

## 技术栈

| 层级 | 技术 |
|---|---|
| 后端 | Go 1.26 / Gin / SQLite (WAL) |
| 前端 | Vue 3 / TypeScript / Vite / Naive UI |
| AI | Claude API / Ollama |
| 可视化 | 3d-force-graph / Sigma.js |

## 快速开始

### 直接运行

**要求**: Go 1.26+、Node.js 20.19+ 或 22.12+

```bash
git clone https://github.com/your-repo/zhipu.git
cd zhipu
make build-all    # 构建前端 + 后端
./build/server
```

访问 `http://localhost:8080`

### Docker 部署（推荐）

```bash
docker build -t zhipu .
docker compose up -d
```

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `KB_PORT` | `8080` | 服务端口 |
| `KB_DB_PATH` | `/data/knowledge.db` | 数据库路径 |
| `KB_NAS_HOST` | - | NAS 主机（可选） |

**数据备份与恢复**:

```bash
# 备份
docker run --rm -v personal-kb-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/kb-backup.tar.gz /data

# 恢复
docker run --rm -v personal-kb-data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/kb-backup.tar.gz -C /
```

**配合 Ollama** — 在 `docker-compose.yml` 中启用:

```yaml
ollama:
  image: ollama/ollama:latest
  ports:
    - "11434:11434"
  volumes:
    - ollama-data:/root/.ollama
```

应用设置中配置 Ollama URL: `http://ollama:11434`

## 开发

```bash
make dev-frontend   # 前端 (Vite dev server)
make dev-backend    # 后端 (支持 air 热重载)
make test           # 运行测试
```

## 目录结构

```
zhipu/
├── main.go                          # 程序入口
├── internal/
│   ├── api/                         # HTTP handlers & 路由
│   ├── ai/                          # AI Provider 接口 (Claude / Ollama)
│   ├── nas/                         # Synology NAS 客户端
│   ├── ollama/                      # Ollama HTTP 客户端 (generate/chat/embed)
│   ├── store/                       # SQLite 数据层 (sqlx)
│   │   ├── knowledge.go             #   KnowledgeStore 定义
│   │   ├── knowledge_entities.go    #   实体
│   │   ├── knowledge_summaries.go   #   摘要
│   │   ├── knowledge_relations.go   #   关系
│   │   ├── knowledge_activity.go    #   活动日志
│   │   ├── knowledge_wiki.go        #   Wiki 页面
│   │   ├── knowledge_concepts.go    #   概念 & 概念图谱
│   │   └── knowledge_outputs.go     #   输出、问题、分类
│   └── sync/                        # 同步调度器
├── frontend/                        # Vue 3 前端
│   └── src/
│       ├── views/                   # 页面组件
│       ├── components/              # 通用组件
│       └── stores/                  # Pinia 状态管理
├── build/                           # 编译输出 (gitignore)
├── data/                            # 运行时数据 (gitignore)
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## API 概览

| 模块 | 端点 | 功能 |
|---|---|---|
| NAS | `POST /api/nas/connect` | 连接 NAS |
| | `POST /api/nas/sync` | 触发同步 |
| 笔记 | `GET /api/notes` | 列出笔记 |
| | `GET /api/notes/:id` | 获取笔记详情 |
| AI | `POST /api/ai/search` | 语义搜索 |
| | `POST /api/ai/index` | 构建知识索引 |
| Wiki | `GET /api/wiki/pages` | Wiki 页面列表 |
| | `GET /api/wiki/graph` | 知识图谱数据 |
| 设置 | `GET /api/settings` | 获取设置 |

## 许可证

[GNU General Public License v3.0](LICENSE)
