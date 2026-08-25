# MySQL Coach —— AI 驱动的技术面试训练平台

> "不是刷题，是陪练。不是背答案，是被追问。"

## 核心理念

用 AI Agent 当教练，通过**苏格拉底式追问**训练学生掌握技术能力。不是给答案，而是逼你思考。

## 技术栈

| 层 | 技术 |
|----|------|
| 前端 | Next.js + Tailwind + Framer Motion |
| 后端 | Go + Gin |
| 数据库 | PostgreSQL + pgvector |
| LLM | OpenAI 协议兼容（可切换国产模型） |
| 部署 | Docker Compose |

## 项目结构

```
mysql-coach/
├── backend/                 # Go 后端
│   ├── cmd/server/          # 入口
│   ├── internal/
│   │   ├── config/          # 配置
│   │   ├── handler/         # HTTP handler
│   │   ├── service/         # 业务逻辑
│   │   ├── repository/      # 数据访问
│   │   ├── llm/             # LLM 接口层（多 provider）
│   │   └── rag/             # RAG 检索层
│   ├── pkg/models/          # 数据模型
│   └── migrations/          # 数据库迁移
├── frontend/                # Next.js 前端
├── docker/                  # 部署配置
│   ├── docker-compose.yml
│   ├── Dockerfile.backend
│   └── Dockerfile.frontend
├── product/                 # 产品设计文档
│   ├── knowledge-base-schema.md  # 知识库数据结构
│   └── schema.sql                # 数据库建表 SQL
└── .env.example             # 环境配置模板
```

## 快速开始

### 1. 启动 PostgreSQL（带 pgvector）

```bash
cd docker
docker-compose up -d postgres
```

### 2. 配置环境变量

```bash
cp .env.example .env
# 填入 LLM_API_KEY
```

### 3. 启动后端

```bash
cd backend
go run cmd/server/main.go
```

### 4. 测试

```bash
# 健康检查
curl http://localhost:8080/health

# LLM 连通性测试
curl http://localhost:8080/api/llm-test
```

## 开发进度

- [x] 项目脚手架
- [x] 数据库设计（schema.sql，11张表+pgvector+触发器）
- [x] LLM 接口层（多 provider 支持，OpenAI协议兼容）
- [x] 数据库连接层（repository/repos.go，6个Repo）
- [x] 数据模型层（pkg/models/models.go，10个结构体）
- [x] 知识库种子数据（MySQL 场景1：考点+原理+场景+追问策略+标准答案）
- [x] **训练引擎（追问流程控制，核心！）** ✅ 端到端验证通过
- [x] HTTP API 层（7个接口：训练2+知识库4+健康检查1）
- [x] **前端 UI** ✅ 首页+领域页+训练对话页
- [ ] RAG 检索层（原理库向量检索，LLM纠错引用）
- [ ] 多场景知识库扩展

## 手动启动数据库

```bash
# 首次初始化（会建表 + 插入种子数据）
docker rm -f coach-postgres
docker volume rm docker_coach_pg_data
cd docker
docker compose up -d postgres

# 等待10秒后验证
docker exec coach-postgres psql -U coach -d mysql_coach -c "\dt"
docker exec coach-postgres psql -U coach -d mysql_coach -c "SELECT id,title FROM scenarios;"
docker exec coach-postgres psql -U coach -d mysql_coach -c "SELECT id,total_levels FROM coaching_strategies;"
```

## 手动启动后端

```bash
cd backend
# 配置环境变量
cp ../.env.example ../.env
# 编辑 .env 填入 LLM_API_KEY

go run cmd/server/main.go
# 访问 http://localhost:8080/health
```
