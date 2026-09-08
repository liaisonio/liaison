package accesssession

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RejectsStaleOrCrossUserAttachment(t *testing.T) {
	registry := NewRegistry()
	descriptor, unregister, err := registry.Register(Handle{
		Descriptor: Descriptor{ID: "session-1", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: ProtocolMySQL},
		Data:       stubData{},
	})
	require.NoError(t, err)
	t.Cleanup(unregister)

	request := ResolveRequest{ID: descriptor.ID, UserID: descriptor.UserID, AccessID: descriptor.AccessID,
		ApplicationID: descriptor.ApplicationID, Protocol: descriptor.Protocol, Generation: descriptor.Generation}
	_, err = registry.Resolve(context.Background(), request)
	require.NoError(t, err)

	request.UserID++
	_, err = registry.Resolve(context.Background(), request)
	assert.ErrorIs(t, err, ErrHandleMismatch)

	request.UserID = descriptor.UserID
	request.Generation++
	_, err = registry.Resolve(context.Background(), request)
	assert.ErrorIs(t, err, ErrHandleMismatch)

	_, err = registry.Describe(context.Background(), descriptor.ID, 8)
	assert.ErrorIs(t, err, ErrHandleMismatch)
	described, err := registry.Describe(context.Background(), descriptor.ID, 7)
	require.NoError(t, err)
	assert.Equal(t, descriptor, described)
}

func TestRegistry_OldUnregisterCannotRemoveReplacement(t *testing.T) {
	registry := NewRegistry()
	first, unregisterFirst, err := registry.Register(Handle{
		Descriptor: Descriptor{ID: "session-1", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: ProtocolMySQL}, Data: stubData{},
	})
	require.NoError(t, err)
	second, unregisterSecond, err := registry.Register(Handle{
		Descriptor: Descriptor{ID: "session-1", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: ProtocolMySQL}, Data: stubData{},
	})
	require.NoError(t, err)
	t.Cleanup(unregisterSecond)
	assert.Greater(t, second.Generation, first.Generation)

	unregisterFirst()
	_, err = registry.Resolve(context.Background(), ResolveRequest{ID: second.ID, UserID: second.UserID, AccessID: second.AccessID,
		ApplicationID: second.ApplicationID, Protocol: second.Protocol, Generation: second.Generation})
	require.NoError(t, err)
}

type stubData struct{}

func (stubData) Schema(context.Context, []string) (json.RawMessage, error) {
	return json.RawMessage(`{"nodes":[]}`), nil
}

func (stubData) Query(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{"rows":[]}`), nil
}
