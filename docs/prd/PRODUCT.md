# PRD Product Scope

> Split from `docs/PRD.md`: Original Chapters 1-3.

## 1. Project Overview

### 1.1 Project Positioning

Saturn is a local-first personal data management system for personal developers and ordinary self-hosted users.

The system is used to centrally manage personal files, notes, task reminders, and manual accounting, while providing backend-hosted LLM-assisted interaction capabilities.

English positioning:

```text
A local-first personal data system for accounting, notes, files, calendar planning, platform operations, and LLM-assisted interaction.
```

Chinese positioning:

```text
一个本地优先的个人数据管理系统，用于管理记账、笔记、文件、日历计划、平台运维能力和 LLM 交互能力。
```

### 1.2 Core Value

The core value of the system is:

```text
Connect personal data usually scattered across different services,
unify accounting, notes, files, tasks, and reminders into a self-hosted service,
and connect to LLMs through stable backend interfaces,
allowing LLMs to search, summarize, generate drafts, and assist in operating this personal data under controlled permissions.
```

In other words, Saturn's main value is not to independently create a cloud drive, note-taking app, blog, ledger, or LLM application, but to provide a unified personal data backend that centralizes originally dispersed service capabilities:

```text
Accounting / Manual bookkeeping service
Notes / Notes and RSS sources
Files / File service
Calendar / Tasks, events, and reminder service
Platform / Authentication, config, search, object references, storage, audit, and operations views
LLM backend interaction interfaces
```

The system's main thread can be summarized as:

```text
Scattered Personal Services
→ Self-hosted Unified Personal Data Service
→ REST API / SSE
→ LLM-ready Backend
```

Among them:

```text
Connecting scattered personal services = Main body
Self-hosted unified data backend = Implementation method
LLM calling interfaces = Key enhancement capability
```

### 1.3 Design Principles

```text
Personal use first
Single-instance friendly
Local first
Default single Docker Host / Single machine deployment
Development defaults to Docker Compose
Separation of files and metadata
REST API first
Do not introduce WebSocket
LLM as an interfaced enhancement capability
Do not process highly sensitive personal data
```

### 1.4 Product Boundaries

System boundaries are used to control the project scale, preventing it from expanding from a personal data system into a general SaaS, enterprise collaboration platform, password manager, email client, or financial system.

#### 1.4.1 Deployment Boundaries

```text
Development defaults to deployment via root directory `docker-compose.yml`.
Later stages may consider splitting into multi-container deployment under a single Docker Host.
Defaults to personal servers, NAS, VPS, or local machines.
Does not target Kubernetes, multi-node clusters, or high availability clusters as default deployment targets.
Does not target large-scale public SaaS operations as a default goal.
```

The system can retain configuration extensibility, but product design, documentation, default configurations, and operational mindset should prioritize personal self-hosting and rapid development startup scenarios.

#### 1.4.2 User Boundaries

```text
Personal use is the first priority.
Exactly one human administrator controls the instance.
Programmatic clients use named, scoped API keys.
Registration, invitations, additional human accounts, roles, sharing, and user-level isolation are not provided.
The system does not do multi-tenancy.
The system does not provide tenant / workspace / organization level isolation models.
The system is not primarily intended for team collaboration, enterprise management, or commercial SaaS scenarios.
```

#### 1.4.3 Real-time Communication Boundaries

```text
Do not do WebSocket.
Ordinary requests use REST.
The current implementation does not require bidirectional real-time communication.
```

The system does not handle scenarios requiring bidirectional low-latency real-time communication, such as multi-person real-time collaborative editing, online chatting, multi-person whiteboards, or real-time gaming.

#### 1.4.4 Sensitive Data Boundaries

The system does not actively enter the high-sensitive data management domain.

Explicitly not acting as the following systems:

```text
Email client
Email archiving system
Password manager
General-purpose key management or secret-vault system
Investment portfolio management system
Securities / Funds / Crypto asset analysis system
Medical health record system
Legal case management system
Identity document vault
```

Defaults to not collecting, syncing, or parsing the following types of data:

```text
Email bodies
Email account credentials
Website passwords
Third-party API secrets / private keys, except Saturn's own one-time-issued access keys
Bank transaction automatic sync data
Securities / Funds / Crypto asset holding data
Medical diagnosis records
Legal case materials
Scans of government identity documents
```

Accounting only handles lightweight income and expense records manually entered by the user, receipt associations, and subscription reminders; it does not provide investment advice, asset allocation, securities analysis, tax advice, or automatic bank syncing.

LLMs should not be designed as agents capable of reading or operating on the highly sensitive data mentioned above.

---

## 2. Administrator and API Key Model

### 2.1 Usage Model

The system targets single-instance self-deployment scenarios and prioritizes personal use.

The system does not do multi-tenancy and does not provide tenant / workspace / organization level isolation models.

System mindset:

```text
This is my personal system.
I am the single administrator, identified by USR-00000001.
I create named API keys for MCP, agents, CLI, and automation.
All business data belongs to the instance, not to an account or API key.
```

### 2.2 Principal Kinds

| Kind | Stable identity | Description |
| --- | --- | --- |
| Administrator | `USR-00000001` | Human Web login, account/API-key/configuration/audit management, and business operations |
| API key | `KEY-xxxxxxxx` | Programmatic access constrained by immutable creation-time scopes |
| System | `SYS-00000000` | Internal or unidentified authentication actions |

Supported API-key scopes are `data:read` and `data:write`; write implies read. API keys cannot manage the administrator, other keys, or audit-log queries.

### 2.3 Object Status

All objects assigned a `ref_code` provide a unified metadata projection via `object_refs`:

```text
title saves the current cross-module display projection by object_refs
tags are saved in the same `object_refs.tags`; tagless objects return an empty array
status values and transition rules are defined by the owning business module
Platform/ObjectRef only saves and returns the current metadata projection
status does not grant read permissions
```

The status of Notes created by the current Notes API is fixed as `draft`.

### 2.4 Audit Requirements

Important operations need to enter the audit log, including:

```text
File upload
File deletion
Batch modification
Permission changes
API key creation / revocation
LLM tool call
LLM action confirmation
```

### 2.5 Permission Execution Model

The permission model serves a single-instance personal system and does not design multi-tenancy, organizational workspaces, or complex ACL expression systems.

Execution principles:

```text
Authentication is completed in middleware
handlers only do entry control, parameter binding, and calling services
Action-scope authorization is completed before or within the service
Business services receive the current actor / Principal
services judge whether an action can be executed via an auth authorizer
Key write operations are recorded in the audit log by the service
repos do not redefine permission policies
```

Lists and searches use the shared instance scope:

```text
authenticated administrator: all
API key with data:read: all instance reads
API key with data:write: all instance reads and writes
```

The auth layer does not generate business SQL. The business repo translates a fixed all-instance scope into parameterized queries. Historical `owner_id` columns are singleton relational anchors only and do not represent data ownership.

---

## 3. Top-Level Domain

The system's top-level domains are as follows:

```text
Saturn
├── Accounting
├── Notes
├── Files
├── Calendar
├── LLM
└── Platform
    ├── Administrator Auth / API Keys
    ├── Config
    ├── Search
    ├── ObjectRef
    ├── Storage
    └── Audit
```

Among them, the system's main closed loop is formed by the following capabilities:

```text
Files
Notes
Accounting
Calendar
Platform Search
Ops UI
REST API
```

---
