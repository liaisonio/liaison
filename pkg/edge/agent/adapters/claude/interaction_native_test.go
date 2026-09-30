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

// 独立 opt-in：仅允许固定 printf 命令或无副作用提问，不读写用户项目。
func TestNativeInteractions(t *testing.T) {
	if os.Getenv("LIAISON_TEST_CLAUDE_INTERACTIONS") != "1" {
		t.Skip("native interaction probe not enabled")
	}
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		t.Fatal(err)
	}
	found, err := discovery.Find(t.Context(), platform, env, Adapter{}, "")
	if err != nil {
		t.Fatal("discovery failed")
	}
	project := t.TempDir()
	var install discovery.Installation
	for _, i := range found.Installations {
		if _, err := launchSpec(i, project); err == nil {
			install = i
			break
		}
	}
	if install.Path == "" {
		t.Fatal("native installation not found")
	}
	for _, mode := range []string{"allow", "deny", "question"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			spec, err := launchSpec(install, project)
			if err != nil {
				t.Fatal(err)
			}
			tool := "Bash"
			prompt := "Call Bash exactly once with command `printf LIAISON_PERMISSION_PROBE`. Do not use any other command. If denied, do not retry. Then reply DONE."
			if mode == "question" {
				tool = "AskUserQuestion"
				prompt = "Call AskUserQuestion exactly once now. Ask which output style I prefer, with options Brief and Detailed. Wait for the answer, then reply DONE. Do not use any other tool."
			}
			for i := range spec.Args {
				if spec.Args[i] == "--tools" {
					spec.Args[i+1] = tool
				}
			}
			spec.Args = append(spec.Args, "--permission-mode", "default", "--settings", `{"permissions":{"ask":["Bash"]}}`)
			d, err := startDriver(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := d.Send(ctx, prompt); err != nil {
				t.Fatal(err)
			}
			decisions, toolResults := 0, 0
			controller := &Session{ctx: ctx, driver: d, thread: "probe", turn: "turn", updates: make(chan agentruntime.Update, 8)}
			mapper := NewUpdates()
			for {
				select {
				case event, ok := <-d.Events():
					if !ok {
						t.Fatal("protocol closed before completion")
					}
					if _, err := mapper.Apply(event); err != nil {
						t.Fatal("native interaction normalization failed")
					}
					switch event.Type {
					case "control_request":
						if !controller.handleInteraction(event) {
							t.Fatal("shared interaction mapping failed")
						}
						select {
						case u := <-controller.updates:
							if u.Kind != agentruntime.InteractionRequested || u.Interaction == nil {
								t.Fatal("missing shared interaction")
							}
						default:
							t.Fatal("native request not supported by shared interaction mapper")
						}
						var incoming struct {
							Request permission `json:"request"`
						}
						if json.Unmarshal(event.Raw, &incoming) != nil || incoming.Request.Tool != tool {
							t.Fatal("unexpected tool request; no approval sent")
						}
						decisions++
						if decisions > 1 {
							t.Fatal("unexpected repeated permission request")
						}
						if mode == "question" {
							questions, err := d.Questions(event.RequestID)
							if err != nil {
								t.Fatal(err)
							}
							answers := map[string][]string{}
							for _, q := range questions {
								if len(q.Options) == 0 {
									t.Fatal("question has no options")
								}
								answers[q.ID] = []string{q.Options[0].Label}
							}
							if err := controller.Answer(ctx, "turn", event.RequestID, answers); err != nil {
								t.Fatal(err)
							}
						} else {
							var input struct {
								Command string `json:"command"`
							}
							if json.Unmarshal(incoming.Request.Input, &input) != nil || input.Command != "printf LIAISON_PERMISSION_PROBE" {
								t.Fatal("command differs from allowed probe; no approval sent")
							}
							if err := controller.Decide(ctx, "turn", event.RequestID, mode == "allow"); err != nil {
								t.Fatal(err)
							}
						}
					case "user":
						var message struct {
							Message struct {
								Content []struct {
									Type  string `json:"type"`
									Error bool   `json:"is_error"`
								} `json:"content"`
							} `json:"message"`
						}
						if json.Unmarshal(event.Raw, &message) != nil {
							continue
						}
						for _, block := range message.Message.Content {
							if block.Type == "tool_result" {
								toolResults++
								if decisions == 0 {
									t.Fatal("tool executed before remote decision")
								}
								if mode == "deny" && !block.Error {
									t.Fatal("denied command succeeded")
								}
								if mode != "deny" && block.Error {
									t.Fatal("approved interaction failed; raw error withheld")
								}
							}
						}
					case "result":
						var result struct {
							Error bool `json:"is_error"`
						}
						if json.Unmarshal(event.Raw, &result) != nil || result.Error {
							t.Fatal("native turn failed; raw error withheld")
						}
						if decisions != 1 || toolResults == 0 {
							t.Fatal("missing native permission/result roundtrip")
						}
						t.Log("native interaction decision and tool result verified")
						return
					}
				case <-ctx.Done():
					t.Fatal("native interaction timed out")
				}
			}
		})
	}
}
