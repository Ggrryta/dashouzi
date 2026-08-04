CREATE DATABASE IF NOT EXISTS auction CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE auction;

CREATE TABLE IF NOT EXISTS rooms (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(128)  NOT NULL,
    description VARCHAR(2048),
    owner_id    BIGINT        NOT NULL COMMENT '房间创建者',
    status      VARCHAR(16)   NOT NULL DEFAULT 'online' COMMENT 'online/closed',
    created_at  TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_owner (owner_id),
    INDEX idx_status_created (status, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS items (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    room_id       BIGINT        NOT NULL COMMENT '所属房间',
    seller_id     BIGINT        NOT NULL COMMENT '卖家',
    title         VARCHAR(128)  NOT NULL,
    description   VARCHAR(2048),
    image_url     VARCHAR(512),
    start_price   BIGINT        NOT NULL COMMENT '起拍价，单位分',
    min_increment BIGINT        NOT NULL COMMENT '最小加价幅度，单位分',
    status        VARCHAR(16)   NOT NULL DEFAULT 'pending' COMMENT 'pending/live/closing/sold/failed',
    start_time    TIMESTAMP     NOT NULL,
    end_time      TIMESTAMP     NOT NULL,
    current_price BIGINT        NOT NULL DEFAULT 0 COMMENT '当前最高价，0 表示无人出价',
    bid_count     INT           NOT NULL DEFAULT 0 COMMENT '有效出价次数',
    winner_id     BIGINT        NULL COMMENT '中标者',
    created_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_room_status (room_id, status),
    INDEX idx_status_end_time (status, end_time),
    INDEX idx_start_time (start_time),
    FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS bids (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    item_id    BIGINT        NOT NULL,
    bidder_id  BIGINT        NOT NULL,
    amount     BIGINT        NOT NULL COMMENT '出价金额，单位分',
    bid_time   TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '出价时间，毫秒精度',
    created_at TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_item_time (item_id, bid_time DESC),
    INDEX idx_item_bidder (item_id, bidder_id),
    FOREIGN KEY (item_id) REFERENCES items(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
