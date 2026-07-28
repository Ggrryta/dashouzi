-- 秒杀系统初始化 SQL

-- 秒杀场次表
CREATE TABLE IF NOT EXISTS seckill_sessions (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    start_time TIMESTAMP    NOT NULL,
    end_time   TIMESTAMP    NOT NULL,
    status     VARCHAR(16)  NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 秒杀商品表
CREATE TABLE IF NOT EXISTS seckill_items (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id   BIGINT       NOT NULL,
    title        VARCHAR(256) NOT NULL,
    price        DECIMAL(10,2) NOT NULL,
    origin_price DECIMAL(10,2) NOT NULL,
    total_stock  INT          NOT NULL,
    sold_count   INT          NOT NULL DEFAULT 0,
    image_url    VARCHAR(512),
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES seckill_sessions(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 秒杀订单表
CREATE TABLE IF NOT EXISTS seckill_orders (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id    BIGINT       NOT NULL,
    session_id BIGINT       NOT NULL,
    item_id    BIGINT       NOT NULL,
    price      DECIMAL(10,2) NOT NULL,
    status     VARCHAR(16)  NOT NULL DEFAULT 'paid',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_item (user_id, item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 测试数据
INSERT IGNORE INTO seckill_sessions (id, name, start_time, end_time, status) VALUES
(1, '双11秒杀专场', '2026-01-01 00:00:00', '2030-12-31 23:59:59', 'active');

INSERT IGNORE INTO seckill_items (id, session_id, title, price, origin_price, total_stock) VALUES
(1, 1, 'iPhone 99', 1.00, 9999.00, 100),
(2, 1, 'AirPods Pro', 0.01, 1999.00, 500);
