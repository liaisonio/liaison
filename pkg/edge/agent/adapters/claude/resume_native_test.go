package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

// 显式 opt-in。仅创建本测试 UUID 的本地 transcript，结束后精确清理。
func TestNativeResume(t *testing.T) {
	if os.Getenv("LIAISON_TEST_CLAUDE_RESUME") != "1" {
		t.Skip("native resume probe not enabled")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		t.Fatal(err)
	}
	found, err := discovery.Find(ctx, platform, env, Adapter{}, "")
	if err != nil {
		t.Fatal("discovery failed")
	}
	var installation discovery.Installation
	for _, candidate := range found.Installations {
		if _, err := launchSpec(candidate, project); err == nil {
			installation = candidate
			break
		}
	}
	if installation.Path == "" {
		t.Fatal("native Claude executable not found")
	}
	id, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := transcriptPath(home, project, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(checkpoint)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("test transcript directory already exists")
	}
	t.Cleanup(func() {
		if err := os.Remove(checkpoint); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Error("could not remove exact test transcript")
		}
		// 仅删除空目录；不递归清理，更不触碰其他会话。
		if err := os.Remove(filepath.Dir(checkpoint)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Log("native CLI left auxiliary test data; directory retained")
		}
	})
	d, err := StartPersistent(ctx, installation, project, id, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	code := strings.ReplaceAll(id, "-", "")[:12]
	if err := d.Send(ctx, "Remember this code: "+code+". Reply exactly OK. Do not use tools."); err != nil {
		t.Fatal(err)
	}
	awaitNativeResult(t, ctx, d, id, "OK")
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkCheckpoint(checkpoint, true); err != nil {
		t.Fatal("closing instance removed or failed to persist history")
	}
	resumed, err := StartPersistent(ctx, installation, project, id, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resumed.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := resumed.Send(ctx, "What code did I ask you to remember? Reply with only the exact code. Do not use tools."); err != nil {
		t.Fatal(err)
	}
	awaitNativeResult(t, ctx, resumed, id, code)
	if err := resumed.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopen the same checkpoint through the provider-neutral runtime contract.
	shared, err := (Adapter{}).Launch(ctx, installation, project)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := shared.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := shared.(agentruntime.ResumableSession).ResumeThread(ctx, id); err != nil {
		t.Fatal(err)
	}
	turn, err := shared.Send(ctx, id, "What code did I ask you to remember? Reply with only the exact code. Do not use tools.")
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	waiting := true
	for waiting {
		select {
		case u, ok := <-shared.(agentruntime.UpdateSession).Updates():
			if !ok {
				t.Fatal("shared session closed before result")
			}
			if u.ThreadID != id || u.TurnID != turn {
				t.Fatal("shared event identity mismatch")
			}
			if u.Kind == agentruntime.MessageDelta {
				text.WriteString(u.Text)
			}
			if u.Kind == agentruntime.TurnEnded {
				if u.Status != "completed" {
					t.Fatal("shared turn failed")
				}
				waiting = false
			}
		case <-ctx.Done():
			t.Fatal("shared session timed out")
		}
	}
	if strings.TrimSpace(text.String()) != code {
		t.Fatal("shared session context mismatch; output withheld")
	}
	if err := shared.Close(); err != nil {
		t.Fatal(err)
	}
	missing, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	if unexpected, err := StartPersistent(ctx, installation, project, missing, true); !errors.Is(err, ErrCheckpoint) {
		if unexpected != nil {
			if closeErr := unexpected.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		t.Fatal("missing checkpoint did not fail closed")
	}
	t.Log("native process restart restored exact session/context; missing checkpoint refused")
}

func awaitNativeResult(t *testing.T, ctx context.Context, d *Driver, id, want string) {
	t.Helper()
	mapper := NewUpdates()
	for {
		select {
		case event, ok := <-d.Events():
			if !ok {
				t.Fatal("native protocol closed before result")
			}
			if _, err := mapper.Apply(event); err != nil {
				t.Fatal("normalization failed")
			}
			if event.Type == "control_request" {
				t.Fatal("tools-disabled session requested a tool")
			}
			if event.Type != "result" {
				continue
			}
			var result struct {
				ID      string `json:"session_id"`
				Text    string `json:"result"`
				Error   bool   `json:"is_error"`
				Subtype string `json:"subtype"`
			}
			if json.Unmarshal(event.Raw, &result) != nil || result.Error || result.Subtype != "success" || result.ID != id || strings.TrimSpace(result.Text) != want {
				t.Fatal("session/context check failed; raw output withheld")
			}
			return
		case <-ctx.Done():
			t.Fatal("native resume probe timed out")
		}
	}
}
