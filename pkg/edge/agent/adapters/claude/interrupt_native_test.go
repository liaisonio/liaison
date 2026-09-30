package claude

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

// 显式 opt-in：使用独立、无工具、无持久化的进程，不触碰用户会话。
func TestNativeActiveInterrupt(t *testing.T) {
	if os.Getenv("LIAISON_TEST_CLAUDE_INTERRUPT") != "1" {
		t.Skip("native active interrupt probe not enabled")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
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
	project := t.TempDir()
	var installation discovery.Installation
	for _, candidate := range found.Installations {
		if _, err := launchSpec(candidate, project); err == nil {
			installation = candidate
			break
		}
	}
	if installation.Path == "" {
		t.Fatal("no native Claude installation")
	}
	d, err := StartProbe(ctx, installation, project)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := d.Send(ctx, "Print integers from 1 through 10000, one per line, without skipping any. Do not use tools."); err != nil {
		t.Fatal(err)
	}
	mapper := NewUpdates()
	interrupted := false
	nativeID := ""
	for {
		select {
		case event, ok := <-d.Events():
			if !ok {
				t.Fatal("native stream ended before interrupt result")
			}
			if event.Type == "control_request" {
				t.Fatal("tools-disabled probe requested authorization")
			}
			updates, err := mapper.Apply(event)
			if err != nil {
				t.Fatal("native event normalization failed")
			}
			for _, update := range updates {
				if update.Kind == agentruntime.MessageDelta && update.Text != "" && !interrupted {
					interruptCtx, stop := context.WithTimeout(ctx, 10*time.Second)
					err := d.Interrupt(interruptCtx)
					stop()
					if err != nil {
						t.Fatal("active interrupt was not acknowledged")
					}
					interrupted = true
				}
			}
			if event.Type != "result" {
				continue
			}
			if !interrupted {
				t.Fatal("turn ended before an active interrupt was tested")
			}
			var result struct {
				ID string `json:"session_id"`
			}
			if json.Unmarshal(event.Raw, &result) != nil || result.ID == "" {
				t.Fatal("missing interrupted session identity")
			}
			nativeID = result.ID
			// 只有 result 才视为本轮结束；interrupt ACK 不是执行结束证明。
			if err := d.Send(ctx, "Stop counting. Reply with exactly OK. Do not use tools."); err != nil {
				t.Fatal(err)
			}
			awaitNativeResult(t, ctx, d, nativeID, "OK")
			t.Log("active stream interrupted, terminal result received, next turn completed in same session")
			return
		case <-ctx.Done():
			t.Fatal("active interrupt probe timed out")
		}
	}
}
