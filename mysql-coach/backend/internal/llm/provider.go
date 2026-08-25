package llm

import "context"

// Provider 接口：支持任意 OpenAI 协议兼容的 LLM
// 接口隔离，方便随时切换 provider（OpenAI/智谱/通义/自部署）
type Provider interface {
	// Chat 对话生成（非流式）
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ChatStream 对话生成（流式，SSE）
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)

	// Embed 文本向量化
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type ChatMessage struct {
	Role    string `json:"role"`              // system / user / assistant
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type ChatResponse struct {
	Content    string `json:"content"`
	FinishReason string `json:"finish_reason"`
	Usage      struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type StreamChunk struct {
	Content string
	Err     error
	Done    bool
}

// New 根据配置创建对应 provider
func New(provider, apiKey, baseURL, model, embeddingModel string) Provider {
	switch provider {
	case "openai":
		return NewOpenAIProvider(apiKey, baseURL, model, embeddingModel)
	// 后续扩展：
	// case "zhipu":
	//     return NewZhipuProvider(...)
	// case "qwen":
	//     return NewQwenProvider(...)
	default:
		// 默认用 OpenAI 协议（大部分国产模型都兼容）
		return NewOpenAIProvider(apiKey, baseURL, model, embeddingModel)
	}
}
