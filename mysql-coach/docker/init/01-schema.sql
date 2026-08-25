-- ============================================================
-- 知识库 + 训练引擎 数据库设计 v1
-- PostgreSQL 15+ with pgvector extension
-- ============================================================

-- 扩展：向量检索
CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================================
-- 1. domain（领域）
-- ============================================================
CREATE TABLE domains (
    id              VARCHAR(32) PRIMARY KEY,         -- mysql / redis / jvm
    name            VARCHAR(64) NOT NULL,
    description    TEXT,
    icon            VARCHAR(16),
    version         VARCHAR(16) DEFAULT '1.0',
    checkpoint_count  INT DEFAULT 0,                  -- 冗余，方便展示
    scenario_count    INT DEFAULT 0,
    is_active       BOOLEAN DEFAULT TRUE,
    sort_order      INT DEFAULT 0,                    -- 展示顺序
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- 2. checkpoint（考点）
-- ============================================================
CREATE TABLE checkpoints (
    id              VARCHAR(64) PRIMARY KEY,          -- mysql-B1.1
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    code            VARCHAR(32) NOT NULL,             -- B1.1
    module          VARCHAR(32) NOT NULL,             -- B1
    module_name     VARCHAR(64),                      -- 成本模型
    title           VARCHAR(256) NOT NULL,
    difficulty      SMALLINT DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 5),
    frequency       VARCHAR(16) DEFAULT 'medium',     -- high/medium/low
    tags            TEXT[],                           -- ['成本模型','优化器']
    related_scenarios TEXT[],                         -- ['mysql-scn-01']
    sort_order      INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(domain_id, code)
);

-- 考点依赖关系（多对多）
CREATE TABLE checkpoint_prerequisites (
    checkpoint_id       VARCHAR(64) NOT NULL REFERENCES checkpoints(id) ON DELETE CASCADE,
    prerequisite_id     VARCHAR(64) NOT NULL REFERENCES checkpoints(id) ON DELETE CASCADE,
    PRIMARY KEY(checkpoint_id, prerequisite_id)
);

CREATE INDEX idx_checkpoint_domain ON checkpoints(domain_id);
CREATE INDEX idx_checkpoint_module ON checkpoints(domain_id, module);
CREATE INDEX idx_checkpoint_tags ON checkpoints USING GIN(tags);

-- ============================================================
-- 3. scenario（场景）
-- ============================================================
CREATE TABLE scenarios (
    id              VARCHAR(64) PRIMARY KEY,          -- mysql-scn-01
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    title           VARCHAR(256) NOT NULL,
    difficulty      SMALLINT DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 5),
    tags            TEXT[],
    -- 场景内容（JSONB，结构灵活）
    background      JSONB NOT NULL,                   -- {env, schema, data_volume, symptom}
    sql_text        TEXT NOT NULL,                    -- 原始 SQL
    question_prompt TEXT NOT NULL,                    -- Agent 第一句出题
    -- 关联
    checkpoint_ids  TEXT[],                           -- ['mysql-B1.1', ...]
    principle_ids   TEXT[],                           -- 引用的原理
    coaching_strategy_id VARCHAR(64),                -- 追问策略
    standard_answer_id   VARCHAR(64),                -- 标准答案
    is_published    BOOLEAN DEFAULT FALSE,
    sort_order      INT DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_scenario_domain ON scenarios(domain_id);
CREATE INDEX idx_scenario_tags ON scenarios USING GIN(tags);
CREATE INDEX idx_scenario_checkpoints ON scenarios USING GIN(checkpoint_ids);
CREATE INDEX idx_scenario_background ON scenarios USING GIN(background);

-- ============================================================
-- 4. principle（原理库）
-- ============================================================
CREATE TABLE principles (
    id              VARCHAR(64) PRIMARY KEY,          -- mysql-prin-B1.1
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    checkpoint_id   VARCHAR(64) REFERENCES checkpoints(id),
    title           VARCHAR(256) NOT NULL,
    content         TEXT NOT NULL,                    -- 原理讲解（Agent 引用依据）
    source          VARCHAR(256),                     -- 来源（官方文档/书籍）
    -- 向量字段（RAG 检索用）
    embedding       vector(1536),                     -- OpenAI text-embedding-3-small 维度
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 向量检索索引（HNSW，适合中小规模）
CREATE INDEX idx_principle_embedding ON principles 
    USING hnsw(embedding vector_cosine_ops);

CREATE INDEX idx_principle_domain ON principles(domain_id);
CREATE INDEX idx_principle_checkpoint ON principles(checkpoint_id);

-- ============================================================
-- 5. coaching_strategy（追问策略——核心壁垒）
-- ============================================================
CREATE TABLE coaching_strategies (
    id              VARCHAR(64) PRIMARY KEY,          -- mysql-strat-01
    scenario_id     VARCHAR(64) NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    total_levels    SMALLINT NOT NULL DEFAULT 5,
    -- 追问分层（JSONB 数组，每层含 question/expected/hint/next）
    levels          JSONB NOT NULL,
    -- 常见错误应对（JSONB 数组）
    common_mistakes JSONB DEFAULT '[]',
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_strategy_scenario ON coaching_strategies(scenario_id);
CREATE INDEX idx_strategy_levels ON coaching_strategies USING GIN(levels);

-- ============================================================
-- 6. standard_answer（标准答案）
-- ============================================================
CREATE TABLE standard_answers (
    id              VARCHAR(64) PRIMARY KEY,          -- mysql-ans-01
    scenario_id     VARCHAR(64) NOT NULL REFERENCES scenarios(id) ON DELETE CASCADE,
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    -- 执行计划
    execution_plan  JSONB NOT NULL,                   -- {type, key, rows, extra, ...}
    root_cause      TEXT NOT NULL,                    -- 一句话说瓶颈
    -- 优化方案
    optimization    JSONB NOT NULL,                   -- {index, sql, reason}
    -- 前后对比
    before_after    JSONB NOT NULL,                   -- {before, after}
    key_takeaways   TEXT[],                           -- 要点提炼
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_answer_scenario ON standard_answers(scenario_id);

-- ============================================================
-- 7. users（用户）
-- ============================================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(64) UNIQUE NOT NULL,
    email           VARCHAR(256) UNIQUE,
    password_hash   VARCHAR(256),                     -- 本地注册用（OAuth 可为空）
    avatar          VARCHAR(512),
    -- GitHub OAuth
    github_id       VARCHAR(64) UNIQUE,
    -- 订阅
    subscription    VARCHAR(16) DEFAULT 'free',        -- free / pro
    subscription_expires_at TIMESTAMPTZ,
    -- 训练统计
    total_sessions     INT DEFAULT 0,
    total_checkpoints   INT DEFAULT 0,
    streak_days         INT DEFAULT 0,                -- 连续训练天数
    last_active_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_user_github ON users(github_id);
CREATE INDEX idx_user_subscription ON users(subscription);

-- ============================================================
-- 8. checkpoint_progress（打勾进度）
-- ============================================================
CREATE TABLE checkpoint_progress (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkpoint_id   VARCHAR(64) NOT NULL REFERENCES checkpoints(id),
    domain_id      VARCHAR(32) NOT NULL REFERENCES domains(id),
    status          VARCHAR(16) DEFAULT 'pending',   -- pending/in_progress/completed
    mastery_score   SMALLINT CHECK (mastery_score BETWEEN 1 AND 5),
    completed_at    TIMESTAMPTZ,
    -- 在哪个场景完成的
    scenario_id     VARCHAR(64) REFERENCES scenarios(id),
    session_id      UUID,                             -- 训练会话
    retry_count     INT DEFAULT 0,                    -- 重试次数
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, checkpoint_id)
);

CREATE INDEX idx_progress_user ON checkpoint_progress(user_id);
CREATE INDEX idx_progress_status ON checkpoint_progress(user_id, status);
CREATE INDEX idx_progress_domain ON checkpoint_progress(domain_id);

-- ============================================================
-- 9. training_sessions（训练记录）
-- ============================================================
CREATE TABLE training_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scenario_id     VARCHAR(64) NOT NULL REFERENCES scenarios(id),
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    -- 训练状态
    status          VARCHAR(16) DEFAULT 'in_progress', -- in_progress/completed/abandoned
    current_level   SMALLINT DEFAULT 1,
    levels_passed   SMALLINT[] DEFAULT '{}',           -- [1,2,3]
    levels_failed   SMALLINT[] DEFAULT '{}',
    hints_used      INT DEFAULT 0,
    -- 对话记录（JSONB 数组）
    messages        JSONB DEFAULT '[]',                -- [{role, content, level, timestamp}]
    -- 评分
    mastery_score   SMALLINT,                          -- 训练表现 1~5
    started_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    -- 关联笔记
    note_id         UUID,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_session_user ON training_sessions(user_id);
CREATE INDEX idx_session_scenario ON training_sessions(scenario_id);
CREATE INDEX idx_session_status ON training_sessions(user_id, status);
CREATE INDEX idx_session_messages ON training_sessions USING GIN(messages);

-- ============================================================
-- 10. notes（笔记）
-- ============================================================
CREATE TABLE notes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id      UUID REFERENCES training_sessions(id) ON DELETE SET NULL,
    scenario_id     VARCHAR(64) NOT NULL REFERENCES scenarios(id),
    domain_id       VARCHAR(32) NOT NULL REFERENCES domains(id),
    title           VARCHAR(256) NOT NULL,
    -- 学生回答（JSONB，每层的回答）
    student_answers JSONB NOT NULL,                    -- {level_1: "...", level_2: "..."}
    -- 标准答案引用
    standard_answer_id VARCHAR(64) REFERENCES standard_answers(id),
    -- 薄弱点（JSONB 数组）
    weak_points     JSONB DEFAULT '[]',                -- [{checkpoint_id, mistake, correction}]
    -- 要点提炼
    key_takeaways   TEXT[],
    -- 间隔重复
    review_count    INT DEFAULT 0,
    next_review_at  TIMESTAMPTZ,                      -- 下次复习时间
    last_reviewed_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_note_user ON notes(user_id);
CREATE INDEX idx_note_scenario ON notes(scenario_id);
CREATE INDEX idx_note_domain ON notes(domain_id);
CREATE INDEX idx_note_review ON notes(user_id, next_review_at) WHERE next_review_at IS NOT NULL;
CREATE INDEX idx_note_weak_points ON notes USING GIN(weak_points);

-- ============================================================
-- 触发器：更新统计冗余字段
-- ============================================================

-- domain.checkpoint_count 自动更新
CREATE OR REPLACE FUNCTION update_domain_checkpoint_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE domains SET checkpoint_count = (
        SELECT COUNT(*) FROM checkpoints WHERE domain_id = COALESCE(NEW.domain_id, OLD.domain_id)
    ) WHERE id = COALESCE(NEW.domain_id, OLD.domain_id);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_checkpoint_count
    AFTER INSERT OR DELETE ON checkpoints
    FOR EACH ROW EXECUTE FUNCTION update_domain_checkpoint_count();

-- domain.scenario_count 自动更新
CREATE OR REPLACE FUNCTION update_domain_scenario_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE domains SET scenario_count = (
        SELECT COUNT(*) FROM scenarios WHERE domain_id = COALESCE(NEW.domain_id, OLD.domain_id)
    ) WHERE id = COALESCE(NEW.domain_id, OLD.domain_id);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_scenario_count
    AFTER INSERT OR DELETE ON scenarios
    FOR EACH ROW EXECUTE FUNCTION update_domain_scenario_count();

-- users.total_sessions 自动更新
CREATE OR REPLACE FUNCTION update_user_session_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE users SET 
        total_sessions = (SELECT COUNT(*) FROM training_sessions WHERE user_id = COALESCE(NEW.user_id, OLD.user_id)),
        last_active_at = NOW()
    WHERE id = COALESCE(NEW.user_id, OLD.user_id);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_session_count
    AFTER INSERT OR DELETE ON training_sessions
    FOR EACH ROW EXECUTE FUNCTION update_user_session_count();

-- users.total_checkpoints 自动更新
CREATE OR REPLACE FUNCTION update_user_checkpoint_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE users SET 
        total_checkpoints = (
            SELECT COUNT(*) FROM checkpoint_progress 
            WHERE user_id = COALESCE(NEW.user_id, OLD.user_id) AND status = 'completed'
        )
    WHERE id = COALESCE(NEW.user_id, OLD.user_id);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_checkpoint_progress_count
    AFTER INSERT OR UPDATE OF status ON checkpoint_progress
    FOR EACH ROW EXECUTE FUNCTION update_user_checkpoint_count();

-- ============================================================
-- 初始化数据：MySQL 领域
-- ============================================================
INSERT INTO domains (id, name, description, icon, is_active, sort_order)
VALUES ('mysql', 'MySQL', 'MySQL 语句调优与原理', '🐬', TRUE, 1)
ON CONFLICT (id) DO NOTHING;
