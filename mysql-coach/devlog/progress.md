# 开发进度

> 持续更新，记录每日开发进展

---

## 2026-08-25（Day 1）

### 完成项
- ✅ 产品定位讨论（参考面试鸭，确定差异化方向）
- ✅ 知识库数据结构设计（10个实体，JSON schema）
- ✅ 数据库表设计（11张表 + pgvector + 3个触发器）
- ✅ Go后端脚手架（Gin + config + LLM接口层）
- ✅ LLM Provider接口设计（OpenAI协议兼容，多provider隔离）
- ✅ Docker Compose配置（PostgreSQL + Backend + Frontend）
- ✅ 数据库初始化验证（11张表全部创建成功）
- ✅ 知识库种子数据（MySQL场景1：考点+原理+场景+追问策略+标准答案）
- ✅ 数据库连接层（6个Repo）
- ✅ 数据模型层（10个结构体）

### 关键决策
- 训练引擎用代码控制追问流程，不用prompt自由发挥
- 原理库+RAG保证技术准确性（不让LLM编）
- 领域无关架构（所有数据带domain标签）

---

## 2026-08-26（Day 2）

### 完成项
- ✅ **训练引擎核心逻辑**（`service/coach.go`）
  - 分层追问流程控制（L1~L5）
  - 关键词匹配判断学生回答对错
  - 常见错误检测 + 针对性应对
  - 训练完成自动生成笔记
  - 自动打勾考点进度
- ✅ HTTP Handler层（`coach.go` + `knowledge.go`）
  - 7个API接口：训练2 + 知识库4 + 健康检查1
- ✅ main.go串联所有组件
- ✅ **端到端验证通过**
  - `POST /api/coach/start` → 加载场景+追问策略 → Agent出题
  - `POST /api/coach/answer` → 判断对错 → 进下一层/给提示
  - 验证了完整训练流程：出题→回答→判断→追问→笔记
- ✅ 前端项目搭建（Next.js 16 + TypeScript + Tailwind v4）
- ✅ 前端3个页面
  - 首页：领域选择卡片（渐变背景 + Framer Motion动画）
  - 领域页：考点清单（打勾进度）+ 场景列表 + 进度条
  - 训练页：对话界面（Agent追问 + 学生回答 + 层级进度 + 完成庆祝）
- ✅ 前端依赖安装（framer-motion / react-markdown / react-syntax-highlighter / typography）
- ✅ **前后端联动验证**（浏览器预览成功）
- ✅ 推送到GitHub（dashouzi仓库dev分支）

### Bug修复
- 修复 PostgreSQL TEXT[] 数组类型 Scan 错误（用 pq.Array）
- 修复种子数据外键顺序问题（standard_answers 在 scenarios 之前插入）
- 修复前端 create-next-app 嵌套 .git 导致子模块问题
- 调整关键词匹配阈值（从50%改为命中1个即通过，避免误判）

### 关键验证结果
```
学生回答 "type is all"
→ 引擎匹配 expected_keywords ["all","全表扫描","500万"]
→ 命中 "all" → 判定 is_correct: true
→ 进入 next_level: 2
→ Agent 输出第二层追问
```

---

## 当前状态：MVP 核心闭环已打通

### 已完成模块

| 模块 | 文件 | 完成度 |
|------|------|--------|
| 数据库设计 | `schema.sql` | 100% |
| 数据库连接层 | `repository/db.go + repos.go` | 100% |
| 数据模型 | `pkg/models/models.go` | 100% |
| LLM接口层 | `llm/provider.go + openai.go` | 100% |
| 训练引擎 | `service/coach.go` | 90% |
| HTTP API | `handler/coach.go + knowledge.go` | 100% |
| 前端首页 | `app/page.tsx` | 90% |
| 前端领域页 | `app/domain/[id]/page.tsx` | 90% |
| 前端训练页 | `app/train/[id]/page.tsx` | 85% |
| 种子数据 | `docker/init/02-seed.sql` | 场景1完成 |

### 待完成

| 任务 | 优先级 | 预估时间 |
|------|--------|---------|
| RAG检索层 | 高 | 1-2天 |
| 接入LLM（配API_KEY） | 高 | 0.5天 |
| 扩展到8个场景 | 高 | 2-3天 |
| 笔记页面 | 中 | 0.5天 |
| 用户OAuth登录 | 中 | 1天 |
| 订阅付费 | 低 | MVP后 |
| 知识地图可视化 | 低 | MVP后 |
| 间隔重复复习 | 低 | MVP后 |

---

## 项目文件结构

```
mysql-coach/
├── backend/
│   ├── cmd/server/main.go              # 主入口
│   ├── internal/
│   │   ├── config/config.go            # 配置加载
│   │   ├── llm/
│   │   │   ├── provider.go             # LLM接口（多provider）
│   │   │   └── openai.go               # OpenAI协议实现
│   │   ├── repository/
│   │   │   ├── db.go                   # 数据库连接
│   │   │   └── repos.go                # 6个Repo
│   │   ├── service/coach.go            # 训练引擎（核心）
│   │   └── handler/
│   │       ├── coach.go               # 训练API
│   │       └── knowledge.go            # 知识库API
│   ├── pkg/models/models.go            # 数据模型
│   └── migrations/001_init.sql         # 建表SQL
├── frontend/
│   ├── app/
│   │   ├── page.tsx                    # 首页
│   │   ├── domain/[id]/page.tsx       # 领域页
│   │   ├── train/[id]/page.tsx        # 训练页
│   │   ├── layout.tsx
│   │   └── globals.css
│   ├── lib/api.ts                      # API调用封装
│   └── package.json
├── docker/
│   ├── docker-compose.yml
│   ├── Dockerfile.backend
│   ├── Dockerfile.frontend
│   └── init/
│       ├── 01-schema.sql               # 建表
│       └── 02-seed.sql                 # 种子数据
├── devlog/                             # 开发日志
│   ├── README.md
│   ├── prd.md
│   ├── progress.md                     # 本文件
│   ├── decisions.md
│   └── issues.md
├── .env.example
├── .gitignore
└── README.md
```
