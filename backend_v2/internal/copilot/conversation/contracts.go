// Package conversation owns Copilot's durable thread and message contract.
package conversation

import "context"

type Citation struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
	Page  int    `json:"page,omitempty"`
}

type Thread struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	UpdatedAt    string `json:"updatedAt"`
	Pinned       bool   `json:"pinned"`
	SDKSessionID string `json:"-"`
}

type ActivityItem struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

type MessageMeta struct {
	NumTurns     int            `json:"numTurns,omitempty"`
	InputTokens  int            `json:"inputTokens,omitempty"`
	OutputTokens int            `json:"outputTokens,omitempty"`
	CostUSD      float64        `json:"costUsd,omitempty"`
	Activity     []ActivityItem `json:"activity,omitempty"`
}

type Message struct {
	ID        string       `json:"id"`
	Role      string       `json:"role"`
	Content   string       `json:"content"`
	Citations []Citation   `json:"citations"`
	Meta      *MessageMeta `json:"meta,omitempty"`
	CreatedAt string       `json:"createdAt"`
}

type ThreadDetail struct {
	Thread   Thread    `json:"thread"`
	Messages []Message `json:"messages"`
}

type StoredMessage struct {
	ID        string
	ThreadID  string
	Role      string
	Content   string
	Citations []Citation
	Meta      *MessageMeta
}

type Store interface {
	CreateThread(ctx context.Context, id, userID, title string) error
	TouchThread(ctx context.Context, threadID string) error
	ListThreads(ctx context.Context, userID string) ([]Thread, error)
	GetThread(ctx context.Context, userID, threadID string) (Thread, bool, error)
	RenameThread(ctx context.Context, userID, threadID, title string) (Thread, bool, error)
	ArchiveThread(ctx context.Context, userID, threadID string) (bool, error)
	SetThreadPinned(ctx context.Context, userID, threadID string, pinned bool) (Thread, bool, error)
	SetThreadSDKSession(ctx context.Context, threadID, sessionID string) error
	AppendMessage(ctx context.Context, message StoredMessage) error
	ListMessages(ctx context.Context, threadID string) ([]Message, error)
}
