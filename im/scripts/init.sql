CREATE TABLE IF NOT EXISTS users (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    username   VARCHAR(64) NOT NULL UNIQUE,
    password   VARCHAR(256) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS im_messages (
    id         BIGINT AUTO_INCREMENT PRIMARY KEY,
    msg_id     VARCHAR(36) NOT NULL UNIQUE,
    from_user  BIGINT NOT NULL,
    to_user    BIGINT NOT NULL,
    content    TEXT NOT NULL,
    msg_type   VARCHAR(16) NOT NULL DEFAULT 'text',
    status     VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_from_user (from_user, created_at),
    INDEX idx_to_user_status (to_user, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
