package claude

import (
	"crypto/sha256"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

const maxTurnText = 256 << 10

type contentBlock struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Input struct {
		Command string `json:"command"`
		Path    string `json:"file_path"`
	} `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// Updates 每轮独立创建，由唯一事件消费者调用，不在 goroutine 间共享。
// 只转换明确支持的字段；system、配置、未知扩展及供应商错误不会进入 UI。
type Updates struct {
	message  string
	streamed map[string]bool
	complete map[[32]byte]bool
	tools    map[string]string
	bytes    int
	ended    bool
}

func NewUpdates() *Updates {
	return &Updates{streamed: map[string]bool{}, complete: map[[32]byte]bool{}, tools: map[string]string{}}
}

func (u *Updates) text(id, text string) agentruntime.Update {
	left := max(0, maxTurnText-u.bytes)
	value, truncated := clipText(text, left)
	u.bytes += len(value)
	return agentruntime.Update{Kind: agentruntime.MessageDelta, ItemID: id, Text: value, Truncated: truncated}
}

func clipText(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	s = s[:limit]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s, true
}

// Apply 不把 content_block_stop 误当成工具完成；只有 tool_result 才结束工具。
func (u *Updates) Apply(event Event) ([]agentruntime.Update, error) {
	if u.ended {
		return nil, nil
	}
	switch event.Type {
	case "stream_event", "assistant", "user", "result":
	default:
		return nil, nil
	}
	if len(event.Raw) > maxFrame {
		return nil, ErrProtocol
	}
	var frame struct {
		Type    string `json:"type"`
		Parent  string `json:"parent_tool_use_id"`
		Message struct {
			ID      string         `json:"id"`
			Content []contentBlock `json:"content"`
		} `json:"message"`
		Event struct {
			Type    string `json:"type"`
			Index   int    `json:"index"`
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
			Block contentBlock `json:"content_block"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		} `json:"event"`
		IsError bool   `json:"is_error"`
		Subtype string `json:"subtype"`
		Result  string `json:"result"`
	}
	if json.Unmarshal(event.Raw, &frame) != nil || frame.Type != event.Type {
		return nil, ErrProtocol
	}
	// 子 Agent 需要独立的活动归属，未接入前不能冒充主会话正文。
	if frame.Parent != "" {
		return nil, nil
	}
	var out []agentruntime.Update
	switch frame.Type {
	case "stream_event":
		switch frame.Event.Type {
		case "message_start":
			id := frame.Event.Message.ID
			if id == "" || len(id) > 256 {
				return nil, ErrProtocol
			}
			u.message = id
		case "content_block_delta":
			if frame.Event.Delta.Type != "text_delta" {
				return nil, nil
			}
			if u.message == "" || frame.Event.Index < 0 || frame.Event.Index > 1024 {
				return nil, ErrProtocol
			}
			if len(u.streamed) >= 128 && !u.streamed[u.message] {
				return nil, ErrOverflow
			}
			u.streamed[u.message] = true
			out = append(out, u.text(u.message+":"+strconv.Itoa(frame.Event.Index), frame.Event.Delta.Text))
		}
	case "assistant":
		key := sha256.Sum256(event.Raw)
		if u.complete[key] {
			return nil, nil
		}
		id := frame.Message.ID
		if id == "" || len(id) > 256 {
			return nil, ErrProtocol
		}
		for index, b := range frame.Message.Content {
			switch b.Type {
			case "text":
				if u.streamed[id] {
					continue
				}
				out = append(out, u.text(id+":"+strconv.Itoa(index), b.Text))
			case "tool_use":
				if b.ID == "" || len(b.ID) > 256 || len(b.Name) > 256 {
					return nil, ErrProtocol
				}
				if _, exists := u.tools[b.ID]; exists {
					continue
				}
				if len(u.tools) >= 128 {
					return nil, ErrOverflow
				}
				u.tools[b.ID] = b.Name
				command, c := clipText(b.Input.Command, 16384)
				path, p := clipText(b.Input.Path, 4096)
				out = append(out, agentruntime.Update{Kind: agentruntime.ActivityUpdate, ItemID: b.ID, Tool: b.Name, Status: "running", Command: command, Path: path, Truncated: c || p})
			}
		}
		if len(u.complete) >= 128 {
			return nil, ErrOverflow
		}
		u.complete[key] = true
	case "user":
		for _, b := range frame.Message.Content {
			if b.Type != "tool_result" {
				continue
			}
			tool, exists := u.tools[b.ToolUseID]
			if !exists {
				continue
			}
			delete(u.tools, b.ToolUseID)
			status, text := "completed", ""
			if b.IsError {
				status = "failed"
			} else {
				if json.Unmarshal(b.Content, &text) != nil {
					var blocks []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					}
					if json.Unmarshal(b.Content, &blocks) == nil {
						var parts []string
						for _, part := range blocks {
							if part.Type == "text" {
								parts = append(parts, part.Text)
							}
						}
						text = strings.Join(parts, "\n")
					}
				}
			}
			text, truncated := clipText(text, 16384)
			out = append(out, agentruntime.Update{Kind: agentruntime.ActivityUpdate, ItemID: b.ToolUseID, Tool: tool, Status: status, Text: text, Truncated: truncated})
		}
	case "result":
		status := "completed"
		if frame.IsError || frame.Subtype != "success" {
			status = "failed"
		} else if u.bytes == 0 && frame.Result != "" {
			out = append(out, u.text("result", frame.Result))
		}
		out = append(out, agentruntime.Update{Kind: agentruntime.TurnEnded, Status: status})
		u.ended = true
	}
	return out, nil
}
