package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentBinder_UsesOnlyServerAuthoredFacts(t *testing.T) {
	registry := accesssession.NewRegistry()
	descriptor, unregister, err := registry.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{ID: "opaque", UserID: 7, AccessID: 11, ApplicationID: 13,
			Protocol: accesssession.ProtocolMySQL, Capabilities: []string{"data.schema", "data.query"}},
		Data: bindingData{},
	})
	require.NoError(t, err)
	t.Cleanup(unregister)
	binder, err := NewAttachmentBinder(registry)
	require.NoError(t, err)

	attachment, err := binder.Bind(context.Background(), tool.Principal{UserID: 7, OrganizationID: 3}, descriptor.ID)
	require.NoError(t, err)
	assert.Equal(t, descriptor.AccessID, attachment.AccessID)
	assert.Equal(t, descriptor.ApplicationID, attachment.ApplicationID)
	assert.Equal(t, tool.ProtocolMySQL, attachment.Protocol)
	assert.Equal(t, descriptor.Generation, attachment.Generation)
	assert.Equal(t, []tool.Capability{"data.schema", "data.query"}, attachment.Capabilities)
}

type bindingData struct{}

func (bindingData) Schema(context.Context, []string) (json.RawMessage, error) {
	return json.RawMessage(`{"nodes":[]}`), nil
}

func (bindingData) Query(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{"rows":[]}`), nil
}
