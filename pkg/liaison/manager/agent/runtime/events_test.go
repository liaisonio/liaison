package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBroker_IsolatesSessionsAndClonesPayload(t *testing.T) {
	broker := NewEventBroker(2)
	subscription, err := broker.Subscribe(context.Background(), "session-a")
	require.NoError(t, err)
	defer subscription.Close()

	require.NoError(t, broker.Publish(context.Background(), Event{SessionID: "session-b", Type: EventTurnStarted}))
	payload := []byte(`{"delta":"hello"}`)
	require.NoError(t, broker.Publish(context.Background(), Event{SessionID: "session-a", Type: EventModelDelta, Payload: payload}))
	payload[0] = 'x'

	select {
	case event := <-subscription.Events:
		assert.NotZero(t, event.Sequence)
		assert.JSONEq(t, `{"delta":"hello"}`, string(event.Payload))
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for agent event")
	}
}

func TestEventBroker_DropsSlowSubscriberWithoutBlockingPublish(t *testing.T) {
	broker := NewEventBroker(1)
	subscription, err := broker.Subscribe(context.Background(), "session")
	require.NoError(t, err)
	require.NoError(t, broker.Publish(context.Background(), Event{SessionID: "session", Type: EventModelDelta}))
	require.NoError(t, broker.Publish(context.Background(), Event{SessionID: "session", Type: EventModelDelta}))

	select {
	case err := <-subscription.Done:
		assert.ErrorIs(t, err, ErrEventSubscriberSlow)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for slow subscriber closure")
	}
}

func TestEventBroker_ContextCancellationClosesSubscription(t *testing.T) {
	broker := NewEventBroker(1)
	ctx, cancel := context.WithCancel(context.Background())
	subscription, err := broker.Subscribe(ctx, "session")
	require.NoError(t, err)
	cancel()

	select {
	case err, ok := <-subscription.Done:
		assert.False(t, ok)
		assert.True(t, err == nil || errors.Is(err, context.Canceled))
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription closure")
	}
}
