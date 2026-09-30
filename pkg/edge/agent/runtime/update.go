package runtime

import "context"

// InteractionSession 只接受已保存在设备端的请求 ID，不接受远程改写工具输入。
type InteractionSession interface {
	Decide(context.Context, string, string, bool) error
	Answer(context.Context, string, string, map[string][]string) error
}

type Question struct {
	ID, Header, Text string
	MultiSelect      bool
	Options          []QuestionOption
}
type QuestionOption struct{ Label, Description string }
type Interaction struct {
	ID, Tool, Command, Reason string
	Questions                 []Question
}

// Update 是 Agent 协议到共享展示层的内部增量，不含供应商原始 JSON。
// 与旧 Codex RPC 通道并行迁移，当前未作为新的远程 API 暴露。
type Update struct {
	ThreadID    string
	TurnID      string
	Kind        UpdateKind
	ItemID      string
	Text        string
	Tool        string
	Status      string
	Command     string
	Path        string
	Truncated   bool
	Interaction *Interaction
}

type UpdateKind string

const (
	SessionMetadata      UpdateKind = "session_metadata"
	MessageDelta         UpdateKind = "message_delta"
	ActivityUpdate       UpdateKind = "activity_update"
	TurnEnded            UpdateKind = "turn_ended"
	InteractionRequested UpdateKind = "interaction_requested"
	InteractionResolved  UpdateKind = "interaction_resolved"
)
