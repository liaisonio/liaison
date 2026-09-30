package claude

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

// Explicit opt-in: this uses the device owner's existing provider configuration
// and makes a small billable request. No credentials or raw frames are logged.
func TestNativeProbe(t *testing.T) {
	if os.Getenv("LIAISON_TEST_CLAUDE_PROBE") != "1" {
		t.Skip("native Claude probe not enabled")
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
	var installation discovery.Installation
	for _, candidate := range found.Installations {
		if _, err := launchSpec(candidate, t.TempDir()); err == nil {
			installation = candidate
			break
		}
	}
	if installation.Path == "" {
		t.Fatal("no native Claude installation")
	}
	d, err := StartProbe(ctx, installation, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := d.Send(ctx, "Remember the word LIAISON. Reply with exactly OK. Do not use tools."); err != nil {
		t.Fatal(err)
	}
	streamed := false
	mapper := NewUpdates()
	rounds := 0
	nativeSession := ""
	for {
		select {
		case event, ok := <-d.Events():
			if !ok {
				t.Fatalf("protocol ended before result: %v", d.Err())
			}
			if _, err := mapper.Apply(event); err != nil {
				t.Fatal("native update normalization failed")
			}
			if event.Type == "stream_event" {
				streamed = true
			}
			if event.Type == "control_request" {
				t.Fatal("tools-disabled probe requested authorization")
			}
			if event.Type != "result" {
				continue
			}
			var result struct {
				IsError bool   `json:"is_error"`
				Result  string `json:"result"`
				Subtype string `json:"subtype"`
				Session string `json:"session_id"`
			}
			if json.Unmarshal(event.Raw, &result) != nil || result.IsError || result.Subtype != "success" {
				t.Fatal("native turn failed; provider details withheld")
			}
			want := "OK"
			if rounds == 1 {
				want = "LIAISON"
			}
			if strings.TrimSpace(result.Result) != want {
				t.Fatal("native turn returned unexpected text; output withheld")
			}
			if !streamed {
				t.Fatal("no partial stream events")
			}
			if result.Session == "" {
				t.Fatal("missing native session identity")
			}
			if rounds == 0 {
				nativeSession = result.Session
				rounds++
				streamed = false
				mapper = NewUpdates()
				if err := d.Send(ctx, "What word did I ask you to remember? Reply with only that word. Do not use tools."); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if result.Session != nativeSession {
				t.Fatal("native session changed between turns")
			}
			if err := d.Interrupt(ctx); err != nil {
				t.Fatal(err)
			}
			t.Log("native initialization, two streamed turns with shared context/session, and idle interrupt acknowledged")
			return
		case <-ctx.Done():
			t.Fatal("native probe timed out")
		}
	}
}
