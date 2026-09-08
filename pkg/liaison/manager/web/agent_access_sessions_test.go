package web

import (
	"context"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterWebDataAgentSession_BindsIdentityAndGeneration(t *testing.T) {
	registry := accesssession.NewRegistry()
	webServer := &web{accessSessions: registry}
	session := &webDataSession{
		token: "data-session", userID: 7, proxyID: 11, protocol: "mysql",
		target: &controlplane.WebDataTarget{ApplicationID: 13},
	}
	require.NoError(t, webServer.registerWebDataAgentSession(session))
	t.Cleanup(session.close)

	handle, err := registry.Resolve(context.Background(), accesssession.ResolveRequest{
		ID: session.token, UserID: session.userID, AccessID: session.proxyID,
		ApplicationID: session.target.ApplicationID, Protocol: accesssession.ProtocolMySQL,
		Generation: session.agentGeneration,
	})
	require.NoError(t, err)
	assert.NotNil(t, handle.Data)
	assert.Nil(t, handle.Terminal)

	session.close()
	_, err = registry.Resolve(context.Background(), accesssession.ResolveRequest{
		ID: session.token, UserID: session.userID, AccessID: session.proxyID,
		ApplicationID: session.target.ApplicationID, Protocol: accesssession.ProtocolMySQL,
		Generation: session.agentGeneration,
	})
	assert.ErrorIs(t, err, accesssession.ErrHandleNotFound)
}

func TestBoundedOutput_TruncatesWithoutShortWrite(t *testing.T) {
	output := &boundedOutput{limit: 5}
	written, err := output.Write([]byte("12345678"))
	require.NoError(t, err)
	assert.Equal(t, 8, written)
	assert.Equal(t, "12345", output.String())
	assert.True(t, output.Truncated())
}

func TestWebSSHAgentHandle_ReadReturnsRecentLines(t *testing.T) {
	handle := &webSSHAgentHandle{}
	handle.observe(strings.Join([]string{"one", "two", "three"}, "\n"))
	content, err := handle.Read(context.Background(), 2)
	require.NoError(t, err)
	assert.JSONEq(t, `{"output":"two\nthree","truncated":true}`, string(content))
}
