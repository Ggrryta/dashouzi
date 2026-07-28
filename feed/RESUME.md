# Feed 流系统

**技术栈：** Go + Gin + MySQL 8.0 + Redis ZSet + Snowflake + Docker

**项目描述：** 社交 Feed 流核心系统，采用推拉结合（Push-Pull Hybrid）架构。普通用户写扩散保证实时性，大V 读扩散避免写扩散风暴，粉丝阈值 10 万自动切换策略。

**核心亮点：**
- **推拉结合架构**：粉丝 < 10 万 → Push 写入粉丝收件箱；粉丝 >= 10 万 → Pull 粉丝拉取大V发件箱。时间线不足时自动触发 Pull 补充，K 路归并排序
- **20 个核心技术点**：Snowflake ID、HackerNews 热度算法、游标分页、重试+死信队列、大V 自动升降级、新粉历史同步、取关收件箱清理
- **异步扩散**：发帖立即返回，goroutine 异步推送粉丝收件箱。失败退避重试 3 次，仍失败入死信队列
- **Redis ZSet 设计**：`timeline:{uid}` 收件箱 / `outbox:{uid}` 发件箱 / `feed:hot` 热门排行。ZREMRANGEBYRANK 自动裁剪上限，ZREVRANGEBYSCORE 游标分页
- **HackerNews 热度公式**：`(likes×2 + comments + 1) / (hours + 2)^1.5`，点赞权重 2:1，指数时间衰减
- **适配器模式**：Feed / Social / Timeline 分别定义接口，5 个适配器解耦，单元测试注入 fake

**工程实践：** 三服务 Docker Compose 编排 | 固定窗口限流 | 39 条单元测试 + 5 条 E2E（build tag） + 3 条性能基准

**测试：** 39 条单元测试 + 5 条 E2E + 3 条压测
