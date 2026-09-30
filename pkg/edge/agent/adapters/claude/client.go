// Package claude implements the local Claude Code stream-json protocol.
// Session adapts its events and controls to the shared remote session bridge.
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

const maxFrame = 2 << 20

var (
	ErrClosed   = errors.New("claude protocol closed")
	ErrProtocol = errors.New("invalid claude protocol")
	ErrOverflow = errors.New("claude protocol capacity exceeded")
	ErrRejected = errors.New("claude rejected control request")
	ErrApproval = errors.New("claude approval is no longer pending")
)

// Event contains native data for the adapter only. Never forward Raw directly
// to a browser: system/result frames can contain local configuration or errors.
type Event struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type permission struct {
	Subtype string          `json:"subtype"`
	Tool    string          `json:"tool_name"`
	Input   json.RawMessage `json:"input"`
}

type response struct {
	Subtype   string          `json:"subtype"`
	RequestID string          `json:"request_id"`
	Response  json.RawMessage `json:"response,omitempty"`
}

// Client owns both pipes. The caller must consume Events continuously and Close
// it on cancellation. Unknown server controls terminate the transport, never grant.
type Client struct {
	expectedSession string
	in              io.WriteCloser
	out             io.ReadCloser
	mu              sync.Mutex
	write           sync.Mutex
	once            sync.Once
	sequence        atomic.Uint64
	pending         map[string]chan response
	approvals       map[string]permission
	events          chan Event
	done            chan struct{}
	readDone        chan struct{}
	err             error
	catalog         nativeCatalog
}

func NewClient(in io.WriteCloser, out io.ReadCloser) *Client {
	return newClient(in, out, "")
}

func newClient(in io.WriteCloser, out io.ReadCloser, expectedSession string) *Client {
	c := &Client{expectedSession: expectedSession, in: in, out: out, pending: make(map[string]chan response), approvals: make(map[string]permission), events: make(chan Event, 64), done: make(chan struct{}), readDone: make(chan struct{})}
	go c.read()
	return c
}

func (c *Client) Events() <-chan Event  { return c.events }
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) Err() error            { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *Client) Close() error          { c.fail(ErrClosed); <-c.readDone; return nil }

func (c *Client) fail(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		clear(c.approvals)
		c.mu.Unlock()
		close(c.done)
		// The first safe error is authoritative; close errors cannot recover a
		// broken protocol and must not expose process/provider diagnostics.
		if closeErr := c.in.Close(); closeErr != nil { /* already closing */
		}
		if closeErr := c.out.Close(); closeErr != nil { /* already closing */
		}
	})
}

func (c *Client) read() {
	defer close(c.readDone)
	defer close(c.events)
	defer func() {
		if recover() != nil {
			c.fail(ErrProtocol)
		}
	}()
	s := bufio.NewScanner(c.out)
	s.Buffer(make([]byte, 4096), maxFrame)
	for s.Scan() {
		var wire struct {
			Event
			SessionID string     `json:"session_id"`
			Request   permission `json:"request"`
			Response  response   `json:"response"`
		}
		if json.Unmarshal(s.Bytes(), &wire) != nil || wire.Type == "" {
			c.fail(ErrProtocol)
			return
		}
		if c.expectedSession != "" && ((wire.SessionID != "" && wire.SessionID != c.expectedSession) || (wire.Type == "result" && wire.SessionID == "")) {
			c.fail(ErrProtocol)
			return
		}
		switch wire.Type {
		case "control_response":
			if wire.Response.RequestID == "" || (wire.Response.Subtype != "success" && wire.Response.Subtype != "error") {
				c.fail(ErrProtocol)
				return
			}
			c.mu.Lock()
			waiting := c.pending[wire.Response.RequestID]
			delete(c.pending, wire.Response.RequestID)
			c.mu.Unlock()
			if waiting != nil {
				waiting <- wire.Response
			}
			continue
		case "control_request":
			if wire.RequestID == "" || len(wire.RequestID) > 256 || wire.Request.Subtype != "can_use_tool" || wire.Request.Tool == "" || len(wire.Request.Input) == 0 || wire.Request.Input[0] != '{' {
				c.fail(ErrProtocol)
				return
			}
			c.mu.Lock()
			_, duplicate := c.approvals[wire.RequestID]
			if duplicate || len(c.approvals) >= 32 {
				c.mu.Unlock()
				c.fail(ErrOverflow)
				return
			}
			c.approvals[wire.RequestID] = wire.Request
			c.mu.Unlock()
		case "control_cancel_request":
			if wire.RequestID == "" {
				c.fail(ErrProtocol)
				return
			}
			c.mu.Lock()
			delete(c.approvals, wire.RequestID)
			c.mu.Unlock()
		}
		wire.Event.Raw = append(json.RawMessage(nil), s.Bytes()...)
		select {
		case c.events <- wire.Event:
		case <-c.done:
			return
		default:
			c.fail(ErrOverflow)
			return
		}
	}
	if s.Err() != nil {
		c.fail(ErrProtocol)
	} else {
		c.fail(ErrClosed)
	}
}

func (c *Client) send(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ErrProtocol
	}
	if len(data)+1 >= maxFrame {
		return ErrOverflow
	}
	stop := context.AfterFunc(ctx, func() { c.fail(ctx.Err()) })
	defer stop()
	c.write.Lock()
	defer c.write.Unlock()
	select {
	case <-c.done:
		return ErrClosed
	default:
	}
	n, err := c.in.Write(append(data, '\n'))
	if err != nil || n != len(data)+1 {
		c.fail(ErrClosed)
		return ErrClosed
	}
	return nil
}

func (c *Client) control(ctx context.Context, request any) (json.RawMessage, error) {
	id := "liaison-" + strconv.FormatUint(c.sequence.Add(1), 10)
	waiting := make(chan response, 1)
	c.mu.Lock()
	if len(c.pending) >= 32 {
		c.mu.Unlock()
		return nil, ErrOverflow
	}
	c.pending[id] = waiting
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	if err := c.send(ctx, struct {
		Type    string `json:"type"`
		ID      string `json:"request_id"`
		Request any    `json:"request"`
	}{"control_request", id, request}); err != nil {
		return nil, err
	}
	select {
	case r := <-waiting:
		if r.Subtype != "success" {
			return nil, ErrRejected
		}
		return r.Response, nil
	case <-ctx.Done():
		// A timed-out control may already have taken effect. Fail closed and
		// never replay it on a replacement process.
		c.fail(ctx.Err())
		return nil, ctx.Err()
	case <-c.done:
		return nil, c.Err()
	}
}

func (c *Client) Initialize(ctx context.Context) error {
	raw, err := c.control(ctx, struct {
		Subtype string            `json:"subtype"`
		Hooks   map[string]string `json:"hooks"`
	}{"initialize", map[string]string{}})
	if err != nil {
		return err
	}
	var catalog nativeCatalog
	if len(raw) > 0 && json.Unmarshal(raw, &catalog) != nil {
		return ErrProtocol
	}
	c.mu.Lock()
	c.catalog = catalog
	c.mu.Unlock()
	return nil
}

func (c *Client) Interrupt(ctx context.Context) error {
	_, err := c.control(ctx, struct {
		Subtype string `json:"subtype"`
	}{"interrupt"})
	return err
}

func (c *Client) Send(ctx context.Context, prompt string) error {
	if prompt == "" {
		return ErrProtocol
	}
	return c.send(ctx, struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Parent  *string `json:"parent_tool_use_id"`
		Session string  `json:"session_id"`
	}{Type: "user", Message: struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{"user", prompt}})
}

// Decide is one-shot and replies with exactly the locally retained tool input.
// It does not support persistent permission updates or browser-supplied inputs.
func (c *Client) Decide(ctx context.Context, requestID string, allow bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	p, ok := c.approvals[requestID]
	// 提问不是普通授权：没有用户答案时不能以“允许”继续。
	if ok && allow && p.Tool == "AskUserQuestion" {
		c.mu.Unlock()
		return ErrAnswer
	}
	delete(c.approvals, requestID)
	c.mu.Unlock()
	if !ok {
		return ErrApproval
	}
	decision := struct {
		Behavior string          `json:"behavior"`
		Input    json.RawMessage `json:"updatedInput,omitempty"`
		Message  string          `json:"message,omitempty"`
	}{Behavior: "deny", Message: "Not authorized by user"}
	if allow {
		decision.Behavior = "allow"
		decision.Input = p.Input
		decision.Message = ""
	}
	return c.respond(ctx, requestID, decision)
}

func (c *Client) respond(ctx context.Context, requestID string, decision any) error {
	return c.send(ctx, struct {
		Type     string `json:"type"`
		Response struct {
			Subtype  string `json:"subtype"`
			ID       string `json:"request_id"`
			Decision any    `json:"response"`
		} `json:"response"`
	}{"control_response", struct {
		Subtype  string `json:"subtype"`
		ID       string `json:"request_id"`
		Decision any    `json:"response"`
	}{"success", requestID, decision}})
}
