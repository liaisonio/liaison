package claude

import (
	"context"
	"encoding/json"
	"errors"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"strconv"
	"strings"
)

var ErrAnswer = errors.New("invalid claude user answer")

// Question 是内部适配结果，不携带模型生成的 HTML preview。
// ID 由本地生成，回答时不会让浏览器改写原问题或工具输入。
type Question = agentruntime.Question
type QuestionOption = agentruntime.QuestionOption

func parseQuestions(input json.RawMessage) ([]Question, error) {
	if len(input) > 24<<10 {
		return nil, ErrAnswer
	}
	var data struct {
		Questions []struct {
			Header      string `json:"header"`
			Question    string `json:"question"`
			MultiSelect bool   `json:"multiSelect"`
			Options     []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(input, &data) != nil || len(data.Questions) == 0 || len(data.Questions) > 4 {
		return nil, ErrAnswer
	}
	questions := make([]Question, 0, len(data.Questions))
	seen := map[string]bool{}
	for i, q := range data.Questions {
		if strings.TrimSpace(q.Question) == "" || len(q.Question) > 4096 || len(q.Header) > 256 || len(q.Options) > 8 || seen[q.Question] {
			return nil, ErrAnswer
		}
		seen[q.Question] = true
		view := Question{ID: "q" + strconv.Itoa(i+1), Header: q.Header, Text: q.Question, MultiSelect: q.MultiSelect}
		labels := map[string]bool{}
		for _, o := range q.Options {
			if strings.TrimSpace(o.Label) == "" || len(o.Label) > 512 || len(o.Description) > 2048 || labels[o.Label] {
				return nil, ErrAnswer
			}
			labels[o.Label] = true
			view.Options = append(view.Options, QuestionOption{Label: o.Label, Description: o.Description})
		}
		questions = append(questions, view)
	}
	return questions, nil
}

// Questions 仅查看仍然待处理的提问。取消或作答后立即失效。
func (c *Client) Questions(requestID string) ([]Question, error) {
	c.mu.Lock()
	p, ok := c.approvals[requestID]
	c.mu.Unlock()
	if !ok {
		return nil, ErrApproval
	}
	if p.Tool != "AskUserQuestion" {
		return nil, ErrAnswer
	}
	return parseQuestions(p.Input)
}

// Answer 允许选项和自由文本，校验单选/多选、数量、长度、重复项。
// 原始工具输入保留在 Edge；仅替换 answers，不接受任意 updatedInput。
func (c *Client) Answer(ctx context.Context, requestID string, answers map[string][]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	p, ok := c.approvals[requestID]
	if !ok {
		c.mu.Unlock()
		return ErrApproval
	}
	if p.Tool != "AskUserQuestion" {
		c.mu.Unlock()
		return ErrAnswer
	}
	questions, err := parseQuestions(p.Input)
	if err != nil || len(answers) != len(questions) {
		c.mu.Unlock()
		return ErrAnswer
	}
	// SDK 以原始 question 文本为 key，而浏览器使用本地生成的 q1/q2。
	native := make(map[string]any, len(questions))
	total := 0
	for _, q := range questions {
		values, exists := answers[q.ID]
		if !exists || len(values) == 0 || len(values) > 8 || (!q.MultiSelect && len(values) != 1) {
			c.mu.Unlock()
			return ErrAnswer
		}
		seen := map[string]bool{}
		for _, v := range values {
			total += len(v)
			if strings.TrimSpace(v) == "" || len(v) > 4096 || total > 16384 || seen[v] {
				c.mu.Unlock()
				return ErrAnswer
			}
			seen[v] = true
		}
		if q.MultiSelect {
			native[q.Text] = append([]string(nil), values...)
		} else {
			native[q.Text] = values[0]
		}
	}
	var original map[string]json.RawMessage
	if json.Unmarshal(p.Input, &original) != nil {
		c.mu.Unlock()
		return ErrAnswer
	}
	encoded, err := json.Marshal(native)
	if err != nil {
		c.mu.Unlock()
		return ErrAnswer
	}
	original["answers"] = encoded
	updated, err := json.Marshal(original)
	if err != nil {
		c.mu.Unlock()
		return ErrAnswer
	}
	// 在 IO 前消费，失败也不能自动重放。
	delete(c.approvals, requestID)
	c.mu.Unlock()
	return c.respond(ctx, requestID, struct {
		Behavior string          `json:"behavior"`
		Input    json.RawMessage `json:"updatedInput"`
	}{"allow", updated})
}
