package social

import (
	"context"
	"database/sql"
	"fmt"
)

// MySQLRepo 社交关系 MySQL 仓库
type MySQLRepo struct {
	db *sql.DB
}

// NewMySQLRepo 创建 MySQL 仓库
func NewMySQLRepo(db *sql.DB) *MySQLRepo {
	return &MySQLRepo{db: db}
}

// InitSchema 初始化表结构
func (r *MySQLRepo) InitSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS follows (
			follower_id BIGINT NOT NULL,
			followee_id BIGINT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (follower_id, followee_id),
			INDEX idx_followee (followee_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS big_v_users (
			user_id BIGINT PRIMARY KEY,
			follower_count INT DEFAULT 0,
			is_big_v TINYINT DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range queries {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("init social schema: %w", err)
		}
	}
	return nil
}

// AddFollow 添加关注关系
func (r *MySQLRepo) AddFollow(ctx context.Context, followerID, followeeID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 写入关注关系
	_, err = tx.ExecContext(ctx,
		`INSERT IGNORE INTO follows (follower_id, followee_id) VALUES (?, ?)`,
		followerID, followeeID)
	if err != nil {
		return err
	}

	// 更新被关注者粉丝计数
	_, err = tx.ExecContext(ctx,
		`INSERT INTO big_v_users (user_id, follower_count) VALUES (?, 1)
		 ON DUPLICATE KEY UPDATE follower_count = follower_count + 1`,
		followeeID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// RemoveFollow 移除关注关系
func (r *MySQLRepo) RemoveFollow(ctx context.Context, followerID, followeeID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`DELETE FROM follows WHERE follower_id = ? AND followee_id = ?`,
		followerID, followeeID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return tx.Commit() // 没有关注记录，无需递减
	}

	// 递减粉丝计数（不低于 0）
	_, err = tx.ExecContext(ctx,
		`UPDATE big_v_users SET follower_count = GREATEST(follower_count - 1, 0) WHERE user_id = ?`,
		followeeID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// FollowerCount 获取粉丝数
func (r *MySQLRepo) FollowerCount(ctx context.Context, userID int64) int {
	var count int
	r.db.QueryRowContext(ctx,
		`SELECT COALESCE(follower_count, 0) FROM big_v_users WHERE user_id = ?`,
		userID).Scan(&count)
	if count == 0 {
		// fallback: 从 follows 表统计
		r.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM follows WHERE followee_id = ?`,
			userID).Scan(&count)
	}
	return count
}

// GetFollowers 获取粉丝列表
func (r *MySQLRepo) GetFollowers(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT follower_id FROM follows WHERE followee_id = ?`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followers []int64
	for rows.Next() {
		var fid int64
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		followers = append(followers, fid)
	}
	return followers, rows.Err()
}

// GetFollowing 获取关注列表
func (r *MySQLRepo) GetFollowing(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT followee_id FROM follows WHERE follower_id = ?`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followees []int64
	for rows.Next() {
		var fid int64
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		followees = append(followees, fid)
	}
	return followees, rows.Err()
}

// IsFollowing 是否已关注
func (r *MySQLRepo) IsFollowing(ctx context.Context, followerID, followeeID int64) bool {
	var count int
	r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM follows WHERE follower_id = ? AND followee_id = ?`,
		followerID, followeeID).Scan(&count)
	return count > 0
}

// SetBigV 设置大V状态
func (r *MySQLRepo) SetBigV(ctx context.Context, userID int64, isBigV bool) error {
	if isBigV {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO big_v_users (user_id, follower_count, is_big_v) VALUES (?, 0, 1)
			 ON DUPLICATE KEY UPDATE is_big_v = 1`, userID)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE big_v_users SET is_big_v = 0 WHERE user_id = ?`, userID)
	return err
}

// IsBigVUser 是否为大V
func (r *MySQLRepo) IsBigVUser(ctx context.Context, userID int64) bool {
	var isBigV bool
	r.db.QueryRowContext(ctx,
		`SELECT COALESCE(is_big_v, 0) FROM big_v_users WHERE user_id = ?`,
		userID).Scan(&isBigV)
	return isBigV
}
