package claude

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

const questionInput = `{"questions":[{"header":"模式","question":"选择模式？","options":[{"label":"Alpha","description":"甲"},{"label":"Beta","description":"乙"}]},{"question":"选择内容？","multiSelect":true,"options":[{"label":"A"},{"label":"B"}]}]}`

func questionFixture(t *testing.T) *fixture {
	f := newFixture(t)
	f.emit(t, `{"type":"control_request","request_id":"ask-1","request":{"subtype":"can_use_tool","tool_name":"AskUserQuestion","input":`+questionInput+`}}`)
	nextEvent(t, f.client)
	return f
}

func TestQuestionAnswer(t *testing.T) {
	f := questionFixture(t)
	questions, err := f.client.Questions("ask-1")
	if err != nil || len(questions) != 2 || questions[0].ID != "q1" || !questions[1].MultiSelect {
		t.Fatal("question mapping failed")
	}
	if err = f.client.Decide(t.Context(), "ask-1", true); !errors.Is(err, ErrAnswer) {
		t.Fatal("question approved without answers")
	}
	if err = f.client.Answer(t.Context(), "ask-1", map[string][]string{"q1": {"自定义模式"}, "q2": {"A", "B"}}); err != nil {
		t.Fatal(err)
	}
	request := f.request(t)
	var got struct {
		Decision struct {
			Behavior string `json:"behavior"`
			Input    struct {
				Questions json.RawMessage            `json:"questions"`
				Answers   map[string]json.RawMessage `json:"answers"`
			} `json:"updatedInput"`
		} `json:"response"`
	}
	if json.Unmarshal(request["response"], &got) != nil || got.Decision.Behavior != "allow" {
		t.Fatal("wrong response")
	}
	if string(got.Decision.Input.Answers["选择模式？"]) != `"自定义模式"` || string(got.Decision.Input.Answers["选择内容？"]) != `["A","B"]` {
		t.Fatal("answers not mapped to original questions")
	}
	if len(got.Decision.Input.Questions) == 0 {
		t.Fatal("original questions lost")
	}
	if err = f.client.Answer(t.Context(), "ask-1", nil); !errors.Is(err, ErrApproval) {
		t.Fatal("answer replay accepted")
	}
}

func TestInvalidAnswersDoNotConsumeQuestion(t *testing.T) {
	for name, answers := range map[string]map[string][]string{
		"missing": {}, "unknown id": {"q1": {"Alpha"}, "q3": {"A"}},
		"single multiple": {"q1": {"Alpha", "Beta"}, "q2": {"A"}},
		"empty":           {"q1": {}, "q2": {"A"}}, "blank": {"q1": {" "}, "q2": {"A"}},
		"duplicate": {"q1": {"Alpha"}, "q2": {"A", "A"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := questionFixture(t)
			if err := f.client.Answer(t.Context(), "ask-1", answers); !errors.Is(err, ErrAnswer) {
				t.Fatal("invalid answer accepted")
			}
			if _, err := f.client.Questions("ask-1"); err != nil {
				t.Fatal("validation consumed request")
			}
		})
	}
}

func TestQuestionCancellation(t *testing.T) {
	f := questionFixture(t)
	f.emit(t, `{"type":"control_cancel_request","request_id":"ask-1"}`)
	nextEvent(t, f.client)
	if _, err := f.client.Questions("ask-1"); !errors.Is(err, ErrApproval) {
		t.Fatal("cancelled question retained")
	}
	if err := f.client.Answer(t.Context(), "ask-1", map[string][]string{"q1": {"Alpha"}, "q2": {"A"}}); !errors.Is(err, ErrApproval) {
		t.Fatal("cancelled answer accepted")
	}
}

func TestQuestionConcurrentAnswerIsOneShot(t *testing.T) {
	f := questionFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- f.client.Answer(t.Context(), "ask-1", map[string][]string{"q1": {"Alpha"}, "q2": {"A"}})
		}()
	}
	wg.Wait()
	close(results)
	allowed, rejected := 0, 0
	for err := range results {
		if err == nil {
			allowed++
		} else if errors.Is(err, ErrApproval) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if allowed != 1 || rejected != 1 {
		t.Fatal("concurrent answer replay")
	}
}

func TestMalformedQuestions(t *testing.T) {
	for name, input := range map[string]string{
		"empty": "{}", "duplicate text": `{"questions":[{"question":"same"},{"question":"same"}]}`,
		"duplicate label": `{"questions":[{"question":"Q","options":[{"label":"A"},{"label":"A"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseQuestions(json.RawMessage(input)); !errors.Is(err, ErrAnswer) {
				t.Fatal("malformed questions accepted")
			}
		})
	}
}
