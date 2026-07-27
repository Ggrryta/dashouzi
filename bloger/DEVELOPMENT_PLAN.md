# 博客系统 - 7 天开发排期计划

> 每天 4-6 小时，上午设计/编码，下午联调/测试。每个任务有明确的**产出物**和**验收标准**。

---

## Day 1：项目骨架 + 基础设施

**目标：** 项目能编译运行，Docker 环境一键启动，数据库自动建表。

### 上午（3h）：Go 项目初始化

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.1 | `go mod init bloger`，搭 Gin 空壳 | `cmd/server/main.go` | `/api/v1/ping` 返回 pong |
| 1.2 | 配置模块：Viper 读 yaml + 环境变量覆盖 | `internal/config/config.go` + `config/config.yaml` | 启动时打印正确配置 |
| 1.3 | 日志模块：Zap 结构化日志 | `pkg/logger/logger.go` | 请求进来有 JSON 格式日志 |
| 1.4 | 统一响应封装 | `pkg/response/response.go` + `pkg/errcode/errcode.go` | 所有接口返回 `{code, message, data}` |
| 1.5 | GORM 连接 PostgreSQL，自动迁移 | `internal/model/*.go` (7 个 model) | 启动后数据库自动建表 |

### 下午（3h）：Docker 环境 + 中间件骨架

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 1.6 | Dockerfile（多阶段构建） | `Dockerfile` | `docker build` 成功 |
| 1.7 | docker-compose.yml（app + PG + Redis） | `docker-compose.yml` | `docker compose up` 三服务全绿 |
| 1.8 | Recovery + Logger 中间件 | `internal/middleware/recovery.go`, `logger.go` | panic 不崩，请求自动打日志 |
| 1.9 | 路由骨架 | `internal/router/router.go` | 按模块分组的空路由注册完成 |
| 1.10 | Makefile | `Makefile` | `make run` / `make build` / `make test` |

**Day 1 结束验收：**
```
docker compose up -d
curl http://localhost:8080/api/v1/ping
# {"code":0,"message":"ok","data":"pong"}
```

**坑点提示：**
- GORM AutoMigrate 不会创建全文索引，`search_vector` 的 GENERATED ALWAYS 列需要手动在 `init.sql` 里建
- docker-compose 里 app 要 `depends_on` PG + Redis，但要加 `healthcheck`，否则 PG 没就绪 app 就连不上

---

## Day 2：用户模块（注册/登录/JWT）

**目标：** 用户注册登录全链路跑通，JWT 认证中间件生效。

### 上午（3h）：Model + Repository + Service

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.1 | User Model 定义 | `internal/model/user.go` | GORM tag 正确，包含钩子 |
| 2.2 | 密码加密（bcrypt） | model 中 BeforeCreate 钩子 | 存库的是 hash，不是明文 |
| 2.3 | User Repository 层 | `internal/repository/user.go` | Create / FindByEmail / FindByUsername / FindByID |
| 2.4 | User Service 层 | `internal/service/user.go` | Register（校验唯一性 + 加密）/ Login（校验密码） |
| 2.5 | JWT 工具包 | `pkg/jwt/jwt.go` | GenerateToken(user_id, role) / ParseToken(token) |

### 下午（3h）：Handler + 中间件 + 联调

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 2.6 | User Handler | `internal/handler/user.go` | 注册/登录/获取个人信息接口 |
| 2.7 | DTO 定义 | `internal/dto/request.go`, `response.go` | 请求校验 binding tag，响应结构体 |
| 2.8 | Auth 中间件 | `internal/middleware/auth.go` | 无 token 返回 401，过期返回 401，正常解析到 ctx |
| 2.9 | Role 中间件 | `internal/middleware/role.go` | 根据 role 拦截，无权限返回 403 |
| 2.10 | 注册路由 + Docker 联调 | 路由注册 | 注册→登录→拿 token→访问 `/users/me` 全链路 |通过

**Day 2 结束验收：**
```bash
# 注册
curl -X POST http://localhost:8080/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","email":"a@b.com","password":"123456"}'

# 登录（拿到 token）
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"email":"a@b.com","password":"123456"}'

# 用 token 访问个人信息
curl http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <token>"
```

**坑点提示：**
- JWT secret 必须从配置文件读，禁止硬编码
- email 和 username 唯一性校验要在 service 层做，repository 只管查
- 密码最短长度校验 + 邮箱格式校验放 binding tag 或 service 层

---

## Day 3：文章模块（CRUD + 状态机 + 标签）

**目标：** 文章完整增删改查、状态机流转、标签关联。

### 上午（3h）：Model + Repository + 状态机

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.1 | Article + Tag Model | `internal/model/article.go`, `tag.go` | GORM 多对多关联正确 |
| 3.2 | Article Repository | `internal/repository/article.go` | CRUD + 分页列表 + 状态筛选 + 标签筛选 |
| 3.3 | Tag Repository | `internal/repository/tag.go` | 创建/查找/列表，FindOrCreate |
| 3.4 | 状态机核心逻辑 | `internal/service/article.go` 中的状态流转函数 | 合法流转通过，非法流转抛错 |
| 3.5 | Article Service | `internal/service/article.go` | 创建（含标签关联）/ 更新 / 删除 / 状态变更 / 置顶 |

### 下午（3h）：Handler + 联调

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 3.6 | Article Handler | `internal/handler/article.go` | 6 个接口全部注册 |
| 3.7 | 文章详情自动 +1 阅读量 | Service 层 | 每次 GET 详情 view_count++ |
| 3.8 | 权限校验 | Handler 层 | 作者只能改自己的文章，admin 全权限 |
| 3.9 | full Docker 联调 | - | 创建文章→提交审核→发布→搜索可见 全链路 |

**Day 3 结束验收：**
```bash
# 作者创建文章（草稿）
curl -X POST /api/v1/articles -H "Authorization: Bearer <token>" \
  -d '{"title":"测试","content":"内容","tags":["Go","后端"]}'

# 提交审核
curl -X PATCH /api/v1/articles/1/status \
  -H "Authorization: Bearer <token>" \
  -d '{"status":"reviewing"}'

# 管理员发布
curl -X PATCH /api/v1/articles/1/status \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"status":"published"}'

# 读者可看到
curl /api/v1/articles  # 列表中包含该文章
```

**坑点提示：**
- 状态机用一个 `map[status][]allowedNextStatus` 配置表，不要写一堆 if-else
- 文章 slug 生成：title 转拼音/英文 + 随机后缀防冲突
- 更新文章时要同步更新 article_tags 关联表（先删后插）

---

## Day 4：评论 + 点赞 + 收藏 + 敏感词

**目标：** 完整的互动系统，评论树形展示，点赞收藏幂等。

### 上午（3h）：评论 + 敏感词

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.1 | Comment Model | `internal/model/comment.go` | parent_id 自关联，soft delete |
| 4.2 | Comment Repository | `internal/repository/comment.go` | 创建/软删除/按文章查顶级评论/查子回复 |
| 4.3 | Comment Service（含树形构建） | `internal/service/comment.go` | 返回两级嵌套结构 |
| 4.4 | DFA 敏感词过滤 | `pkg/sensitive/dfa.go` | 构建 Trie，O(n) 匹配，返回敏感词列表 |
| 4.5 | 评论接入敏感词 | Service 层调用 | 命中敏感词拒绝发布或替换为 `***` |

### 下午（3h）：点赞 + 收藏 + 联调

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 4.6 | Like Model + Repository + Service | 对应文件 | 幂等切换：点赞→取消，再点→再赞 |
| 4.7 | Favorite Model + Repository + Service | 对应文件 | 收藏/取消收藏切换，收藏列表分页 |
| 4.8 | Like + Favorite Handler | `internal/handler/like.go`, `favorite.go` | 接口联调通过 |
| 4.9 | Comment Handler | `internal/handler/comment.go` | 发表/删除/查看回复 |
| 4.10 | 全模块 Docker 联调 | - | 评论树形结构返回正确 |

**Day 4 结束验收：**
```bash
# 发表顶级评论
curl -X POST /api/v1/comments -H "Authorization: Bearer <token>" \
  -d '{"article_id":1,"content":"好文章"}'

# 回复评论（嵌套）
curl -X POST /api/v1/comments -H "Authorization: Bearer <token>" \
  -d '{"article_id":1,"parent_id":1,"content":"同意"}' 

# 查看评论树
curl /api/v1/articles/1/comments
# 应返回: [{id:1, content:"好文章", replies:[{id:2, content:"同意"}]}]

# 点赞（幂等）
curl -X POST /api/v1/likes -H "Authorization: Bearer <token>" \
  -d '{"target_type":"article","target_id":1}'
# 再调一次 → 取消点赞
```

**坑点提示：**
- DFA 的敏感词库需要一个初始词表，放 `pkg/sensitive/words.txt`，启动时加载
- 评论树查询：先查 `parent_id IS NULL` 的顶级评论，再批量查它们的子回复，避免 N+1
- 点赞/收藏的 UNIQUE 约束在 DB 层兜底，service 层做 upsert 逻辑

---

## Day 5：搜索 + 统计 + 限流

**目标：** 全文搜索可用，热门榜单有数据，限流生效。

### 上午（3h）：搜索 + 统计

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.1 | 全文搜索 Repository | `internal/repository/search.go` | ts_query 搜索，按 ts_rank 排序 |
| 5.2 | 搜索 Service + Handler | `internal/service/search.go`, `internal/handler/search.go` | 关键词搜索 + 分页 + 高亮摘要 |
| 5.3 | 标签列表接口 | Handler 中 | 返回标签 + 每个标签下已发布文章数 |
| 5.4 | 热门文章统计 | `internal/service/stats.go` | 按周/月 view_count 排序 Top N |
| 5.5 | 用户活跃度排行 | `internal/service/stats.go` | 按发表文章数 + 评论数综合排名 |

### 下午（3h）：限流 + 收尾

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 5.6 | 令牌桶限流中间件 | `internal/middleware/ratelimit.go` | Redis 实现，不同接口不同速率 |
| 5.7 | 限流接入路由 | `internal/router/router.go` | 登录 10/min，评论 20/min，注册 3/min |
| 5.8 | Stats Handler | `internal/handler/stats.go` | 热门/排行接口联调 |
| 5.9 | 所有中间件串联顺序确认 | router | Logger → Recovery → RateLimit → Auth → Role → Handler |
| 5.10 | 全量 Docker 联调 | - | 8 个模块接口全部通过 |

**Day 5 结束验收：**
```bash
# 全文搜索
curl "/api/v1/search?q=Go语言&page=1&size=10"

# 热门文章
curl /api/v1/stats/trending?period=week

# 限流测试（连续发 11 次登录请求）
for i in {1..11}; do
  curl -X POST /api/v1/users/login -d '{"email":"a@b.com","password":"wrong"}'
done
# 第 11 次应返回 429 Too Many Requests
```

**坑点提示：**
- PostgreSQL 中文分词需要 `zhparser` 扩展，Docker 镜像选 `pgvector/pgvector:pg15` 或自己装扩展
- Redis 限流 key 要设 TTL，否则内存泄漏
- 统计接口如果数据量大了会慢，Day 5 先不做缓存优化，后面秒杀系统会深入

---

## Day 6：单元测试

**目标：** 核心逻辑单元测试覆盖率 > 70%。

### 上午（3h）：Service 层测试

| # | 任务 | 测试文件 | 测试内容 |
|---|---|---|---|
| 6.1 | User Service 测试 | `internal/service/user_test.go` | 注册成功/邮箱重复/密码加密/登录成功/密码错误 |
| 6.2 | Article Service 测试 | `internal/service/article_test.go` | 状态机流转合法/非法/文章 CRUD |
| 6.3 | Comment Service 测试 | `internal/service/comment_test.go` | 创建评论/树形构建/软删除 |
| 6.4 | JWT 工具测试 | `pkg/jwt/jwt_test.go` | 生成/解析/过期/篡改 |

### 下午（3h）：Middleware + 工具测试

| # | 任务 | 测试文件 | 测试内容 |
|---|---|---|---|
| 6.5 | Auth 中间件测试 | `internal/middleware/auth_test.go` | 无 token/过期/正常/错误格式 |
| 6.6 | 敏感词 DFA 测试 | `pkg/sensitive/dfa_test.go` | 单敏感词/多敏感词/无敏感词/中文 |
| 6.7 | 限流测试 | `internal/middleware/ratelimit_test.go` | 未超限正常/超限拒绝/时间窗口重置 |
| 6.8 | 统一响应测试 | `pkg/response/response_test.go` | 各响应函数格式正确 |
| 6.9 | 覆盖率检查 | `go test -coverprofile=coverage.out ./...` | 整体 > 70% |

**Day 6 结束验收：**
```bash
go test ./... -v -cover
# ok  bloger/internal/service    0.5s  coverage: 78.0% of statements
# ok  bloger/pkg/jwt             0.3s  coverage: 85.0% of statements
# ...
```

**坑点提示：**
- Service 层测试用 `go-sqlmock` 或 `testcontainers-go` 拉起真实 PG + Redis
- 推荐 testcontainers：Docker 里开 PG 实例，测完自动销毁，符合"测试必须走 Docker"规则
- 敏感词测试要用真实中文敏感词样本

---

## Day 7：集成测试 + 文档 + GitHub

**目标：** Docker 环境下全链路集成测试，代码 push 到 GitHub，README 写好。

### 上午（3h）：集成测试

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 7.1 | 集成测试脚本 | `scripts/integration_test.sh` | 自动启动 docker compose → 跑全接口用例 → 输出结果 |
| 7.2 | 用户注册→登录→发文→评论 全链路 | 测试用例 | 端到端通过 |
| 7.3 | 状态机全路径覆盖 | 测试用例 | 所有合法/非法流转都验到 |
| 7.4 | 限流端到端验证 | 测试用例 | Docker 内 Redis 限流生效 |

### 下午（3h）：文档 + 发布

| # | 任务 | 产出物 | 验收标准 |
|---|---|---|---|
| 7.5 | README.md | 项目根目录 | 项目介绍/技术栈/快速启动/API 文档/架构图 |
| 7.6 | API 文档 | README 中或单独文件 | 每个接口的请求/响应示例 |
| 7.7 | 初始化 Git + 推送 GitHub | 公开仓库 | 别人能 clone 下来 `docker compose up` 跑起来 |
| 7.8 | 最终全量验证 | - | clone → docker compose up → 跑一遍所有接口 |

**Day 7 结束验收：**
```bash
# 终极验收：任何人 clone 下来一条命令跑通
git clone <your-repo>
cd bloger
docker compose up -d
# 等待服务就绪
./scripts/integration_test.sh
# 全部 PASS
```

---

## 整体时间线

```
Day1    Day2    Day3    Day4    Day5    Day6    Day7
 基础    用户    文章   互动    搜索   单元   集成
 设施    模块    模块   模块    +限流   测试   +发布
  ██      ██     ██     ██     ██     ██     ██
  └─ docker compose 从 Day1 一直跑到 Day7 ─┘
```

**每个 Day 提交一次 commit**，养成原子提交习惯。

---

## 验收检查清单

完成此项目后，你应该能回答以下面试问题：

- [ ] JWT 的签发和校验流程是怎样的？为什么用无状态 Token？
- [ ] RBAC 三个角色分别有哪些权限？中间件如何串联的？
- [ ] 文章状态机有哪些状态？为什么不允许 `archived` → `published`？
- [ ] DFA 算法的时间复杂度是多少？为什么比正则快？
- [ ] 令牌桶和漏桶的区别？你用的哪种？Redis 怎么实现的？
- [ ] PostgreSQL 全文搜索的原理？tsvector 和 tsquery 是什么？
- [ ] 评论树形结构怎么查的？怎么避免 N+1 问题？
- [ ] 点赞的幂等性怎么保证的？
- [ ] 三层架构（Handler → Service → Repository）各层的职责是什么？
- [ ] Docker Compose 中 healthcheck 怎么配置？depends_on 有什么坑？
