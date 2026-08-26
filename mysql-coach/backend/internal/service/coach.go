package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mysql-coach/backend/internal/llm"
	"github.com/mysql-coach/backend/internal/repository"
	"github.com/mysql-coach/backend/pkg/models"
)

// CoachService 训练引擎核心
// 职责：控制追问流程、判断学生回答、生成笔记
type CoachService struct {
	scenarioRepo  *repository.ScenarioRepo
	sessionRepo   *repository.SessionRepo
	progressRepo  *repository.ProgressRepo
	noteRepo      *repository.NoteRepo
	llmProvider   llm.Provider
}

func NewCoachService(
	sr *repository.ScenarioRepo,
	sr2 *repository.SessionRepo,
	pr *repository.ProgressRepo,
	nr *repository.NoteRepo,
	llm llm.Provider,
) *CoachService {
	return &CoachService{
		scenarioRepo:  sr,
		sessionRepo:   sr2,
		progressRepo:  pr,
		noteRepo:      nr,
		llmProvider:   llm,
	}
}

// StartSession 开始训练：加载场景+策略，返回第一题
func (s *CoachService) StartSession(ctx context.Context, userID, scenarioID string) (*StartSessionResponse, error) {
	// 1. 加载场景（含追问策略 + 标准答案）
	scenario, err := s.scenarioRepo.GetByID(ctx, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("load scenario: %w", err)
	}

	// 2. 创建训练会话
	session := &models.TrainingSession{
		UserID:     userID,
		ScenarioID: scenarioID,
		DomainID:   scenario.DomainID,
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 3. 解析追问策略第 1 层
	level1, err := s.parseLevel(scenario.Strategy, 1)
	if err != nil {
		return nil, fmt.Errorf("parse level 1: %w", err)
	}

	// 4. 记录 Agent 第一条消息
	coachMsg := s.buildCoachOpening(scenario, level1)
	s.sessionRepo.AppendMessage(ctx, session.ID, "coach", coachMsg, 1)

	return &StartSessionResponse{
		SessionID:    session.ID,
		DomainID:     scenario.DomainID,
		ScenarioTitle: scenario.Title,
		Background:   scenario.Background,
		SQLText:      scenario.SQLText,
		CoachMessage: coachMsg,
		CurrentLevel: 1,
		TotalLevels:  scenario.Strategy.TotalLevels,
	}, nil
}

// SubmitAnswer 学生提交回答，引擎判断对错+决定下一步
func (s *CoachService) SubmitAnswer(ctx context.Context, req SubmitAnswerRequest) (*SubmitAnswerResponse, error) {
	// 1. 加载场景+策略
	scenario, err := s.scenarioRepo.GetByID(ctx, req.ScenarioID)
	if err != nil {
		return nil, err
	}

	// 1.5 加载会话，以数据库中的 current_level 为权威进度（不信任前端传参）
	session, err := s.sessionRepo.GetByID(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if session.Status == "completed" {
		return nil, fmt.Errorf("session %s already completed", req.SessionID)
	}
	currentLevel := session.CurrentLevel
	if currentLevel <= 0 {
		currentLevel = 1
	}

	// 2. 记录学生回答（使用权威层级）
	s.sessionRepo.AppendMessage(ctx, req.SessionID, "student", req.Answer, currentLevel)

	// 3. 解析当前层策略
	level, err := s.parseLevel(scenario.Strategy, currentLevel)
	if err != nil {
		return nil, err
	}

	// 4. 判断学生回答
	judgment := s.judgeAnswer(req.Answer, level)

	// 5. 检测常见错误
	mistakeResponse := s.detectCommonMistake(req.Answer, scenario.Strategy)

	// 6. 根据判断决定下一步
	var coachReply string
	var nextLevel int
	var isCompleted bool
	var mastery int

	if judgment.IsCorrect {
		// 答对 → 进下一层
		nextLevel = level.NextIfCorrect
		s.sessionRepo.UpdateProgress(ctx, req.SessionID, currentLevel, true, req.HintsUsed)

		if nextLevel == 0 || nextLevel > scenario.Strategy.TotalLevels {
			// 全部完成
			isCompleted = true
			coachReply = "很好！你已经完成了所有层级的分析。让我帮你整理一份总结笔记。"
			mastery = s.calcMasteryScore(scenario.Strategy.TotalLevels, session.HintsUsed+req.HintsUsed, len(session.LevelsFailed))
			s.sessionRepo.Complete(ctx, req.SessionID, mastery)
		} else {
			// 进下一层
			nextLevelStrategy, _ := s.parseLevel(scenario.Strategy, nextLevel)
			coachReply = s.buildCoachReply(judgment, nextLevelStrategy, false)
		}
	} else {
		// 答错 → 给提示
		nextLevel = currentLevel
		s.sessionRepo.UpdateProgress(ctx, req.SessionID, currentLevel, false, req.HintsUsed)

		if mistakeResponse != "" {
			coachReply = mistakeResponse
		} else {
			coachReply = s.buildCoachReply(judgment, level, true)
		}
	}

	// 7. 记录 Agent 回复
	s.sessionRepo.AppendMessage(ctx, req.SessionID, "coach", coachReply, nextLevel)

	// 8. 如果完成，生成笔记
	var note *models.Note
	if isCompleted {
		note, err = s.generateNote(ctx, req.SessionID, req.UserID, scenario)
		if err == nil {
			s.markCheckpoints(ctx, req.UserID, scenario, req.SessionID, mastery)
		}
	}

	return &SubmitAnswerResponse{
		CoachMessage: coachReply,
		IsCorrect:    judgment.IsCorrect,
		NextLevel:    nextLevel,
		IsCompleted:  isCompleted,
		Note:         note,
	}, nil
}

// ============ 核心判断逻辑 ============

// judgeAnswer 判断学生回答是否正确
// 策略：关键词匹配（v1）+ LLM 语义判断（v2 预留）
func (s *CoachService) judgeAnswer(answer string, level *models.StrategyLevel) Judgment {
	answerLower := strings.ToLower(answer)
	
	// 1. 先检查 acceptable_answers（精确匹配，宽松）
	for _, acceptable := range level.AcceptableAnswers {
		if s.fuzzyMatch(answerLower, strings.ToLower(acceptable)) {
			return Judgment{IsCorrect: true, Feedback: "回答正确！"}
		}
	}

	// 2. 检查 expected_keywords（关键词命中）
	hitCount := 0
	for _, keyword := range level.ExpectedKeywords {
		kw := strings.ToLower(keyword)
		if strings.Contains(answerLower, kw) {
			hitCount++
		}
	}

	// 命中 1 个以上关键词就算通过（宽松匹配，避免误判）
	if hitCount >= 1 && len(level.ExpectedKeywords) > 0 {
		return Judgment{
			IsCorrect: true,
			Feedback: "回答正确，抓住了关键点。",
		}
	}

	// 3. 未通过
	return Judgment{
		IsCorrect:  false,
		Feedback:  "",
		Hint:      level.HintIfWrong,
		HintPartial: level.HintIfPartial,
		HitCount:  hitCount,
		TotalKeys: len(level.ExpectedKeywords),
	}
}

// fuzzyMatch 模糊匹配（关键词包含，不要求完全一致）
func (s *CoachService) fuzzyMatch(answer, target string) bool {
	// 简单实现：如果 target 的每个词都在 answer 里出现
	words := strings.Fields(target)
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		if len(w) < 2 { // 忽略太短的词
			continue
		}
		if !strings.Contains(answer, strings.ToLower(w)) {
			return false
		}
	}
	return true
}

// detectCommonMistake 检测常见错误模式
func (s *CoachService) detectCommonMistake(answer string, strategy *models.CoachingStrategy) string {
	if strategy == nil {
		return ""
	}
	var mistakes []models.CommonMistake
	if err := json.Unmarshal(strategy.CommonMistakes, &mistakes); err != nil {
		return ""
	}

	answerLower := strings.ToLower(answer)
	for _, m := range mistakes {
		// detection 字段格式："回答包含 XXX 但不包含 YYY"
		// 简化：提取关键词检查
		detection := strings.ToLower(m.Detection)
		if strings.Contains(detection, "包含") {
			parts := strings.Split(detection, "但不包含")
			positive := strings.TrimSpace(strings.TrimPrefix(parts[0], "回答包含"))
			if strings.Contains(answerLower, positive) {
				if len(parts) > 1 {
					negative := strings.TrimSpace(parts[1])
					if strings.Contains(answerLower, negative) {
						continue // 有排除项且命中，不算错
					}
				}
				return m.Response
			}
		}
	}
	return ""
}

// ============ 辅助方法 ============

// parseLevel 解析某一层追问策略
func (s *CoachService) parseLevel(strategy *models.CoachingStrategy, level int) (*models.StrategyLevel, error) {
	var levels []models.StrategyLevel
	if err := json.Unmarshal(strategy.Levels, &levels); err != nil {
		return nil, err
	}
	for _, l := range levels {
		if l.Level == level {
			return &l, nil
		}
	}
	return nil, fmt.Errorf("level %d not found", level)
}

// buildCoachOpening 构建 Agent 开场白
func (s *CoachService) buildCoachOpening(scenario *models.Scenario, level *models.StrategyLevel) string {
	var bg struct {
		Env        string `json:"env"`
		Schema     string `json:"schema"`
		DataVolume string `json:"data_volume"`
		Symptom    string `json:"symptom"`
	}
	json.Unmarshal(scenario.Background, &bg)

	return fmt.Sprintf("📋 %s\n\n"+
		"【环境】%s\n"+
		"【表结构】\n%s\n"+
		"【数据量】%s\n"+
		"【性能症状】%s\n\n"+
		"📌 原始 SQL：\n%s\n\n"+
		"🤔 %s",
		scenario.Title,
		bg.Env,
		bg.Schema,
		bg.DataVolume,
		bg.Symptom,
		scenario.SQLText,
		level.CoachQuestion,
	)
}

// buildCoachReply 构建 Agent 回复
func (s *CoachService) buildCoachReply(judgment Judgment, level *models.StrategyLevel, isRetry bool) string {
	if isRetry {
		// 答错 → 给提示 + 重复当前问题
		hint := judgment.Hint
		if judgment.HitCount > 0 && judgment.HintPartial != "" {
			hint = judgment.HintPartial
		}
		if hint == "" {
			hint = judgment.Hint
		}
		return fmt.Sprintf("💡 提示：%s\n\n🤔 %s", hint, level.CoachQuestion)
	}
	// 答对 → 肯定 + 下一题
	return fmt.Sprintf("✅ %s\n\n🤔 %s", judgment.Feedback, level.CoachQuestion)
}

// calcMasteryScore 计算掌握度（1~5）
// 基于总层级、使用提示次数（每次/2）、失败层级数（每次-1）真实计算
func (s *CoachService) calcMasteryScore(totalLevels, hintsUsed, failedLevels int) int {
	score := 5
	score -= failedLevels
	score -= hintsUsed / 2
	if score < 1 {
		score = 1
	}
	if score > 5 {
		score = 5
	}
	return score
}

// generateNote 训练完成自动生成笔记
func (s *CoachService) generateNote(ctx context.Context, sessionID, userID string, scenario *models.Scenario) (*models.Note, error) {
	note := &models.Note{
		UserID:           userID,
		SessionID:        &sessionID,
		ScenarioID:       scenario.ID,
		DomainID:         scenario.DomainID,
		Title:            fmt.Sprintf("场景：%s", scenario.Title),
		StandardAnswerID: scenario.StandardAnswerID,
		StudentAnswers:   make(map[string]string),
		KeyTakeaways:     scenario.Answer.KeyTakeaways,
	}

	// 从标准答案提取薄弱点和要点
	if scenario.Answer != nil {
		note.WeakPoints = []map[string]any{
			{
				"checkpoint_id": "scenario-level",
				"root_cause":    scenario.Answer.RootCause,
			},
		}
	}

	if err := s.noteRepo.Create(ctx, note); err != nil {
		return nil, err
	}

	return note, nil
}

// markCheckpoints 自动打勾考点进度（掌握度与本次训练一致）
func (s *CoachService) markCheckpoints(ctx context.Context, userID string, scenario *models.Scenario, sessionID string, mastery int) {
	for _, cpID := range scenario.CheckpointIDs {
		s.progressRepo.MarkComplete(ctx, userID, cpID, scenario.DomainID, scenario.ID, sessionID, mastery)
	}
}

// ============ 请求/响应结构 ============

type StartSessionResponse struct {
	SessionID     string          `json:"session_id"`
	DomainID      string          `json:"domain_id"`
	ScenarioTitle string          `json:"scenario_title"`
	Background    json.RawMessage `json:"background"`
	SQLText       string          `json:"sql_text"`
	CoachMessage  string          `json:"coach_message"`
	CurrentLevel  int             `json:"current_level"`
	TotalLevels   int             `json:"total_levels"`
}

type SubmitAnswerRequest struct {
	SessionID    string `json:"session_id"`
	UserID       string `json:"user_id"`
	ScenarioID   string `json:"scenario_id"`
	Answer       string `json:"answer"`
	CurrentLevel int    `json:"current_level"`
	HintsUsed    int    `json:"hints_used"`
}

type SubmitAnswerResponse struct {
	CoachMessage string       `json:"coach_message"`
	IsCorrect    bool         `json:"is_correct"`
	NextLevel    int          `json:"next_level"`
	IsCompleted  bool         `json:"is_completed"`
	Note         *models.Note `json:"note,omitempty"`
}

type Judgment struct {
	IsCorrect   bool   `json:"is_correct"`
	Feedback    string `json:"feedback"`
	Hint        string `json:"hint"`
	HintPartial string `json:"hint_partial"`
	HitCount    int    `json:"hit_count"`
	TotalKeys   int    `json:"total_keys"`
}

// 忽略未使用
var _ = time.Now
