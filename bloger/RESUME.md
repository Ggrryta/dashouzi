# 博客 CMS 系统

**技术栈：** Go + Gin + PostgreSQL + Redis + JWT + Docker

**项目描述：** 从零搭建的全功能博客平台，涵盖用户、文章、评论、点赞、收藏等模块。采用三层架构（Handler/Service/Repository），统一错误码和响应格式，具备 JWT + RBAC 认证授权体系。

**核心亮点：**
- **文章状态机**：草稿 → 审核 → 发布 → 归档，禁止逆向状态跳转，Service 层纯逻辑无副作用
- **敏感词过滤**：基于 DFA（确定性有限状态自动机）算法，预构建 Trie 树，O(n) 时间复杂度。支持动态添加/删除词库，中文拼音变体识别
- **树形评论**：自关联 parent_id 实现无限层级嵌套，GORM Preload 解决 N+1 查询
- **防刷校验**：Redis 存储点赞/收藏状态，UNIQUE 联合索引防止重复操作
- **分页 + 搜索**：GORM Offset/Limit + ILIKE 全文模糊搜索

**工程实践：** Docker Compose 三服务编排（App + PostgreSQL + Redis）| 多阶段构建 15MB 镜像 | CGO_ENABLED=0 交叉编译 | healthcheck 健康检查 | 结构化日志

**测试：** ~82 条单元测试，覆盖 9 个包
