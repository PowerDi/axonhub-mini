<div align="center">

# AxonHub Mini — All-in-one AI 开发平台
### 任意 SDK、任意模型、零代码改动

[![Go 版本](https://img.shields.io/github/go-mod/go-version/looplj/axonhub?logo=go&logoColor=white)](https://golang.org/)
[![Docker Ready](https://img.shields.io/badge/docker-ready-2496ED?logo=docker&logoColor=white)](https://docker.com)

[中文](README.md) | [English](README.en-US.md) | [日本語](README.ja-JP.md)

</div>

---

## 🔱 关于本 Fork（深度二开）

**[PowerDi/axonhub-mini](https://github.com/PowerDi/axonhub-mini)** 基于原始项目 **[looplj/axonhub](https://github.com/looplj/axonhub)** 二次开发。原项目提供了完整、优秀的多协议 AI 网关能力，在此向原作者与社区致以诚挚感谢 🙏。

本仓库已从"跟随上游"转为 **深度二次开发（深度二开）**：在持续同步上游通用能力的同时，围绕**大规模、多渠道**的实际运维场景做了以下重点增强。

### ✨ 本 Fork 重点解决的问题

- **🎯 逐模型策略（per-model）**
  路由、负载均衡、重试、粘性等策略支持**按单个模型粒度**配置，不再只能在渠道级别统一设置——不同模型可拥有各自独立的故障转移与负载均衡行为。

- **🧩 多端点 Provider，精简配置**
  同一个 Provider 下支持配置**多个端点（endpoint）**，将原本需要在多个渠道里重复填写的鉴权、代理、模型等信息合并到一处，**显著减少配置环节**与维护成本。

- **↕️ 拖拽调整优先级**
  渠道 / 端点优先级支持**拖拽排序**，直观调整故障转移顺序与负载均衡权重，所见即所得。

- **🔗 未关联模型优化**
  对"未关联到任何渠道"的模型，在识别、批量关联、过滤无效项等展示与处理上做了优化，避免无效条目干扰配置与选路。

> 此外还有一系列增量改进，例如密钥 Profile 的渠道限制支持 **include / exclude（白名单 / 黑名单）模式** 等。上游的通用能力（多协议兼容、请求转换管线、追踪、RBAC 等）仍在持续同步。

---

## 📖 项目介绍

### All-in-one AI 开发平台

**AxonHub 是 AI 网关，让你无需改动一行代码即可切换模型供应商。**

无论你使用的是 OpenAI SDK、Anthropic SDK 还是任何 AI SDK，AxonHub 都会透明地将你的请求转换为与任何支持的模型供应商兼容的格式。无需重构，无需更换 SDK——只需更改配置即可。

**它解决了什么问题：**
- 🔒 **供应商锁定** - 从 GPT 瞬间切换到 Claude 或 Gemini
- 🔧 **集成复杂性** - 一个 API 格式对接 10+ 供应商
- 📊 **可观测性缺口** - 开箱即用的完整请求追踪
- 💸 **成本控制** - 实时用量追踪和预算管理

<div align="center">
  <img src="docs/axonhub-architecture-light.svg" alt="AxonHub Architecture" width="700"/>
</div>

---

### 核心特性 Core Features

| 特性 | 你能获得什么 |
|------|-------------|
| 🔄 [**任意 SDK → 任意模型**](docs/zh/api-reference/openai-api.md) | 用 OpenAI SDK 调用 Claude，或用 Anthropic SDK 调用 GPT。零代码改动。 |
| 🔍 [**完整请求追踪**](docs/zh/guides/tracing.md) | 线程级可观测性的完整请求时间线。更快定位问题。 |
| 🔐 [**企业级 RBAC**](docs/zh/guides/permissions.md) | 细粒度访问控制、用量配额和数据隔离。 |
| ⚡ [**智能负载均衡**](docs/zh/guides/load-balance.md) | <100ms 自动故障转移。始终路由到最健康的渠道。 |
| 💰 [**实时成本追踪**](docs/zh/guides/cost-tracking.md) | 每次请求的成本明细。输入、输出、缓存 Token——全部追踪。 |

> 在上游能力之上，本 Fork 把上述路由 / 负载均衡 / 重试策略进一步细化到**逐模型**粒度，并优化了多端点 Provider 与渠道优先级的配置体验（见上文「关于本 Fork」）。

---

## 🚀 快速开始 | Quick Start

### 使用 Docker 镜像（本 Fork · GHCR）

本 Fork 通过 GitHub Container Registry 发布镜像：

```bash
# 拉起服务（默认 SQLite；生产建议挂载数据卷 / 使用外部数据库）
docker run -d --name axonhub \
  -p 8090:8090 \
  -v axonhub-data:/app/data \
  ghcr.io/powerdi/axonhub-mini:latest

# 打开 http://localhost:8090
# 首次运行：按照初始化向导设置系统（创建管理员账号，密码至少 6 位）
```

> 也可从上游 [GitHub Releases](https://github.com/looplj/axonhub/releases) 下载二进制直接运行 `./axonhub`。

### 零代码迁移示例 | Zero-Code Migration Example

**你的现有代码无需任何改动。** 只需将 SDK 指向 AxonHub：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8090/v1",  # 指向 AxonHub
    api_key="your-axonhub-api-key"        # 使用 AxonHub API 密钥
)

# 用 OpenAI SDK 调用 Claude！
response = client.chat.completions.create(
    model="claude-3-5-sonnet",  # 或 gpt-4、gemini-pro、deepseek-chat...
    messages=[{"role": "user", "content": "Hello!"}]
)
```

切换模型只需改一行：`model="gpt-4"` → `model="claude-3-5-sonnet"`。无需改动 SDK。

---

## 🤖 支持的提供商与 API | Providers & APIs

### API 类型 | API Types

| API 类型 | 状态 | 描述 | 文档 |
|---------|--------|-------------|--------|
| **文本生成（Text Generation）** | ✅ Done | 对话交互接口 | [OpenAI](docs/zh/api-reference/openai-api.md)、[Anthropic](docs/zh/api-reference/anthropic-api.md)、[Gemini](docs/zh/api-reference/gemini-api.md) |
| **图片生成（Image Generation）** | ✅ Done | 图片生成 | [Image Generation](docs/zh/api-reference/image-generation.md) |
| **重排序（Rerank）** | ✅ Done | 结果排序 | [Rerank API](docs/zh/api-reference/rerank-api.md) |
| **嵌入（Embedding）** | ✅ Done | 向量嵌入生成 | [Embedding API](docs/zh/api-reference/embedding-api.md) |
| **实时对话（Realtime）** | 📝 Todo | 实时对话功能 | - |

### 支持的提供商 | Supported Providers

| 提供商 Provider | 状态 | 兼容 API |
| --- | --- | --- |
| **OpenAI** | ✅ | OpenAI, Anthropic, Gemini, Embedding, Image Generation |
| **Anthropic** | ✅ | OpenAI, Anthropic, Gemini |
| **智谱 AI (Zhipu)** | ✅ | OpenAI, Anthropic, Gemini |
| **月之暗面 (Moonshot)** | ✅ | OpenAI, Anthropic, Gemini |
| **DeepSeek** | ✅ | OpenAI, Anthropic, Gemini |
| **字节跳动豆包** | ✅ | OpenAI, Anthropic, Gemini, Image Generation |
| **Gemini** | ✅ | OpenAI, Anthropic, Gemini, Image Generation |
| **Jina AI** | ✅ | Jina Embedding, Jina Rerank |
| **OpenRouter** | ✅ | OpenAI, Anthropic, Gemini, Image Generation |
| **NanoGPT** | ✅ | OpenAI, Anthropic, Gemini, Image Generation |
| **AWS Bedrock** | 🔄 测试中 | OpenAI, Anthropic, Gemini |
| **Google Cloud** | 🔄 测试中 | OpenAI, Anthropic, Gemini |

---

## 🚀 部署 | Deployment

### 数据库支持 | Database Support

| 数据库 | 支持版本 | 推荐场景 | 自动迁移 |
|--------|----------|----------|----------|
| **SQLite** | 3.0+ | 开发环境、小型部署 | ✅ |
| **PostgreSQL** | 15+ | 生产环境、中大型部署 | ✅ |
| **MySQL** | 8.0+ | 生产环境、中大型部署 | ✅ |
| **TiDB / TiDB Cloud** | V8.0+ / Serverless | 分布式、大规模 | ✅ |
| **Neon DB** | - | Serverless、Free tier | ✅ |

### 配置文件 | Configuration

AxonHub 使用 YAML 配置文件，支持环境变量覆盖：

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"
  debug: false

db:
  dialect: "sqlite"          # sqlite / postgres / mysql / tidb
  dsn: "file:data/axonhub.db?_fk=1"

log:
  level: "info"
  encoding: "json"
```

对应环境变量：`AXONHUB_SERVER_PORT`、`AXONHUB_DB_DIALECT`、`AXONHUB_DB_DSN`、`AXONHUB_LOG_LEVEL` 等。详见 [config.example.yml](config.example.yml)。

### Docker Compose

```bash
umask 077
cat > .env <<'EOF'
DB_PASSWORD=replace-with-a-long-random-password
AXONHUB_IMAGE=ghcr.io/powerdi/axonhub-mini:latest
POSTGRES_IMAGE=postgres@sha256:replace-with-postgres-digest
EOF

docker compose --env-file .env up -d
docker compose ps
```

### Helm / Kubernetes

```bash
helm install axonhub ./deploy/helm
# 生产环境
helm install axonhub ./deploy/helm -f ./deploy/helm/values-production.yaml
kubectl port-forward svc/axonhub 8090:8090
```

详见 [Helm Chart 文档](deploy/helm/README.md)。

---

## 📖 使用指南 | Usage Guide

1. **初始化**：访问 `http://localhost:8090`，按向导创建管理员账号。
2. **配置渠道**：添加 AI 提供商渠道并测试连接。详见 [渠道配置指南](docs/zh/guides/channel-management.md)。
3. **模型管理**：通过模型关联把抽象模型（如 `gpt-4`、`claude-3-opus`）映射到具体渠道，支持精确匹配、正则、标签选择与基于优先级的回退。详见 [模型管理指南](docs/zh/guides/model-management.md)。
4. **创建 API Key**：每个密钥可配置多个 Profile——模型映射、**渠道限制（白名单 / 黑名单）**、模型访问控制、Profile 即时切换。详见 [API 密钥配置文件指南](docs/zh/guides/api-key-profiles.md)。
5. **AI 编程工具集成**：[OpenCode](docs/zh/guides/opencode-integration.md) · [Claude Code](docs/zh/guides/claude-code-integration.md) · [Codex](docs/zh/guides/codex-integration.md)。

完整文档索引见 [docs/zh/index.md](docs/zh/index.md)。

---

## 🛠️ 开发 | Development

开发说明、架构设计与贡献指南见 [docs/zh/development/development.md](docs/zh/development/development.md)。

---

## 🤝 致谢 | Acknowledgments

- 🌟 **[looplj/axonhub](https://github.com/looplj/axonhub)** — 本项目的上游，提供了完整的多协议 AI 网关能力，本 Fork 在其基础上深度二开。
- 🙏 [musistudio/llms](https://github.com/musistudio/llms) - LLM 转换框架，灵感来源
- 🎨 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) - 管理界面模板
- 🔧 [99designs/gqlgen](https://github.com/99designs/gqlgen) - GraphQL 代码生成
- 🌐 [gin-gonic/gin](https://github.com/gin-gonic/gin) - HTTP 框架
- 🗄️ [ent/ent](https://github.com/ent/ent) - ORM 框架

---

## 📄 许可证 | License

本项目采用多种许可证授权（Apache-2.0 和 LGPL-3.0），与上游保持一致。详见 [LICENSE](LICENSE)。

---

<div align="center">

**AxonHub Mini** — 基于 [looplj/axonhub](https://github.com/looplj/axonhub) 的深度二次开发分支

[🏠 本仓库](https://github.com/PowerDi/axonhub-mini) • [⬆️ 上游项目](https://github.com/looplj/axonhub) • [🐛 问题反馈](https://github.com/PowerDi/axonhub-mini/issues)

</div>
