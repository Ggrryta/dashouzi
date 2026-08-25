package models

import (
	"encoding/json"
	"time"
)

// Domain 领域
type Domain struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description    string    `json:"description"`
	Icon            string    `json:"icon"`
	Version         string    `json:"version"`
	CheckpointCount int       `json:"checkpoint_count"`
	ScenarioCount   int       `json:"scenario_count"`
	IsActive        bool      `json:"is_active"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
}

// Checkpoint 考点
type Checkpoint struct {
	ID                string   `json:"id"`
	DomainID          string   `json:"domain_id"`
	Code              string   `json:"code"`
	Module            string   `json:"module"`
	ModuleName        string   `json:"module_name"`
	Title             string   `json:"title"`
	Difficulty        int      `json:"difficulty"`
	Frequency         string   `json:"frequency"`
	Tags              []string `json:"tags"`
	RelatedScenarios  []string `json:"related_scenarios"`
	SortOrder         int      `json:"sort_order"`
}

// Scenario 场景
type Scenario struct {
	ID                 string            `json:"id"`
	DomainID           string            `json:"domain_id"`
	Title              string            `json:"title"`
	Difficulty         int               `json:"difficulty"`
	Tags               []string          `json:"tags"`
	Background         json.RawMessage   `json:"background"`
	SQLText           string            `json:"sql_text"`
	QuestionPrompt     string            `json:"question_prompt"`
	CheckpointIDs      []string          `json:"checkpoint_ids"`
	PrincipleIDs       []string          `json:"principle_ids"`
	CoachingStrategyID *string           `json:"coaching_strategy_id"`
	StandardAnswerID   *string           `json:"standard_answer_id"`
	Strategy           *CoachingStrategy `json:"strategy,omitempty"`
	Answer             *StandardAnswer   `json:"answer,omitempty"`
	IsPublished        bool              `json:"is_published"`
	SortOrder          int               `json:"sort_order"`
}

// CoachingStrategy 追问策略
type CoachingStrategy struct {
	ID             string          `json:"id"`
	ScenarioID     string          `json:"scenario_id"`
	DomainID       string          `json:"domain_id"`
	TotalLevels    int             `json:"total_levels"`
	Levels         json.RawMessage `json:"levels"`
	CommonMistakes json.RawMessage `json:"common_mistakes"`
}

// StrategyLevel 单层追问策略（用于 JSON 解析）
type StrategyLevel struct {
	Level             int      `json:"level"`
	Name              string   `json:"name"`
	CoachQuestion     string   `json:"coach_question"`
	ExpectedKeywords  []string `json:"expected_keywords"`
	AcceptableAnswers []string `json:"acceptable_answers"`
	HintIfWrong       string   `json:"hint_if_wrong"`
	HintIfPartial     string   `json:"hint_if_partial"`
	NextIfCorrect     int      `json:"next_if_correct"`
	NextIfWrong       int      `json:"next_if_wrong"`
	MaxRetries        int      `json:"max_retries"`
}

// CommonMistake 常见错误应对
type CommonMistake struct {
	MistakePattern string `json:"mistake_pattern"`
	Detection      string `json:"detection"`
	Response       string `json:"response"`
}

// StandardAnswer 标准答案
type StandardAnswer struct {
	ID            string          `json:"id"`
	ScenarioID    string          `json:"scenario_id"`
	DomainID      string          `json:"domain_id"`
	ExecutionPlan json.RawMessage `json:"execution_plan"`
	RootCause     string          `json:"root_cause"`
	Optimization  json.RawMessage `json:"optimization"`
	BeforeAfter   json.RawMessage `json:"before_after"`
	KeyTakeaways  []string        `json:"key_takeaways"`
}

// TrainingSession 训练会话
type TrainingSession struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ScenarioID   string    `json:"scenario_id"`
	DomainID     string    `json:"domain_id"`
	Status       string    `json:"status"`
	CurrentLevel int       `json:"current_level"`
	LevelsPassed []int     `json:"levels_passed"`
	LevelsFailed []int     `json:"levels_failed"`
	HintsUsed    int       `json:"hints_used"`
	MasteryScore *int      `json:"mastery_score"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	NoteID       *string   `json:"note_id"`
}

// Note 笔记
type Note struct {
	ID                string            `json:"id"`
	UserID            string            `json:"user_id"`
	SessionID         *string            `json:"session_id"`
	ScenarioID        string            `json:"scenario_id"`
	DomainID          string            `json:"domain_id"`
	Title             string            `json:"title"`
	StudentAnswers    map[string]string `json:"student_answers"`
	StandardAnswerID  *string           `json:"standard_answer_id"`
	WeakPoints        []map[string]any  `json:"weak_points"`
	KeyTakeaways      []string          `json:"key_takeaways"`
	ReviewCount       int               `json:"review_count"`
	NextReviewAt      *time.Time        `json:"next_review_at"`
	LastReviewedAt    *time.Time        `json:"last_reviewed_at"`
	CreatedAt         time.Time         `json:"created_at"`
}
