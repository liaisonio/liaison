package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type EventType string

const (
	EventTurnStarted      EventType = "turn.started"
	EventModelDelta       EventType = "model.delta"
	EventToolStarted      EventType = "tool.started"
	EventToolCompleted    EventType = "tool.completed"
	EventApprovalRequired EventType = "approval.required"
	EventApprovalResolved EventType = "approval.resolved"
	EventTurnCompleted    EventType = "turn.completed"
	EventTurnCancelled    EventType = "turn.cancelled"
	EventTurnFailed       EventType = "turn.failed"
)

type Event struct {
	Sequence  uint64          `json:"sequence"`
	SessionID string          `json:"session_id"`
	TurnID    string          `json:"turn_id,omitempty"`
	StepID    string          `json:"step_id,omitempty"`
	Type      EventType       `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Occurred  time.Time       `json:"occurred_at"`
}

type EventSink interface {
	Publish(ctx context.Context, event Event) error
}

type EventSinkFunc func(context.Context, Event) error

func (fn EventSinkFunc) Publish(ctx context.Context, event Event) error {
	return fn(ctx, event)
}

type discardEventSink struct{}

func (discardEventSink) Publish(context.Context, Event) error { return nil }

var ErrEventSubscriberSlow = errors.New("agent event subscriber is too slow")

type EventSubscription struct {
	Events <-chan Event
	Done   <-chan error
	cancel func()
}

func (subscription *EventSubscription) Close() {
	if subscription != nil && subscription.cancel != nil {
		subscription.cancel()
	}
}

// EventBroker fans runtime events out to SSE adapters without allowing a slow
// browser to block a Turn. It is intentionally ephemeral: reconnecting clients
// restore authoritative state from Store, then subscribe for new deltas.
type EventBroker struct {
	mu          sync.Mutex
	bufferSize  int
	nextID      uint64
	subscribers map[uint64]*eventSubscriber
}

type eventSubscriber struct {
	sessionID string
	events    chan Event
	done      chan error
	closed    chan struct{}
}

func NewEventBroker(bufferSize int) *EventBroker {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &EventBroker{bufferSize: bufferSize, subscribers: make(map[uint64]*eventSubscriber)}
}

func (broker *EventBroker) Publish(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.SessionID == "" || event.Type == "" {
		return errors.New("agent event session and type are required")
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.nextID++
	event.Sequence = broker.nextID
	event.Payload = append(json.RawMessage(nil), event.Payload...)
	for id, subscriber := range broker.subscribers {
		if subscriber.sessionID != event.SessionID {
			continue
		}
		select {
		case subscriber.events <- event:
		default:
			broker.removeSubscriberLocked(id, ErrEventSubscriberSlow)
		}
	}
	return nil
}

func (broker *EventBroker) Subscribe(ctx context.Context, sessionID string) (*EventSubscription, error) {
	if sessionID == "" {
		return nil, errors.New("agent event subscription session is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	broker.mu.Lock()
	broker.nextID++
	id := broker.nextID
	subscriber := &eventSubscriber{
		sessionID: sessionID,
		events:    make(chan Event, broker.bufferSize),
		done:      make(chan error, 1),
		closed:    make(chan struct{}),
	}
	broker.subscribers[id] = subscriber
	broker.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			broker.removeSubscriber(id, nil)
		})
	}
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-subscriber.closed:
		}
	}()
	return &EventSubscription{Events: subscriber.events, Done: subscriber.done, cancel: cancel}, nil
}

func (broker *EventBroker) removeSubscriber(id uint64, reason error) {
	broker.mu.Lock()
	broker.removeSubscriberLocked(id, reason)
	broker.mu.Unlock()
}

func (broker *EventBroker) removeSubscriberLocked(id uint64, reason error) {
	subscriber, ok := broker.subscribers[id]
	if !ok {
		return
	}
	delete(broker.subscribers, id)
	close(subscriber.events)
	if reason != nil {
		subscriber.done <- reason
	}
	close(subscriber.done)
	close(subscriber.closed)
}
