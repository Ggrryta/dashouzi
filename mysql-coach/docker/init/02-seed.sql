-- ============================================================
-- 种子数据：MySQL 场景 1（用户查询列表慢）
-- 包含：考点 + 场景 + 原理 + 追问策略 + 标准答案
-- ============================================================

-- 考点
INSERT INTO checkpoints (id, domain_id, code, module, module_name, title, difficulty, frequency, tags, related_scenarios, sort_order)
VALUES
  ('mysql-B1.1', 'mysql', 'B1.1', 'B1', '成本模型', '优化器按 IO+CPU 成本选最低路径', 2, 'high', ARRAY['成本模型','优化器','索引选择'], ARRAY['mysql-scn-01'], 1),
  ('mysql-B1.3', 'mysql', 'B1.3', 'B1', '成本模型', '回表是随机 IO（贵）', 3, 'high', ARRAY['回表','随机IO','二级索引'], ARRAY['mysql-scn-01'], 3),
  ('mysql-B1.4', 'mysql', 'B1.4', 'B1', '成本模型', '命中行数占比过高 → 优化器放弃索引', 3, 'high', ARRAY['选择性','索引放弃'], ARRAY['mysql-scn-01'], 4),
  ('mysql-B2.1', 'mysql', 'B2.1', 'B2', '索引选择性', '选择性 = 命中行数/总行数，越小越好', 2, 'high', ARRAY['选择性','基数'], ARRAY['mysql-scn-01'], 5),
  ('mysql-B4.1', 'mysql', 'B4.1', 'B4', '覆盖索引', '覆盖索引 = 返回列都在索引里', 2, 'high', ARRAY['覆盖索引','回表'], ARRAY['mysql-scn-01'], 10),
  ('mysql-E1', 'mysql', 'E1', 'E', '边界判断', '结果集大 → 优化在业务层', 3, 'high', ARRAY['结果集边界','业务优化'], ARRAY['mysql-scn-01'], 60)
ON CONFLICT DO NOTHING;

-- 考点依赖
INSERT INTO checkpoint_prerequisites (checkpoint_id, prerequisite_id) VALUES
  ('mysql-B1.3', 'mysql-B1.1'),
  ('mysql-B1.4', 'mysql-B2.1'),
  ('mysql-B4.1', 'mysql-B1.3')
ON CONFLICT DO NOTHING;

-- 原理库
INSERT INTO principles (id, domain_id, checkpoint_id, title, content, source) VALUES
  ('mysql-prin-B1.1', 'mysql', 'mysql-B1.1', '优化器成本模型',
   '优化器按 IO成本+CPU成本 选最低路径。IO成本=读页次数×单页IO代价；CPU成本=处理行数×单行CPU代价。回表是随机IO，全表扫描是顺序IO，命中行数占比过高时回表代价>全表扫描，优化器放弃索引。',
   'MySQL官方文档 + 高性能MySQL'),
  ('mysql-prin-B1.3', 'mysql', 'mysql-B1.3', '回表是随机IO',
   '二级索引叶子节点存主键ID，命中后要拿主键回聚簇索引取整行=回表。回表是随机IO（离散页访问），全表扫描是顺序IO（连续页）。命中行数多且分散时，回表总成本>全表扫描。',
   'MySQL官方文档'),
  ('mysql-prin-B2.1', 'mysql', 'mysql-B2.1', '索引选择性',
   '选择性=命中行数/总行数，越小越好。选择性差的列（如status占80%）即使建索引也缩不掉范围。过滤列选择性差时联合索引救不了回表问题。',
   '高性能MySQL')
ON CONFLICT DO NOTHING;

-- 场景（先插，因为 standard_answers 和 coaching_strategies 有外键引用 scenarios）
INSERT INTO scenarios (
    id, domain_id, title, difficulty, tags, background,
    sql_text, question_prompt, checkpoint_ids, principle_ids,
    coaching_strategy_id, standard_answer_id, is_published, sort_order
) VALUES (
    'mysql-scn-01', 'mysql', '用户查询列表慢', 1,
    ARRAY['索引缺失','成本模型','结果集边界'],
    '{"env":"MySQL 8.0, InnoDB, RR","schema":"CREATE TABLE t_user (id BIGINT PRIMARY KEY, name VARCHAR(64), email VARCHAR(128), status TINYINT, created_at DATETIME, KEY idx_name(name))","data_volume":"500w行，status=1占80%，name姓王占20%","symptom":"接口从300ms恶化到6s，CPU高"}'::jsonb,
    'SELECT id, name, email, status, created_at FROM t_user WHERE status = 1 AND name LIKE ''王%'';',
    '这条SQL慢了，你来分析一下。先猜猜 type 会是什么？key 会用哪个索引？预计扫多少行？',
    ARRAY['mysql-B1.1','mysql-B1.3','mysql-B2.1','mysql-B4.1','mysql-E1'],
    ARRAY['mysql-prin-B1.1','mysql-prin-B1.3','mysql-prin-B2.1'],
    'mysql-strat-01', 'mysql-ans-01', true, 1
)
ON CONFLICT (id) DO UPDATE SET
    background = EXCLUDED.background,
    sql_text = EXCLUDED.sql_text,
    question_prompt = EXCLUDED.question_prompt,
    is_published = true;

-- 标准答案（场景之后插）
INSERT INTO standard_answers (id, scenario_id, domain_id, execution_plan, root_cause, optimization, before_after, key_takeaways) VALUES
  ('mysql-ans-01', 'mysql-scn-01', 'mysql',
   '{"type":"ALL","key":"NULL","possible_keys":"idx_name","rows":5000000,"filtered":1.60,"extra":"Using where"}'::jsonb,
   'idx_name命中约100万行，回表100万次随机IO代价 > 全表扫描500万行顺序读，优化器放弃索引。',
   '{"index":"ALTER TABLE t_user ADD INDEX idx_name_status_cover (name, status, email, created_at);","reason":"name选择性最好放最左，status过滤，email/created_at凑覆盖免回表"}'::jsonb,
   '{"before":"type=ALL, rows=500万, Extra=Using where, 耗时6s","after":"type=range, rows=8万, Extra=Using index, 耗时<100ms"}'::jsonb,
   ARRAY['优化器按成本选计划，不是看有没有索引','回表是随机IO，命中行数多时代价大','覆盖索引免回表但有成本需权衡','结果集大时优化在业务层']
  )
ON CONFLICT DO NOTHING;

-- 追问策略（核心！）
INSERT INTO coaching_strategies (id, scenario_id, domain_id, total_levels, levels, common_mistakes) VALUES
  ('mysql-strat-01', 'mysql-scn-01', 'mysql', 5,
   '[
     {
       "level": 1,
       "name": "预判",
       "coach_question": "先猜猜这条SQL的type会是什么？key会用哪个索引？预计扫多少行？",
       "expected_keywords": ["all", "全表扫描", "500万"],
       "acceptable_answers": ["type=all，没用索引，扫全表", "type是all因为回表代价大于全表扫描"],
       "hint_if_wrong": "想想name LIKE 王命中多少行，回表代价大不大？优化器是算代价的。",
       "hint_if_partial": "type判断对了，但能说说为什么优化器不用索引吗？",
       "next_if_correct": 2,
       "next_if_wrong": 1,
       "max_retries": 2
     },
     {
       "level": 2,
       "name": "读计划",
       "coach_question": "真实执行计划是 type=ALL, key=NULL, rows=500万, filtered=1.60。possible_keys=idx_name但key=NULL说明什么？filtered=1.60是什么意思？",
       "expected_keywords": ["放弃索引", "回表代价", "过滤比例", "1.6%"],
       "acceptable_answers": ["索引存在但优化器放弃了因为回表代价大", "filtered=1.6%表示扫500万行只有1.6%满足条件"],
       "hint_if_wrong": "possible_keys有但key=NULL说明优化器算了成本后放弃了。算的是什么成本？",
       "next_if_correct": 3,
       "next_if_wrong": 2,
       "max_retries": 2
     },
     {
       "level": 3,
       "name": "定位",
       "coach_question": "瓶颈到底在哪？是索引缺失、顺序不对、还是写法问题？",
       "expected_keywords": ["索引缺失", "回表", "覆盖索引", "结果集边界"],
       "acceptable_answers": ["索引缺失+回表代价大应该建覆盖索引", "结果集只有8万行但要扫500万应该用覆盖索引避免回表"],
       "hint_if_wrong": "想想如果要返回8万行结果但结果集本身就是瓶颈——SQL优化有边界吗？",
       "next_if_correct": 4,
       "next_if_wrong": 3,
       "max_retries": 2
     },
     {
       "level": 4,
       "name": "方案",
       "coach_question": "你会怎么改？建什么索引？为什么？",
       "expected_keywords": ["联合索引", "name在前", "覆盖", "status"],
       "acceptable_answers": ["建idx(name,status,email,created_at)覆盖索引", "name放最左因为选择性最好其余列凑覆盖"],
       "hint_if_wrong": "name放最左还是status？想想哪个选择性更好？覆盖列要塞哪些？",
       "next_if_correct": 5,
       "next_if_wrong": 4,
       "max_retries": 2
     },
     {
       "level": 5,
       "name": "对比",
       "coach_question": "改完预期type/rows/Extra变成什么？怎么验证优化生效了？",
       "expected_keywords": ["ref或range", "8万或更少", "using index", "对比耗时"],
       "acceptable_answers": ["type变成range rows降到几万 Extra有Using index", "用EXPLAIN前后对比看耗时"],
       "hint_if_wrong": "建了覆盖索引后type应该从ALL变成什么？Extra会出现什么标志？",
       "next_if_correct": 0,
       "next_if_wrong": 5,
       "max_retries": 2
     }
   ]'::jsonb,
   '[
     {"mistake_pattern":"学生说type=range","detection":"回答包含range但不包含all","response":"等值走ref范围才走range。这里name LIKE 王是范围吗？还是优化器直接放弃了？"},
     {"mistake_pattern":"学生认为索引一定能加速","detection":"回答包含 一定 或 肯定能","response":"索引不是万能的。当命中行数占比过高回表代价>全表扫描优化器会放弃索引。"},
     {"mistake_pattern":"学生想用分表解决","detection":"回答包含 分表 或 分库","response":"分表是架构层方案不是语句调优。先SQL层能做的做完——能先想SQL层怎么优化吗？"}
   ]'::jsonb
  )
ON CONFLICT (id) DO UPDATE SET
    levels = EXCLUDED.levels,
    common_mistakes = EXCLUDED.common_mistakes;
