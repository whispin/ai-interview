package llm

import (
	"context"
	"strings"
	"sync"
)

// Assistant 管理会话上下文。
type Assistant struct {
	streamer     Streamer
	systemPrompt string

	mu      sync.Mutex
	history []Message
}

// NewAssistant 创建助手。
func NewAssistant(streamer Streamer, systemPrompt string) *Assistant {
	return &Assistant{streamer: streamer, systemPrompt: systemPrompt}
}

// History 返回历史消息副本。
func (a *Assistant) History() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()

	dup := make([]Message, len(a.history))
	copy(dup, a.history)
	return dup
}

// Clear 清空历史。
func (a *Assistant) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = nil
}

// StreamAndAppend 追加用户消息并流式获取助手回复，返回完整回复文本。
func (a *Assistant) StreamAndAppend(ctx context.Context, userContent string, handler StreamHandler) (string, error) {
	a.mu.Lock()
	historyCopy := append([]Message(nil), a.history...)
	a.mu.Unlock()

	historyCopy = append(historyCopy, Message{Role: "user", Content: userContent})

	var builder strings.Builder
	err := a.streamer.Stream(ctx, a.prependSystem(historyCopy), func(chunk string, done bool) error {
		if chunk != "" {
			builder.WriteString(chunk)
		}
		if handler != nil {
			return handler(chunk, done)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	reply := builder.String()

	a.mu.Lock()
	a.history = append(a.history, Message{Role: "user", Content: userContent})
	if reply != "" {
		a.history = append(a.history, Message{Role: "assistant", Content: reply})
	}
	a.mu.Unlock()

	return reply, nil
}

func (a *Assistant) prependSystem(messages []Message) []Message {
	if a.systemPrompt == "" {
		return messages
	}
	combined := make([]Message, 0, len(messages)+1)
	combined = append(combined, Message{Role: "system", Content: a.systemPrompt})
	combined = append(combined, messages...)
	return combined
}
