package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceManager_LoadDisableEnableAndUnload(t *testing.T) {
	engine := NewEngine(nil, nil)
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	source := &staticSource{
		id:    "test",
		trust: TrustBuiltin,
		registrations: []ToolRegistration{
			testRegistration(descriptor, "ok"),
		},
	}
	manager := NewSourceManager(engine)
	require.NoError(t, manager.Load(context.Background(), source))

	status, _, _, exists := engine.Status(descriptor.ID)
	assert.True(t, exists)
	assert.Equal(t, ToolStatusActive, status)

	require.NoError(t, manager.Apply(context.Background(), source, ToolSourceEvent{Kind: SourceEventDisable, ToolID: descriptor.ID}))
	status, _, _, exists = engine.Status(descriptor.ID)
	assert.True(t, exists)
	assert.Equal(t, ToolStatusDisabled, status)

	require.NoError(t, manager.Apply(context.Background(), source, ToolSourceEvent{Kind: SourceEventEnable, ToolID: descriptor.ID}))
	status, _, _, exists = engine.Status(descriptor.ID)
	assert.True(t, exists)
	assert.Equal(t, ToolStatusActive, status)

	complete, err := manager.Unload(context.Background(), source.ID(), false)
	require.NoError(t, err)
	assert.True(t, complete)
	_, _, _, exists = engine.Status(descriptor.ID)
	assert.False(t, exists)
}

func TestSourceManager_RejectsSourceOwnershipMismatch(t *testing.T) {
	engine := NewEngine(nil, nil)
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	descriptor.Source.ID = "another-source"
	source := &staticSource{
		id:            "test",
		trust:         TrustBuiltin,
		registrations: []ToolRegistration{testRegistration(descriptor, "ok")},
	}

	err := NewSourceManager(engine).Load(context.Background(), source)
	assert.ErrorContains(t, err, "expected \"test\"")
}

func TestSourceManager_SourceCannotReplaceAnotherSourcesTool(t *testing.T) {
	engine := NewEngine(nil, nil)
	firstDescriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	firstDescriptor.Source = ToolSourceRef{ID: "first", Kind: "builtin", Trust: TrustBuiltin}
	first := &staticSource{id: "first", trust: TrustBuiltin, registrations: []ToolRegistration{testRegistration(firstDescriptor, "first")}}
	manager := NewSourceManager(engine)
	require.NoError(t, manager.Load(context.Background(), first))

	secondDescriptor := testDescriptor("ssh", "execute", "2.0.0", DisclosureAttachment)
	secondDescriptor.Source = ToolSourceRef{ID: "second", Kind: "builtin", Trust: TrustBuiltin}
	second := &staticSource{id: "second", trust: TrustBuiltin}
	require.NoError(t, manager.Load(context.Background(), second))
	err := manager.Apply(context.Background(), second, ToolSourceEvent{
		Kind:         SourceEventUpsert,
		Registration: testRegistration(secondDescriptor, "second"),
	})
	assert.ErrorContains(t, err, "cannot replace")
}

type staticSource struct {
	id            string
	trust         TrustLevel
	registrations []ToolRegistration
	events        <-chan ToolSourceEvent
}

func (source *staticSource) ID() string {
	return source.id
}

func (source *staticSource) TrustLevel() TrustLevel {
	return source.trust
}

func (source *staticSource) Snapshot(context.Context) ([]ToolRegistration, error) {
	return append([]ToolRegistration(nil), source.registrations...), nil
}

func (source *staticSource) Watch(context.Context) (<-chan ToolSourceEvent, error) {
	return source.events, nil
}
