package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

func TestLocalHandshake(t *testing.T) {
	if os.Getenv("LIAISON_TEST_CODEX_HANDSHAKE") != "1" {
		t.Skip("opt-in: start own local Codex, no model request")
	}
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		t.Fatal(err)
	}
	result, err := discovery.Find(context.Background(), platform, env, Adapter{}, "")
	if err != nil || len(result.Installations) == 0 {
		t.Fatalf("discovery failed: %v", err)
	}
	timeout := 20 * time.Second
	if os.Getenv("LIAISON_TEST_CODEX_TURN") == "1" {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(project, ".agents", "skills", "liaison-smoke")
	if err := os.MkdirAll(skillDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: liaison-smoke\ndescription: Reply with a readiness marker without tools.\n---\nReply with exactly READY. Do not use tools, read files, or change anything.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	inputTest := os.Getenv("LIAISON_TEST_CODEX_INPUT") == "1"
	session, err := start(ctx, result.Installations[0], project, inputTest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	ready, err := session.CheckAuthentication(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("owned Codex handshake passed, authentication ready=%v (no credentials read by Liaison)", ready)
	models, err := session.Models(ctx)
	if err != nil || len(models) == 0 {
		t.Fatalf("native catalog unavailable: %v", err)
	}
	t.Logf("native model catalog contains %d selectable models", len(models))
	if _, err := session.Send(ctx, "another-users-thread", "hello"); err == nil {
		t.Fatal("foreign thread accepted")
	}
	thread, err := session.NewThread(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if thread.Model == "" {
		t.Fatal("actual model missing from thread/start")
	}
	if os.Getenv("LIAISON_TEST_CODEX_TURN") == "1" {
		skills, err := session.Skills(ctx)
		if err != nil {
			t.Fatal("skills discovery failed", err)
		}
		skillID := ""
		for _, skill := range skills {
			if skill.Name == "liaison-smoke" {
				skillID = skill.ID
			}
		}
		if skillID == "" {
			t.Fatal("project skill missing")
		}
		if _, err := session.SendSkill(ctx, thread.ID, "hello", "unknown"); err == nil {
			t.Fatal("unknown skill accepted")
		}
		prompt := "Reply with exactly READY. Do not call tools, read files, run commands, access the network, or change anything."
		if os.Getenv("LIAISON_TEST_CODEX_WRITE") == "1" {
			if err := session.SetPermissionMode("workspace-write"); err != nil {
				t.Fatal(err)
			}
			prompt = "Create liaison-ready.txt in the current working directory with exactly READY as its content. Do not read or change any other files, do not access the network. Then reply READY."
			skillID = ""
		}
		if inputTest {
			_, err = session.client.Call(ctx, "turn/start", map[string]any{"threadId": thread.ID, "approvalPolicy": "never", "sandboxPolicy": map[string]string{"type": "readOnly"}, "collaborationMode": map[string]any{"mode": "plan", "settings": map[string]any{"model": thread.Model, "reasoning_effort": "low"}}, "input": []map[string]string{{"type": "text", "text": "Do not read files or run tools other than request_user_input. Use request_user_input to ask one question: choose a theme, Light or Dark. Wait for the answer, then reply READY and the selected theme. Do not change any files."}}})
		} else {
			_, err = session.SendSkill(ctx, thread.ID, prompt, skillID)
		}
		if err != nil {
			t.Fatal(err)
		}
		completed := false
		gotText := false
		answered := false
		for !completed {
			select {
			case event, ok := <-session.Events():
				if !ok {
					t.Fatal("protocol closed before turn completed")
				}
				if len(event.ID) > 0 {
					if inputTest && event.Method == "item/tool/requestUserInput" {
						var params struct {
							Questions []struct {
								ID string `json:"id"`
							} `json:"questions"`
						}
						if err := json.Unmarshal(event.Params, &params); err != nil || len(params.Questions) != 1 {
							t.Fatal("unexpected native question")
						}
						if err := session.AnswerInput(ctx, event, map[string][]string{params.Questions[0].ID: {"Dark"}}); err != nil {
							t.Fatal(err)
						}
						answered = true
					} else if err := session.RejectRequest(ctx, event); err != nil {
						t.Fatal(err)
					}
				}
				if event.Method == "item/agentMessage/delta" {
					gotText = true
				}
				if event.Method == "turn/completed" {
					var p struct {
						Turn struct {
							Status string `json:"status"`
						} `json:"turn"`
					}
					if err := json.Unmarshal(event.Params, &p); err != nil {
						t.Fatal(err)
					}
					if p.Turn.Status != "completed" {
						t.Fatalf("turn status: %s", p.Turn.Status)
					}
					completed = true
				}
			case <-ctx.Done():
				t.Fatal("turn completion timed out")
			}
		}
		if !gotText {
			t.Fatal("no streamed assistant text")
		}
		if inputTest && !answered {
			t.Fatal("native request_user_input was not observed")
		}
		if inputTest {
			t.Log("real native question, answer and turn continuation verified in plan mode")
		}
		if os.Getenv("LIAISON_TEST_CODEX_WRITE") == "1" {
			content, err := os.ReadFile(filepath.Join(project, "liaison-ready.txt"))
			if err != nil || string(content) != "READY" && string(content) != "READY\n" {
				t.Fatalf("workspace write not verified: %v", err)
			}
			t.Log("workspace write verified in isolated temporary project")
		} else {
			t.Log("read-only turn completed with streamed assistant output")
		}
	} else {
		return
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-session.Done():
	default:
		t.Fatal("owned process still running")
	}
	resumed, err := Start(ctx, result.Installations[0], project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := resumed.Close(); err != nil {
			t.Error(err)
		}
	})
	reopened, err := resumed.ResumeThread(ctx, thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ID != thread.ID {
		t.Fatalf("resumed thread mismatch: got %q want %q", reopened.ID, thread.ID)
	}
	if _, err := resumed.client.Call(ctx, "thread/archive", map[string]string{"threadId": thread.ID}); err != nil {
		t.Fatal("cleanup persisted smoke thread:", err)
	}
	t.Log("persistent native thread resumed after owned app-server restart")
}
