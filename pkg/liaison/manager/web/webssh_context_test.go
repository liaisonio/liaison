package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestShellContext_FragmentedMarkersAndSharing(t *testing.T) {
	stream := "\x1b]633;P;Cwd=/tmp/work\\x20dir\a\x1b]633;A\aroot$ \x1b]633;B\aDRAFT_NOT_COLLECTED\x1b]633;E;cat\\x20missing\a\x1b]633;C\a\x1b[31mfile missing\x1b[0m\n\x1b]633;D;1\x1b\\"
	for _, size := range []int{1, 2, 7, len(stream)} {
		var s shellContext
		for start := 0; start < len(stream); start += size {
			end := start + size
			if end > len(stream) {
				end = len(stream)
			}
			s.observe(stream[start:end])
		}
		require.Len(t, s.commands, 1)
		require.Equal(t, "cat missing", s.commands[0].Command)
		require.Equal(t, 1, *s.commands[0].ExitCode)
		text := string(s.snapshot(true))
		require.Contains(t, text, "file missing")
		require.Contains(t, text, "/tmp/work dir")
		require.NotContains(t, text, "DRAFT_NOT_COLLECTED")
		require.NotContains(t, text, `\u001b`)
		require.NotContains(t, string(s.snapshot(false)), "file missing")
		require.Contains(t, string(s.snapshot(false)), "cat missing")
	}
}

func TestShellContext_UnknownHistoryBoundsAndExpiry(t *testing.T) {
	var s shellContext
	for i := 0; i < 12; i++ {
		s.observe("\x1b]633;A\a\x1b]633;B\a\x1b]633;C\a" + strings.Repeat("x", 10000) + "\x1b]633;D;0\a")
	}
	require.Len(t, s.commands, 8)
	require.Equal(t, "unknown", s.commands[0].Source)
	require.Empty(t, s.commands[0].Command)
	require.Len(t, s.commands[0].Output, 2048)
	require.True(t, s.commands[0].Truncated)
	var value struct {
		Commands []shellCommand `json:"recent_commands"`
	}
	require.NoError(t, json.Unmarshal(s.snapshot(true), &value))
	require.Len(t, value.Commands, 3)
	s.updated = time.Now().Add(-11 * time.Minute)
	require.JSONEq(t, `{"quality":"unavailable"}`, string(s.snapshot(true)))
	require.Empty(t, s.commands)
	s.observe("\x1b]" + strings.Repeat("x", 20000))
	require.LessOrEqual(t, len(s.sequence), 8192)
	s.observe("\a\x1b]633;B\a")
	require.Equal(t, "input", s.phase)
}

func TestShellContext_RedactionAndCancelledRead(t *testing.T) {
	for _, v := range []string{"password=supersecret", "--token supersecret", "https://user:supersecret@example.invalid", "-----BEGIN PRIVATE KEY-----supersecret"} {
		require.NotContains(t, cleanShellText(v), "supersecret")
	}
	h := &webSSHAgentHandle{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.ShellContext(ctx, true)
	require.ErrorIs(t, err, context.Canceled)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				h.observe("\x1b]633;B\a")
				_, e := h.ShellContext(context.Background(), true)
				require.NoError(t, e)
			}
		}()
	}
	wg.Wait()
}

func TestAssistanceHTTP_ShellContextConsentAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		mode    string
		owner   uint
		want    int
		context bool
		output  bool
	}{
		{"", 7, 200, false, false}, {"none", 7, 200, false, false}, {"commands", 7, 200, true, false}, {"output", 7, 200, true, true}, {"output", 8, 404, false, false}, {"all", 7, 400, false, false},
	} {
		t.Run(tc.mode+string(rune(tc.owner)), func(t *testing.T) {
			registry := accesssession.NewRegistry()
			h := &webSSHAgentHandle{}
			h.observe("\x1b]633;P;Cwd=/tmp/test\a\x1b]633;E;false\a\x1b]633;C\aOUTPUT_FIXTURE\x1b]633;D;1\a")
			_, cleanup, err := registry.Register(accesssession.Handle{Descriptor: accesssession.Descriptor{ID: "live", UserID: 7, AccessID: 2, Protocol: accesssession.ProtocolWebSSH}, Terminal: h})
			require.NoError(t, err)
			defer cleanup()
			service := &fakeAssistanceService{}
			web := &web{agentService: service, accessSessions: registry}
			body := `{"handle_id":"live","editor_id":"editor-123","revision":1,"text":"cat ","cursor":4,"context_mode":"` + tc.mode + `"}`
			r := httptest.NewRequest(http.MethodPost, "/api/v1/assistance/suggestions", strings.NewReader(body))
			r = r.WithContext(context.WithValue(r.Context(), "user", &model.User{Model: gorm.Model{ID: tc.owner}}))
			w := httptest.NewRecorder()
			web.handleAssistanceHTTP(w, r)
			require.Equal(t, tc.want, w.Code)
			if tc.want != 200 {
				require.Zero(t, service.calls)
				return
			}
			require.Equal(t, tc.context, service.lastInput.ShellContext != "")
			require.Equal(t, tc.output, strings.Contains(service.lastInput.ShellContext, "OUTPUT_FIXTURE"))
			if tc.context {
				require.Contains(t, service.lastInput.ShellContext, "/tmp/test")
			}
		})
	}
}
