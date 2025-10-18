package llm

import "context"

// Message 表示对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamHandler 用于处理流式片段。
type StreamHandler func(chunk string, done bool) error

// Streamer 执行流式对话。
type Streamer interface {
	Stream(ctx context.Context, messages []Message, handler StreamHandler) error
}
