# 博客系统 - 技术点审查清单

> 逐条过，能讲清楚原理 + 手写核心代码 = ✅，讲不清楚或写不出来 = 复习重点。

---

## 一、项目工程化

- [ ] 三层架构：Handler / Service / Repository 各层职责是什么，为什么不能反过来调
- [ ] 依赖注入：Service 依赖 Repository 接口，如何切换真实实现和 mock
- [ ] Go module + vendor：为什么用 vendor，什么场景需要
- [ ] 统一错误码：`{code, message}` 设计，为什么不用 HTTP 状态码直接代表业务错误
- [ ] 统一响应格式：`{code, message, data}`，为什么 code=0 代表成功
- [ ] 配置管理：Viper 读 yaml + 环境变量覆盖，敏感信息（JWT secret）为什么不硬编码
- [ ] 结构化日志：Zap 输出 JSON，包含 method / path / status / latency / client_ip
- [ ] Makefile 管理常用命令：make run / build / test / up / down

---

## 二、Docker & 部署

- [ ] Dockerfile 多阶段构建：builder 阶段编译 → 运行阶段只有 15MB 二进制
- [ ] CGO_ENABLED=0 + GOOS=linux：为什么交叉编译必须禁用 CGO
- [ ] docker-compose.yml：三服务编排（app + PG + Redis）
- [ ] depends_on + healthcheck：为什么 depends_on 不够，必须加 healthcheck
- [ ] pg_isready / redis-cli ping 作为健康检查命令
- [ ] 为什么配置里数据库地址写 `postgres` 而不是 `localhost`
- [ ] Alpine 镜像为什么只有 ~5MB，缺点是什么（musl vs glibc）

---

## 三、数据库设计

- [ ] 7 张表：users / articles / tags / article_tags / comments / likes / favorites
- [ ] 多对多关系：article_tags 中间表，GORM `many2many` tag
- [ ] 外键关联：Author 预加载用 `Preload("Author")`，什么是 N+1 问题
- [ ] 自关联：comments.parent_id → comments.id，树形结构
- [ ] 软删除：is_deleted 字段，为什么不用物理删除
- [ ] UNIQUE 约束：likes(user_id, target_type, target_id)，防止重复点赞
- [ ] 索引：为什么 username/email/slug 要建唯一索引
- [ ] GORM AutoMigrate：自动建表 vs 手动 SQL，各自的适用场景
- [ ] GORM 日志级别：Silent / Error / Warn / Info，生产用哪个

---

## 四、认证与授权

- [ ] JWT 三段式结构：Header.Payload.Signature 各自包含什么
- [ ] HS256 签名：为什么比 RS256 简单，适用场景区别
- [ ] Claims 设计：user_id / username / role，为什么不放敏感信息
- [ ] Token 生成：`jwt.NewWithClaims()` + `SignedString()`
- [ ] Token 解析：`jwt.ParseWithClaims()` + keyFunc 校验
- [ ] 过期处理：`jwt.ErrTokenExpired` vs `jwt.ErrSignatureInvalid`
- [ ] JWT vs Session：无状态优劣，如何主动踢人（黑名单）
- [ ] RBAC 角色层级：reader(1) < author(2) < admin(3)
- [ ] hasAccess 实现：`userLevel >= requiredLevel` 的简洁性
- [ ] 中间件责任链：Auth 解析 token → 写入 ctx → Role 校验层级
- [ ] Gin ctx.Set / ctx.Get：跨中间件传数据的机制

---

## 五、认证中间件实现细节

- [ ] Authorization header 格式：`Bearer <token>`，为什么前缀叫 Bearer
- [ ] 三种错误分支：无 header → 401 / 格式不对 → 401 / token 过期 → 401
- [ ] 错误的 HTTP 状态码选择：401 vs 403 的区别
- [ ] c.Abort() 的作用：阻止后续 handler 执行
- [ ] c.GetUint() / c.Get() 的区别

---

## 六、状态机

- [ ] 状态机为什么用 map 配置表而不是 if-else
- [ ] 合法流转：draft→reviewing→published→archived
- [ ] 驳回回退：reviewing→draft
- [ ] 非法流转的防御：published→draft 被拒绝
- [ ] 状态变更和所有权的双重校验
- [ ] 发布时记录 published_at 时间戳
- [ ] 对外只展示 published 状态的文章

---

## 七、DFA 敏感词过滤

- [ ] Trie 树（前缀树）的数据结构：`map[rune]*trieNode` + `isEnd` 标记
- [ ] AddWords：如何构建 Trie 树，每个字符是一个节点
- [ ] Match 算法：两层循环，外层遍历文本，内层在 Trie 上转移
- [ ] 时间复杂度：O(n)，n = 文本长度，与词库大小无关
- [ ] 最长匹配原则：`abc` 不会重复命中 `ab`
- [ ] 为什么不用正则：ReDoS 攻击风险 + 性能差
- [ ] `[]rune(text)` 的作用：支持中文等多字节字符
- [ ] Filter 接口设计：`type Filter interface { Match(string) []string }`

---

## 八、限流

- [ ] 固定窗口算法原理：计数器 + 窗口起始时间
- [ ] 窗口过期判断：`now.Sub(entry.window) > window`
- [ ] Key 设计：IP + Path 组合，为什么不是只有 IP
- [ ] 登录限流 30次/分钟 的业务考量
- [ ] 超限返回 429 Too Many Requests
- [ ] sync.Mutex 保护并发安全
- [ ] 固定窗口 vs 滑动窗口 vs 令牌桶，各自的优劣

---

## 九、评论树

- [ ] parent_id 自关联实现无限层级嵌套
- [ ] 两级返回策略：顶级评论 + 子回复，更深层懒加载
- [ ] GORM Preload 子回复：`Preload("Replies").Preload("Replies.User")`
- [ ] 软删除的评论树展现：`[该评论已删除]` 但保留回复关系
- [ ] 删除权限：只有作者本人能删自己的评论

---

## 十、点赞与收藏的幂等

- [ ] Toggle 模式：POST 同一接口，第一次创建、第二次删除
- [ ] check-then-act：先查是否存在，再决定创建还是删除
- [ ] DB 层的兜底：UNIQUE(user_id, target_type, target_id)
- [ ] GORM OnConflict：`clause.OnConflict{DoNothing: true}`
- [ ] 为什么这个设计是幂等的：同一请求多次执行结果一致
- [ ] 点赞和收藏的区别：收藏只针对 article，点赞是(article|comment)多态

---

## 十一、搜索

- [ ] ILIKE 模糊搜索：`WHERE title ILIKE '%keyword%' OR content ILIKE '%keyword%'`
- [ ] ILIKE vs LIKE：大小写不敏感
- [ ] 为什么 Level 1 用 ILIKE 而不是 PostgreSQL 全文索引
- [ ] 搜索空关键词的防御
- [ ] 超长关键词截断（>200 字符）
- [ ] 默认分页参数：page=1, size=10

---

## 十二、统计

- [ ] 热门文章：`ORDER BY view_count DESC LIMIT N`
- [ ] 用户活跃排行：LEFT JOIN articles + LEFT JOIN comments + GROUP BY
- [ ] SQL 聚合函数：COUNT(DISTINCT) 的正确用法
- [ ] 为什么只统计 published 文章和未删除评论

---

## 十三、测试体系

- [ ] 测试金字塔：80% 单元 / 15% 集成 / 5% E2E
- [ ] 单元测试：用 mock 替换 DB，测试纯逻辑
- [ ] 接口注入：`NewUserService(repo UserRepository)`
- [ ] Table-driven tests：`tests := []struct{name, input, want}`
- [ ] Arrange-Act-Assert 三段式
- [ ] DAMP > DRY：测试代码允许重复
- [ ] 测结果不测实现：assert.Equal(want, got)，不 mock 内部调用
- [ ] 集成测试在 Docker 内运行：`CGO_ENABLED=0 GOOS=linux go test -c`
- [ ] E2E 测试脚本：bash + curl 全链路验证
- [ ] 测试覆盖率统计：`go test -coverprofile` + `go tool cover`
- [ ] `[no test files]` 的包（dto/model）为什么不写测试

---

## 十四、测试用例关键场景

- [ ] 注册：成功 / 邮箱重复 / 用户名为空 / 密码太短
- [ ] 登录：成功 / 密码错误 / 用户不存在
- [ ] JWT：正常解析 / 过期 / 篡改 / 空 token / 错误密钥
- [ ] 中间件：无 token / 无效 token / 错误格式 / 正常解析
- [ ] 角色：reader 越权访问 author 接口 → 403
- [ ] 状态机：所有合法流转 / 所有非法流转 / 未知状态
- [ ] 文章：创建 / 更新(含标签同步) / 删除 / 非作者操作拒绝
- [ ] 评论：发表 / 敏感词拒绝 / 树形返回 / 删除 / 非作者删除拒绝
- [ ] 点赞：创建→取消→再创建，三次调用验证幂等
- [ ] 限流：限内放行 / 超限 429 / 不同接口独立计数

---

## 十五、Go 语言特性

- [ ] interface 隐式实现：不需要 `implements` 关键字
- [ ] `type XxxRepository interface` 定义契约
- [ ] context.Context 传递：为什么每个函数第一个参数是 ctx
- [ ] defer 的堆栈顺序：`defer l.mu.Unlock()` 与 recover 的关系
- [ ] `:=` vs `=`：变量遮盖（shadow）的坑
- [ ] `[]rune(text)` 为什么不是 `[]byte(text)`：Unicode 安全
- [ ] map 的并发不安全：为什么需要 sync.Mutex
- [ ] `sync.Mutex` vs `sync.RWMutex`：读多写少用哪个
- [ ] GORM tag 语法：`gorm:"uniqueIndex;size:64;not null"`
- [ ] Gin binding tag：`binding:"required,min=6,max=128"`
- [ ] error 的 wrap 和 sentinel：`errors.Is(err, ErrEmailExists)`

---

## 十六、面试场景模拟

- [ ] "介绍一下你这个博客系统的技术架构"
- [ ] "JWT 你是怎么实现的，和 Session 比有什么优劣"
- [ ] "你是怎么保证点赞接口幂等的"
- [ ] "DFA 算法的时间复杂度是多少，为什么不用正则"
- [ ] "评论的树形结构你怎么实现的，查询是怎么优化的"
- [ ] "你的测试是怎么组织的，覆盖率多少"
- [ ] "限流算法你用了哪种，还了解其他方案吗"
- [ ] "如果用户量从 100 涨到 100 万，当前架构有什么瓶颈"
- [ ] "为什么用 PG 而不是 MySQL"
- [ ] "Docker 多阶段构建解决了什么问题"

---

## 复习建议

1. **第一遍**：逐条勾选，标记不熟的
2. **第二遍**：对着未勾选的条目，看源码理解
3. **第三遍**：合上代码，用笔手写核心逻辑（状态机、DFA、JWT 签发校验）
4. **面试前**：对着第十六节的 10 个问题，用手机录音自问自答
