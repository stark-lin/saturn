<a id="readme-en"></a>

# Saturn

[![CI/CD](https://github.com/stark-lin/saturn/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/stark-lin/saturn/actions/workflows/ci-cd.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-net%2Fhttp-00ADD8.svg)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED.svg)](https://www.docker.com/)

[English](#readme-en) | [中文](#readme-zh)

Saturn is a local-first, self-hosted personal data service for notes, files, accounting, calendar planning, platform operations, and controlled LLM-assisted workflows.

It is designed as a unified backend for personal data. Important objects can be referenced, searched, audited, and safely exposed to AI-assisted workflows through controlled service-layer interfaces.

```text
Scattered Personal Services
-> Self-hosted Unified Personal Data Service
-> REST API / SSE
-> LLM-ready Backend
```

## Why Saturn

Most personal tools split data across separate applications. Notes live in one app, files in another, accounting records somewhere else, and AI workflows often operate outside the application boundary.

Saturn takes a different approach. It brings multiple personal data modules into one self-hosted backend, gives important objects stable human-readable reference codes, and keeps all meaningful reads and writes behind service-layer authorization.

The goal is not to build another cloud SaaS product. The goal is to build a local-first personal system where data ownership, auditability, explicit interfaces, and controlled AI integration are treated as core design constraints.

## Features

| Area              | Status      | Description                                                     |
| ----------------- | ----------- | --------------------------------------------------------------- |
| Auth / Session    | Implemented | Login, current user, logout, and Redis-backed sessions          |
| Object References | Implemented | Stable reference codes and shared object metadata               |
| Search            | Implemented | Cross-module metadata search through ObjectRef                  |
| Audit Logs        | Implemented | Superuser-visible platform audit logs                           |
| Notes             | Implemented | Owner-only Markdown note CRUD                                   |
| Files             | Implemented | Collections, upload, metadata, download, and delete             |
| Accounting        | Implemented | Accounts and immutable transactions                             |
| Calendar          | Implemented | Event aggregates, events, calendar views, finish/void flow      |
| LLM               | Implemented | Sessions, queued requests, worker execution, and result polling |
| SSE Transport     | Implemented | Shared event transport endpoint for future event payloads       |

## Core Concepts

### Object Ref Code

Saturn assigns stable, readable, copyable reference codes to first-class business objects:

```text
NTE-00000001
FIL-00000002
ACC-00000003
CAL-00000004
LLM-00000005
```

The `object_refs` table is the authoritative registry for reference codes and shared display metadata such as title, tags, status, module, and object type.

Reference codes are used for:

* Direct human references
* Frontend navigation
* Metadata search
* Recent object views
* Cross-module association
* LLM tool calls

Reference codes do not replace database primary keys and do not bypass business rules. Reading or modifying the real object still goes through the owning module's service layer.

### Modular Monolith

Saturn is built as a Go modular monolith.

Each business module owns its own model, service, repository, handler, and API contract. `internal/app` wires dependencies and registers routes. `internal/platform` provides shared infrastructure such as authentication, audit logging, configuration, database bootstrap, object references, search, storage, Redis integration, and HTTP helpers.

This structure keeps the project deployable as a single application while preserving clear module boundaries inside the codebase.

### Controlled LLM Workflow

Saturn's LLM module uses a backend-hosted model provider. The frontend does not call the model provider directly.

The current baseline is a read-only / draft-oriented queued request-result workflow:

```text
submit request
-> persist queued task
-> worker claims task
-> controlled context / provider call / tool path
-> persist status and result
-> client polls result
-> audit trail
```

LLM requests do not directly access PostgreSQL, Redis, or local filesystem storage. Any future tool access should go through controlled backend interfaces and service-layer authorization.

## Tech Stack

* Go `net/http`
* PostgreSQL 17
* Redis
* `database/sql` with `github.com/jackc/pgx/v5/stdlib`
* `sqlc` for generated query code
* Plain HTML, CSS, and vanilla JavaScript
* Docker and Docker Compose
* Local filesystem object storage

Saturn intentionally does not use a frontend framework, bundler, CSS framework, or Node build step.

## Project Structure

```text
cmd/server/              HTTP server entry point
internal/app/            dependency wiring, lifecycle, routes
internal/accounting/     accounts and transactions
internal/calendar/       event aggregates, events, calendar view
internal/files/          file collections, files, upload, download
internal/llm/            controlled LLM sessions and queued requests
internal/notes/          Markdown notes
internal/platform/       auth, audit, config, db, ref, search, storage, Redis, HTTP helpers
migrations/              PostgreSQL schema migrations
web/src/                 plain static frontend
docs/                    product, API, module, data, deploy, testing docs
docker/                  Docker runtime configuration
scripts/                 development helper scripts
```

## Quick Start

### Run with Docker Compose

```sh
docker compose up --build
```

If you want a one-command startup without cloning the repository first, run:

```sh
curl -fsSL https://raw.githubusercontent.com/stark-lin/saturn/main/docker-compose.yml | docker compose -f - up -d
```

Or use the development helper script:

```sh
sh scripts/dev.sh
```

Open:

```text
http://localhost:8080
```

Check service health:

```sh
curl http://localhost:8080/healthz
```

Expected response shape:

```json
{
  "status": "ok",
  "service": "saturn",
  "started_at": "..."
}
```

After the development schema is initialized, Saturn creates a default local development account:

```text
username: admin
password: admin
role: superuser
```

The default account and default password are for local development only. `docker/config.json` keeps a development JWT secret template, and if `config.json` is missing on first startup Saturn generates a new config file with a random `auth.jwt_secret`. All of them must be changed or replaced before any real deployment.

### Run the Go Server Directly

When running on the host machine, PostgreSQL and Redis must be available first.

```sh
go run ./cmd/server
```

Or with an explicit config path:

```sh
go run ./cmd/server -config config.json
```

If `config.json` is missing on first startup, Saturn generates a local config file from built-in defaults and current `SATURN_*` environment variables. The generated file includes a random `auth.jwt_secret`, so keep `config.json` persistent if you want JWT sessions to survive restarts.

## Configuration

Runtime configuration comes from a JSON file. The default path is:

```text
config.json
```

The repository includes two committed configuration files:

| File                  | Purpose                                                                                   |
| --------------------- | ----------------------------------------------------------------------------------------- |
| `config.example.json` | Host-machine development template using `localhost` PostgreSQL and `127.0.0.1:6379` Redis |
| `docker/config.json`  | Docker runtime config using Compose service names and `/app/objects` storage              |

Both committed templates keep a development JWT secret. If `config.json` is missing, Saturn bootstraps a new file with a random secret instead.

Main configuration sections:

| Section    | Purpose                                                |
| ---------- | ------------------------------------------------------ |
| `http`     | HTTP listen address and trusted proxy CIDRs            |
| `web`      | Static frontend root                                   |
| `database` | PostgreSQL URL and development schema rebuild switch   |
| `redis`    | Redis address, currently required mainly for sessions  |
| `auth`     | JWT secret and token TTL                               |
| `storage`  | Local object storage root                              |
| `llm`      | LLM provider, worker, timeout, and rate-limit settings |
| `logging`  | Log level                                              |

If an existing config file cannot be parsed, contains unknown fields, or misses required fields, Saturn fails fast. If the file is missing, Saturn bootstraps one from built-in defaults and current `SATURN_*` environment variables instead of failing.

## API Overview

The main API surface is under `/api`.

| Capability         | Main entry                                                    |
| ------------------ | ------------------------------------------------------------- |
| Health check       | `GET /healthz`                                                |
| Auth / session     | `/api/auth/login`, `/api/auth/me`, `/api/auth/logout`         |
| SSE transport      | `GET /api/events`                                             |
| ObjectRef metadata | `/api/platform/object-refs/*`, `/api/platform/recent-objects` |
| Audit logs         | `GET /api/platform/audit-logs`                                |
| Notes              | `/api/notes`                                                  |
| Files              | `/api/files`                                                  |
| Accounting         | `/api/accounting`                                             |
| Calendar           | `/api/calendar`                                               |
| LLM                | `/api/llm`                                                    |

Detailed endpoint contracts are documented in `docs/api/`.

## Development

Run all Go tests:

```sh
go test ./...
```

Regenerate `sqlc` code after changing `queries/*.sql`, `sqlc.yaml`, or related schema:

```sh
go tool sqlc generate
```

Run formatting, generated-code checks, architecture checks, and tests:

```sh
sh scripts/check.sh
```

Smoke-check a running local service:

```sh
sh scripts/smoke.sh
```

Run the architecture dependency check directly:

```sh
go run github.com/arch-go/arch-go/v2 --color no
```

## CI/CD and Container Images

GitHub Actions runs generated-code checks, formatting, `go vet`, architecture checks, Go tests, and a Docker build.

Pull requests validate the image build. Every successful push to `main` publishes a multi-platform container image with `latest` and immutable `sha-<commit>` tags.

```sh
docker pull ghcr.io/stark-lin/saturn:latest
docker pull ghcr.io/stark-lin/saturn:sha-<full-commit-sha>
```

Published images support:

```text
linux/amd64
linux/arm64
```

See `docs/DEPLOY.md` for production configuration mounts, package permissions, branch protection, and deployment boundaries.

## Documentation

| Document                     | Content                                       |
| ---------------------------- | --------------------------------------------- |
| `docs/PRD.md`                | Product scope and current baseline            |
| `docs/prd/PRODUCT.md`        | Positioning, user model, product boundaries   |
| `docs/prd/FEATURES.md`       | Feature module scope                          |
| `docs/prd/TECH_DECISIONS.md` | REST / SSE / LLM technical decisions          |
| `docs/FILES.md`              | Directory structure and dependency rules      |
| `docs/MODULES.md`            | Module split and dependency direction         |
| `docs/API.md`                | General API conventions and module API index  |
| `docs/api/`                  | Module endpoint contracts and route statuses  |
| `docs/ER.md`                 | Data model and ObjectRef concept              |
| `docs/PERMISSION.md`         | Permission model                              |
| `docs/DEPLOY.md`             | Deployment boundaries and configuration notes |
| `docs/TESTING_LOGGING.md`    | Testing and logging standards                 |
| `docs/OBJECT_REF_CODE.md`    | Object Ref Code design                        |

## Project Scope

Saturn targets personal developers and self-hosted users. The default deployment model is a single machine or a single Docker host.

Saturn is intentionally not:

* A public SaaS platform
* A multi-tenant workspace system
* An enterprise collaboration product
* A password manager
* An email client
* An investment system
* A medical, legal, or identity vault
* An autonomous AI write-agent

The current design favors explicit service boundaries, auditability, local deployment, and controlled AI integration over broad SaaS-style collaboration features.

## Roadmap Direction

Saturn's current baseline is implemented across the core modules. Future work should focus on:

* Hardening deployment defaults
* Improving frontend polish and interaction consistency
* Expanding controlled LLM tool paths
* Improving search and object relationship views
* Strengthening recovery and administration flows
* Adding more complete user-facing documentation

## Contributing

This is primarily a personal self-hosted system, but issues and pull requests are welcome for:

* Bug fixes
* Documentation improvements
* Deployment notes
* Test coverage
* Small, well-scoped feature improvements

Large architectural changes should be discussed before implementation.

## License

Saturn is licensed under the GNU Affero General Public License v3.0.

See [LICENSE](./LICENSE) for the full license text.

---

<a id="readme-zh"></a>

# Saturn 中文说明

[![CI/CD](https://github.com/stark-lin/saturn/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/stark-lin/saturn/actions/workflows/ci-cd.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-net%2Fhttp-00ADD8.svg)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED.svg)](https://www.docker.com/)

[English](#readme-en) | [中文](#readme-zh)

Saturn 是一个本地优先、面向个人自托管的统一数据服务，用于管理笔记、文件、记账、日历计划、平台操作，以及受控的 LLM 辅助工作流。

它被设计为个人数据的统一后端。重要对象可以被稳定引用、搜索、审计，并通过受控的 service 层接口安全地暴露给 AI 辅助工作流。

```text
分散的个人服务
-> 自托管统一个人数据服务
-> REST API / SSE
-> LLM-ready 后端
```

## 为什么做 Saturn

大多数个人工具会把数据拆散到不同应用里。笔记在一个应用，文件在另一个应用，账务记录在别的地方，而 AI 工作流往往又运行在应用边界之外。

Saturn 采用不同的方式。它把多个个人数据模块收束到一个自托管后端中，为重要对象提供稳定、可读的引用码，并让所有有意义的读取和写入都经过 service 层授权。

Saturn 的目标不是再做一个云端 SaaS 产品。它的目标是构建一个本地优先的个人系统，把数据所有权、可审计性、显式接口和受控 AI 集成作为核心设计约束。

## 功能

| 区域            | 状态  | 说明                                                       |
| ------------- | --- | -------------------------------------------------------- |
| 认证 / 会话       | 已实现 | 登录、当前用户、登出，以及基于 Redis 的 session                          |
| 对象引用          | 已实现 | 稳定引用码和通用对象 metadata                                      |
| 搜索            | 已实现 | 通过 ObjectRef 进行跨模块 metadata 搜索                           |
| 审计日志          | 已实现 | superuser 可查看的平台审计日志                                     |
| 笔记            | 已实现 | owner-only Markdown 笔记 CRUD                              |
| 文件            | 已实现 | collection、upload、metadata、download、delete               |
| 记账            | 已实现 | accounts 和 immutable transactions                        |
| 日历            | 已实现 | event aggregates、events、calendar views、finish/void flow  |
| LLM           | 已实现 | sessions、queued requests、worker execution、result polling |
| SSE Transport | 已实现 | 共享事件传输端点，用于后续 event payload                              |

## 核心概念

### Object Ref Code

Saturn 为一等业务对象分配稳定、可读、可复制的引用码：

```text
NTE-00000001
FIL-00000002
ACC-00000003
CAL-00000004
LLM-00000005
```

`object_refs` 表是引用码以及 title、tags、status、module、object type 等通用展示 metadata 的权威注册表。

引用码用于：

* 用户直接引用
* 前端导航
* metadata 搜索
* 最近对象视图
* 跨模块关联
* LLM 工具调用

引用码不替代数据库主键，也不绕过业务规则。读取或修改真实对象仍然必须经过所属模块的 service 层。

### 模块化单体

Saturn 是一个 Go modular monolith。

每个业务模块拥有自己的 model、service、repository、handler 和 API 契约。`internal/app` 负责依赖装配和路由注册。`internal/platform` 提供共享基础能力，例如认证、审计日志、配置、数据库初始化、对象引用、搜索、存储、Redis 集成和 HTTP helper。

这种结构让项目可以作为单个应用部署，同时在代码层面保留清晰的模块边界。

### 受控 LLM 工作流

Saturn 的 LLM 模块使用后端托管的模型 provider。前端不会直接调用模型 provider。

当前基线是 read-only / draft-oriented 的 queued request-result 工作流：

```text
提交请求
-> 持久化 queued task
-> worker claim task
-> 受控上下文 / provider 调用 / 工具路径
-> 持久化状态和结果
-> client 轮询结果
-> 审计轨迹
```

LLM 请求不会直接访问 PostgreSQL、Redis 或本地文件系统存储。未来任何 tool access 都应通过受控后端接口和 service 层授权。

## 技术栈

* Go `net/http`
* PostgreSQL 17
* Redis
* `database/sql` with `github.com/jackc/pgx/v5/stdlib`
* `sqlc` 生成 query code
* Plain HTML、CSS 和 vanilla JavaScript
* Docker 和 Docker Compose
* 本地文件系统对象存储

Saturn 有意不使用 frontend framework、bundler、CSS framework 或 Node build step。

## 项目结构

```text
cmd/server/              HTTP server 入口
internal/app/            依赖装配、生命周期、路由
internal/accounting/     accounts 和 transactions
internal/calendar/       event aggregates、events、calendar view
internal/files/          file collections、files、upload、download
internal/llm/            受控 LLM sessions 和 queued requests
internal/notes/          Markdown notes
internal/platform/       auth、audit、config、db、ref、search、storage、Redis、HTTP helpers
migrations/              PostgreSQL schema migrations
web/src/                 plain static frontend
docs/                    product、API、module、data、deploy、testing docs
docker/                  Docker runtime configuration
scripts/                 development helper scripts
```

## 快速开始

### 使用 Docker Compose 运行

```sh
docker compose up --build
```

如果你想在不先克隆仓库的情况下直接一键启动，可以运行：

```sh
curl -fsSL https://raw.githubusercontent.com/stark-lin/saturn/main/docker-compose.yml | docker compose -f - up -d
```

也可以使用开发辅助脚本：

```sh
sh scripts/dev.sh
```

打开：

```text
http://localhost:8080
```

检查服务健康状态：

```sh
curl http://localhost:8080/healthz
```

预期响应形状：

```json
{
  "status": "ok",
  "service": "saturn",
  "started_at": "..."
}
```

开发 schema 初始化后，Saturn 会创建一个本地开发默认账号：

```text
username: admin
password: admin
role: superuser
```

默认账号和默认密码只适合本地开发。`docker/config.json` 保留的是开发用 JWT secret 模板；如果首次启动时缺少 `config.json`，Saturn 会生成一个包含随机 `auth.jwt_secret` 的新配置文件。真实部署前，这些内容都必须替换。

### 直接运行 Go Server

在宿主机直接运行时，需要先准备 PostgreSQL 和 Redis。

```sh
go run ./cmd/server
```

或显式指定配置路径：

```sh
go run ./cmd/server -config config.json
```

如果首次启动时缺少 `config.json`，Saturn 会根据内置默认值和当前 `SATURN_*` 环境变量生成本地配置文件。生成出来的文件包含随机 `auth.jwt_secret`，所以如果你希望 JWT 会话在重启后仍然有效，就要把 `config.json` 持久化保存。

## 配置

运行时配置来自 JSON 文件。默认路径是：

```text
config.json
```

仓库包含两个已提交的配置文件：

| 文件                    | 用途                                                         |
| --------------------- | ---------------------------------------------------------- |
| `config.example.json` | 宿主机开发模板，使用 `localhost` PostgreSQL 和 `127.0.0.1:6379` Redis |
| `docker/config.json`  | Docker 运行配置，使用 Compose 服务名，并使用 `/app/objects` 作为存储路径       |

这两个已提交模板都保留了开发用 JWT secret。如果 `config.json` 缺失，Saturn 会改为生成一个带随机 secret 的新文件。

主要配置区域：

| 区域         | 用途                                          |
| ---------- | ------------------------------------------- |
| `http`     | HTTP 监听地址和可信代理 CIDR                         |
| `web`      | 静态前端根目录                                     |
| `database` | PostgreSQL URL 和开发 schema 重建开关              |
| `redis`    | Redis 地址，当前主要用于 session                     |
| `auth`     | JWT secret 和 token TTL                      |
| `storage`  | 本地对象存储根目录                                   |
| `llm`      | LLM provider、worker、timeout 和 rate-limit 设置 |
| `logging`  | 日志等级                                        |

如果已有配置文件无法解析、包含未知字段或缺少必需字段，Saturn 会 fail fast。若配置文件不存在，它会先根据内置默认值和当前 `SATURN_*` 环境变量生成一个。

## API 概览

主要 API 位于 `/api` 下。

| 能力                 | 主要入口                                                          |
| ------------------ | ------------------------------------------------------------- |
| Health check       | `GET /healthz`                                                |
| 认证 / 会话            | `/api/auth/login`, `/api/auth/me`, `/api/auth/logout`         |
| SSE transport      | `GET /api/events`                                             |
| ObjectRef metadata | `/api/platform/object-refs/*`, `/api/platform/recent-objects` |
| 审计日志               | `GET /api/platform/audit-logs`                                |
| 笔记                 | `/api/notes`                                                  |
| 文件                 | `/api/files`                                                  |
| 记账                 | `/api/accounting`                                             |
| 日历                 | `/api/calendar`                                               |
| LLM                | `/api/llm`                                                    |

详细 endpoint 契约记录在 `docs/api/` 中。

## 开发

运行全部 Go 测试：

```sh
go test ./...
```

修改 `queries/*.sql`、`sqlc.yaml` 或相关 schema 后，重新生成 `sqlc` 代码：

```sh
go tool sqlc generate
```

运行格式化、generated-code check、架构检查和测试：

```sh
sh scripts/check.sh
```

对已启动的本地服务做 smoke check：

```sh
sh scripts/smoke.sh
```

直接运行架构依赖检查：

```sh
go run github.com/arch-go/arch-go/v2 --color no
```

## CI/CD 和容器镜像

GitHub Actions 会运行 generated-code check、formatting、`go vet`、architecture check、Go tests 和 Docker build。

Pull request 会验证镜像构建。每次成功 push 到 `main` 后，会发布 multi-platform container image，包含 `latest` 和不可变的 `sha-<commit>` tags。

```sh
docker pull ghcr.io/stark-lin/saturn:latest
docker pull ghcr.io/stark-lin/saturn:sha-<full-commit-sha>
```

发布镜像支持：

```text
linux/amd64
linux/arm64
```

生产配置挂载、package 权限、branch protection 和部署边界见 `docs/DEPLOY.md`。

## 文档

| 文档                           | 内容                       |
| ---------------------------- | ------------------------ |
| `docs/PRD.md`                | 产品范围和当前 baseline         |
| `docs/prd/PRODUCT.md`        | 项目定位、用户模型、产品边界           |
| `docs/prd/FEATURES.md`       | 功能模块范围                   |
| `docs/prd/TECH_DECISIONS.md` | REST / SSE / LLM 技术决策    |
| `docs/FILES.md`              | 目录结构和依赖规则                |
| `docs/MODULES.md`            | 模块拆分和依赖方向                |
| `docs/API.md`                | API 通用约定和模块 API 索引       |
| `docs/api/`                  | 模块 endpoint 契约和 route 状态 |
| `docs/ER.md`                 | 数据模型和 ObjectRef 概念       |
| `docs/PERMISSION.md`         | 权限模型                     |
| `docs/DEPLOY.md`             | 部署边界和配置说明                |
| `docs/TESTING_LOGGING.md`    | 测试和日志标准                  |
| `docs/OBJECT_REF_CODE.md`    | Object Ref Code 设计       |

## 项目范围

Saturn 面向个人开发者和自托管用户。默认部署模型是单机或单个 Docker host。

Saturn 有意不做：

* 公共 SaaS 平台
* 多租户 workspace 系统
* 企业协作产品
* 密码管理器
* 邮件客户端
* 投资系统
* 医疗、法律或身份文档库
* 自主执行写操作的 AI agent

当前设计相比广泛的 SaaS 协作功能，更重视显式 service 边界、可审计性、本地部署和受控 AI 集成。

## Roadmap 方向

Saturn 当前 baseline 已经覆盖核心模块。后续工作重点包括：

* 强化部署默认配置
* 改进前端 polish 和交互一致性
* 扩展受控 LLM tool path
* 改进搜索和对象关系视图
* 强化 recovery 和 administration flows
* 补充更完整的用户文档

## 贡献

Saturn 主要是一个个人自托管系统，但欢迎以下类型的 issue 和 pull request：

* Bug fix
* 文档改进
* 部署说明
* 测试覆盖
* 小范围、边界清晰的功能改进

大型架构调整应在实现前先讨论。

## 许可证

Saturn 使用 GNU Affero General Public License v3.0 授权。

完整许可证文本见 [LICENSE](./LICENSE)。
