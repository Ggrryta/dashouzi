package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/mysql-coach/backend/pkg/models"
)

// ScenarioRepo 场景数据访问
type ScenarioRepo struct {
	db *sql.DB
}

func NewScenarioRepo(db *sql.DB) *ScenarioRepo {
	return &ScenarioRepo{db: db}
}

func (r *ScenarioRepo) GetByID(ctx context.Context, id string) (*models.Scenario, error) {
	var s models.Scenario
	var background []byte

	err := r.db.QueryRowContext(ctx, `
		SELECT id, domain_id, title, difficulty, tags, background,
			   sql_text, question_prompt, checkpoint_ids, principle_ids,
			   coaching_strategy_id, standard_answer_id, is_published
		FROM scenarios WHERE id = $1
	`, id).Scan(
		&s.ID, &s.DomainID, &s.Title, &s.Difficulty, pq.Array(&s.Tags), &background,
		&s.SQLText, &s.QuestionPrompt, pq.Array(&s.CheckpointIDs), pq.Array(&s.PrincipleIDs),
		&s.CoachingStrategyID, &s.StandardAnswerID, &s.IsPublished,
	)
	if err != nil {
		return nil, fmt.Errorf("get scenario %s: %w", id, err)
	}

	s.Background = json.RawMessage(background)

	// 加载追问策略
	if s.CoachingStrategyID != nil && *s.CoachingStrategyID != "" {
		strategy, err := r.GetStrategy(ctx, *s.CoachingStrategyID)
		if err == nil {
			s.Strategy = strategy
		}
	}

	// 加载标准答案
	if s.StandardAnswerID != nil && *s.StandardAnswerID != "" {
		answer, err := r.GetAnswer(ctx, *s.StandardAnswerID)
		if err == nil {
			s.Answer = answer
		}
	}

	return &s, nil
}

func (r *ScenarioRepo) GetStrategy(ctx context.Context, id string) (*models.CoachingStrategy, error) {
	var s models.CoachingStrategy
	var levels, mistakes []byte

	err := r.db.QueryRowContext(ctx, `
		SELECT id, scenario_id, domain_id, total_levels, levels, common_mistakes
		FROM coaching_strategies WHERE id = $1
	`, id).Scan(
		&s.ID, &s.ScenarioID, &s.DomainID, &s.TotalLevels, &levels, &mistakes,
	)
	if err != nil {
		return nil, err
	}

	s.Levels = json.RawMessage(levels)
	s.CommonMistakes = json.RawMessage(mistakes)
	return &s, nil
}

func (r *ScenarioRepo) GetAnswer(ctx context.Context, id string) (*models.StandardAnswer, error) {
	var a models.StandardAnswer
	var execPlan, opt, ba []byte

	err := r.db.QueryRowContext(ctx, `
		SELECT id, scenario_id, domain_id, execution_plan, root_cause,
			   optimization, before_after, key_takeaways
		FROM standard_answers WHERE id = $1
	`, id).Scan(
		&a.ID, &a.ScenarioID, &a.DomainID, &execPlan, &a.RootCause,
		&opt, &ba, pq.Array(&a.KeyTakeaways),
	)
	if err != nil {
		return nil, err
	}

	a.ExecutionPlan = json.RawMessage(execPlan)
	a.Optimization = json.RawMessage(opt)
	a.BeforeAfter = json.RawMessage(ba)
	return &a, nil
}

// ListByDomain 列出某领域的场景
func (r *ScenarioRepo) ListByDomain(ctx context.Context, domainID string) ([]models.Scenario, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, domain_id, title, difficulty, is_published, sort_order
		FROM scenarios WHERE domain_id = $1 AND is_published = true
		ORDER BY sort_order, difficulty
	`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Scenario
	for rows.Next() {
		var s models.Scenario
		if err := rows.Scan(&s.ID, &s.DomainID, &s.Title, &s.Difficulty,
			&s.IsPublished, &s.SortOrder); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

// ============ Checkpoint ============

type CheckpointRepo struct {
	db *sql.DB
}

func NewCheckpointRepo(db *sql.DB) *CheckpointRepo {
	return &CheckpointRepo{db: db}
}

func (r *CheckpointRepo) ListByDomain(ctx context.Context, domainID string) ([]models.Checkpoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, domain_id, code, module, module_name, title, difficulty,
			   frequency, tags, related_scenarios, sort_order
		FROM checkpoints WHERE domain_id = $1
		ORDER BY sort_order, code
	`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Checkpoint
	for rows.Next() {
		var c models.Checkpoint
		if err := rows.Scan(&c.ID, &c.DomainID, &c.Code, &c.Module, &c.ModuleName,
			&c.Title, &c.Difficulty, &c.Frequency, pq.Array(&c.Tags), pq.Array(&c.RelatedScenarios),
			&c.SortOrder); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

// ============ Training Session ============

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, s *models.TrainingSession) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO training_sessions (user_id, scenario_id, domain_id, status, current_level, messages)
		VALUES ($1, $2, $3, 'in_progress', 1, '[]')
		RETURNING id, started_at
	`, s.UserID, s.ScenarioID, s.DomainID).Scan(&s.ID, &s.StartedAt)
}

// GetByID 加载训练会话（读取权威的 current_level / status，避免信任前端传参）
func (r *SessionRepo) GetByID(ctx context.Context, id string) (*models.TrainingSession, error) {
	var s models.TrainingSession
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, scenario_id, domain_id, status, current_level, hints_used
		FROM training_sessions WHERE id = $1
	`, id).Scan(
		&s.ID, &s.UserID, &s.ScenarioID, &s.DomainID, &s.Status,
		&s.CurrentLevel, &s.HintsUsed,
	)
	if err != nil {
		return nil, fmt.Errorf("get session %s: %w", id, err)
	}
	return &s, nil
}

func (r *SessionRepo) AppendMessage(ctx context.Context, sessionID string, role, content string, level int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE training_sessions
		SET messages = messages || jsonb_build_array(
			jsonb_build_object('role', $1, 'content', $2, 'level', $3, 'timestamp', now())
		)
		WHERE id = $4
	`, role, content, level, sessionID)
	return err
}

func (r *SessionRepo) UpdateProgress(ctx context.Context, sessionID string, level int, passed bool, hintsUsed int) error {
	if passed {
		_, err := r.db.ExecContext(ctx, `
			UPDATE training_sessions
			SET levels_passed = array_append(levels_passed, $1),
			    current_level = $1 + 1,
			    hints_used = $2,
			    updated_at = now()
			WHERE id = $3
		`, level, hintsUsed, sessionID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE training_sessions
		SET levels_failed = array_append(levels_failed, $1),
		    hints_used = $2,
		    updated_at = now()
		WHERE id = $3
	`, level, hintsUsed, sessionID)
	return err
}

func (r *SessionRepo) Complete(ctx context.Context, sessionID string, masteryScore int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE training_sessions
		SET status = 'completed', mastery_score = $1, completed_at = now()
		WHERE id = $2
	`, masteryScore, sessionID)
	return err
}

// ============ Checkpoint Progress ============

type ProgressRepo struct {
	db *sql.DB
}

func NewProgressRepo(db *sql.DB) *ProgressRepo {
	return &ProgressRepo{db: db}
}

func (r *ProgressRepo) MarkComplete(ctx context.Context, userID, checkpointID, domainID, scenarioID, sessionID string, score int) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO checkpoint_progress (user_id, checkpoint_id, domain_id, status, mastery_score, completed_at, scenario_id, session_id)
		VALUES ($1, $2, $3, 'completed', $4, now(), $5, $6)
		ON CONFLICT (user_id, checkpoint_id)
		DO UPDATE SET status = 'completed', mastery_score = $4, completed_at = now(),
		              scenario_id = $5, session_id = $6, updated_at = now()
	`, userID, checkpointID, domainID, score, scenarioID, sessionID)
	return err
}

func (r *ProgressRepo) ListByUser(ctx context.Context, userID, domainID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT checkpoint_id, status FROM checkpoint_progress
		WHERE user_id = $1 AND domain_id = $2
	`, userID, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var cp, status string
		if err := rows.Scan(&cp, &status); err != nil {
			return nil, err
		}
		m[cp] = status
	}
	return m, nil
}

// ============ Domain ============

type DomainRepo struct {
	db *sql.DB
}

func NewDomainRepo(db *sql.DB) *DomainRepo {
	return &DomainRepo{db: db}
}

func (r *DomainRepo) List(ctx context.Context) ([]models.Domain, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, icon, checkpoint_count, scenario_count, is_active, sort_order
		FROM domains WHERE is_active = true ORDER BY sort_order
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Domain
	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.Icon,
			&d.CheckpointCount, &d.ScenarioCount, &d.IsActive, &d.SortOrder); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, nil
}

// ============ Note ============

type NoteRepo struct {
	db *sql.DB
}

func NewNoteRepo(db *sql.DB) *NoteRepo {
	return &NoteRepo{db: db}
}

func (r *NoteRepo) Create(ctx context.Context, n *models.Note) error {
	studentAnswers, _ := json.Marshal(n.StudentAnswers)
	weakPoints, _ := json.Marshal(n.WeakPoints)

	return r.db.QueryRowContext(ctx, `
		INSERT INTO notes (user_id, session_id, scenario_id, domain_id, title,
		                   student_answers, standard_answer_id, weak_points, key_takeaways)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`, n.UserID, n.SessionID, n.ScenarioID, n.DomainID, n.Title,
		studentAnswers, n.StandardAnswerID, weakPoints, n.KeyTakeaways,
	).Scan(&n.ID, &n.CreatedAt)
}

func (r *NoteRepo) ListByUser(ctx context.Context, userID string) ([]models.Note, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, scenario_id, domain_id, title, created_at, review_count
		FROM notes WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Note
	for rows.Next() {
		var n models.Note
		if err := rows.Scan(&n.ID, &n.ScenarioID, &n.DomainID, &n.Title,
			&n.CreatedAt, &n.ReviewCount); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, nil
}

// 忽略未使用的 time
var _ = time.Now
