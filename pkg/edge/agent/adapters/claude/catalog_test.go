package claude

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeCatalogUsesOnlyPublicFields(t *testing.T) {
	f := newFixture(t)
	done := make(chan error, 1)
	go func() { done <- f.client.Initialize(t.Context()) }()
	r := f.request(t)
	require.NotContains(t, string(r["request"]), `"skills":[]`)
	f.emit(t, `{"type":"control_response","response":{"subtype":"success","request_id":`+string(r["request_id"])+`,"response":{"models":[{"value":"provider-model","displayName":"Local model"},{"value":"bad\nmodel"}],"commands":[{"name":"review","description":"Review changes"},{"name":"../../private","description":"invalid"}],"account":{"token":"do-not-forward"}}}}`)
	require.NoError(t, <-done)
	require.Len(t, f.client.models(), 1)
	require.Len(t, f.client.skills(), 1)
	require.Equal(t, "/review", f.client.skills()[0].Name)
	require.Len(t, f.client.skills()[0].ID, 32)
}

func TestModelAndSkillDispatchUsesNativeControl(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, json.Unmarshal([]byte(`{"commands":[{"name":"review","description":"Review"}]}`), &f.client.catalog))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := &Session{ctx: ctx, cancel: cancel, thread: "owned", driver: &Driver{Client: f.client}}
	done := make(chan error, 1)
	go func() {
		_, err := s.SendModel(ctx, "owned", "Check changes", f.client.skills()[0].ID, "provider-model")
		done <- err
	}()
	control := f.request(t)
	require.JSONEq(t, `{"subtype":"set_model","model":"provider-model"}`, string(control["request"]))
	f.emit(t, `{"type":"control_response","response":{"subtype":"success","request_id":`+string(control["request_id"])+`,"response":{}}}`)
	msg := f.request(t)
	require.True(t, strings.Contains(string(msg["message"]), "/review Check changes"))
	require.NoError(t, <-done)
	_, err := s.Send(ctx, "owned", "concurrent turn")
	require.Error(t, err)
}
