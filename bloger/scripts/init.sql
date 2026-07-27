-- 创建全文搜索扩展
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 文章表全文搜索向量列（如果 GORM 自动迁移没有创建 GENERATED 列，则手动添加）
-- 注意: ALTER TABLE 仅在列不存在时执行

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'articles' AND column_name = 'search_vector'
    ) THEN
        ALTER TABLE articles
        ADD COLUMN search_vector tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
            setweight(to_tsvector('simple', coalesce(content, '')), 'B')
        ) STORED;

        CREATE INDEX articles_search_idx ON articles USING GIN (search_vector);
    END IF;
END $$;
