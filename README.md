# 知谱 (ZhiPu) - 个人知识库

[![License](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js)](https://vuejs.org)

一个基于 AI 的个人知识管理系统，从 Synology NAS Note Station 同步笔记，构建知识索引，提供语义搜索、自动生成 Wiki 页面和交互式知识图谱。

## 功能特性

- **笔记同步** - 从 Synology Note Station 全量/增量同步，30 分钟自动后台同步
- **AI 知识索引** - 实体提取、自动摘要、关系发现、分类体系生成
- **语义搜索** - 基于实体的搜索，LLM 优化结果相关性分析
- **Wiki 生成** - 自动聚合知识生成概念页面，2D/3D 知识图谱可视化
- **AI 对话** - 流式响应助手，笔记上下文感知，AI 辅助编辑
- **问题追踪** - 记录知识探索中的未解问题

## 技术栈

| 层级     | 技术                                      |
| -------- | ----------------------------------------- |
| 后端     | Go 1.26+ / Gin / SQLite (WAL)             |
| 前端     | Vue 3 / TypeScript / Vite / Naive UI      |
| AI       | Claude API / Ollama                       |
| 可视化   | 3d-force-graph / Sigma.js                 |

## 快速开始

### 方式一：直接运行

**系统要求**: Go 1.26+、Node.js 20.19+ 或 22.12+

```bash
# 克隆项目
git clone https://github.com/your-repo/zhipu.git
cd zhipu

# 构建并运行
make build-all
./build/server
```

访问 `http://localhost:8080`

### 方式二：Docker 部署

**推荐方式**，一键启动：

```bash
# 构建镜像
docker build -t zhipu:latest .

# 启动服务
docker compose up -d

# 查看日志
docker compose logs -f
```

#### Docker 配置

| 环境变量      | 默认值               | 说明                 |
| ------------- | -------------------- | -------------------- |
| `KB_PORT`     | `8080`               | 服务端口             |
| `KB_DB_PATH`  | `/data/knowledge.db` | 数据库路径           |
| `KB_NAS_HOST` | -                    | NAS 主机（可选）     |

#### 数据管理

```bash
# 备份数据
docker run --rm -v personal-kb-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/kb-backup.tar.gz /data

# 恢复数据
docker run --rm -v personal-kb-data:/data -v $(pwd):/backup alpine \
  tar xzf /backup/kb-backup.tar.gz -C /
```

#### 配合 Ollama

编辑 `docker-compose.yml` 启用 Ollama 服务：

```yaml
ollama:
  image: ollama/ollama:latest
  ports:
    - "11434:11434"
  volumes:
    - ollama-data:/root/.ollama
```

在应用设置中配置 Ollama URL: `http://ollama:11434`

## 开发

```bash
# 后端开发 (支持 air 热重载)
make dev-backend

# 前端开发
make dev-frontend

# 运行测试
make test
```

## 目录结构

```
zhipu/
├── main.go                     # 入口
├── internal/               # Go 后端
│   ├── api/                # HTTP handlers
│   ├── ai/                 # AI 提供商层
│   ├── nas/                # NAS 客户端
│   ├── ollama/             # Ollama 集成
│   ├── store/              # 数据层
│   ├── sync/               # 同步调度

├── frontend/               # Vue 前端
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## API 概览

| 模块       | 端点                              | 功能             |
| ---------- | --------------------------------- | ---------------- |
| NAS        | `POST /api/nas/connect`           | 连接 NAS         |
|            | `POST /api/nas/sync`              | 触发同步         |
| 笔记       | `GET /api/notes`                  | 列出笔记         |
|            | `GET /api/notes/:id`              | 获取笔记         |
| AI         | `POST /api/ai/search`             | 语义搜索         |
|            | `POST /api/ai/index`              | 知识索引         |
| Wiki       | `GET /api/wiki/pages`             | Wiki 页面        |
|            | `GET /api/wiki/graph`             | 知识图谱         |
| 设置       | `GET /api/settings`               | 获取设置         |

## 许可证

[GNU General Public License v3.0](LICENSE)

## 致谢

- [Claude API](https://www.anthropic.com) - Anthropic
- [Ollama](https://ollama.ai) - 本地 LLM
- [Naive UI](https://www.naiveui.com) - Vue 3 UI
- [3d-force-graph](https://github.com/vasturiano/3d-force-graph)
- [Sigma.js](https://www.sigmajs.org)
