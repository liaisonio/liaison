// Package rpc provides bounded bidirectional JSON-line RPC for Agent adapters.
package rpc

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

var ErrClosed = errors.New("agent protocol closed")
var ErrOverflow = errors.New("agent event queue overflow")

const maxFrame = 2 << 20

type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type Client struct {
	in         io.WriteCloser
	out        io.ReadCloser
	write      sync.Mutex
	mu         sync.Mutex
	pending    map[string]chan Message
	sequence   atomic.Uint64
	events     chan Message
	done       chan struct{}
	readerDone chan struct{}
	once       sync.Once
	err        error
}

func New(in io.WriteCloser, out io.ReadCloser) *Client {
	c := &Client{in: in, out: out, pending: map[string]chan Message{}, events: make(chan Message, 64), done: make(chan struct{}), readerDone: make(chan struct{})}
	go c.read()
	return c
}
func (c *Client) Events() <-chan Message { return c.events }
func (c *Client) Done() <-chan struct{}  { return c.done }
func (c *Client) Err() error             { c.mu.Lock(); defer c.mu.Unlock(); return c.err }

func (c *Client) fail(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.done)
		// Closing pipes unblocks readers/writers. The first protocol error is kept;
		// pipe-close errors after process exit have no actionable recovery.
		if closeErr := c.in.Close(); closeErr != nil && err == nil {
			c.mu.Lock()
			c.err = closeErr
			c.mu.Unlock()
		}
		if closeErr := c.out.Close(); closeErr != nil && err == nil {
			c.mu.Lock()
			c.err = closeErr
			c.mu.Unlock()
		}
	})
}
func (c *Client) Close() error { c.fail(ErrClosed); <-c.readerDone; return nil }

func (c *Client) read() {
	defer close(c.readerDone)
	defer close(c.events)
	defer func() {
		if recover() != nil {
			c.fail(errors.New("agent protocol reader failed"))
		}
	}()
	scanner := bufio.NewScanner(c.out)
	scanner.Buffer(make([]byte, 4096), maxFrame)
	for scanner.Scan() {
		var msg Message
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			c.fail(errors.New("invalid agent protocol frame"))
			return
		}
		if msg.Method == "" && len(msg.ID) > 0 {
			c.mu.Lock()
			waiting := c.pending[string(msg.ID)]
			c.mu.Unlock()
			if waiting != nil {
				select {
				case waiting <- msg:
				default:
				}
			}
			continue
		}
		if msg.Method == "" {
			c.fail(errors.New("invalid agent notification"))
			return
		}
		select {
		case c.events <- msg:
		case <-c.done:
			return
		default:
			c.fail(ErrOverflow)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		c.fail(errors.New("agent protocol read failed"))
	} else {
		c.fail(ErrClosed)
	}
}

func (c *Client) send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(data) > maxFrame {
		return errors.New("agent request too large")
	}
	// Cancelling a blocked pipe write must close the protocol, not leak a writer.
	stop := context.AfterFunc(ctx, func() { c.fail(ctx.Err()) })
	defer stop()
	c.write.Lock()
	defer c.write.Unlock()
	select {
	case <-c.done:
		return ErrClosed
	default:
	}
	var n int
	n, err = c.in.Write(append(data, '\n'))
	if err == nil && n != len(data)+1 {
		err = io.ErrShortWrite
	}
	if err != nil {
		c.fail(ErrClosed)
		return ErrClosed
	}
	return nil
}

func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := strconv.FormatUint(c.sequence.Add(1), 10)
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	waiting := make(chan Message, 1)
	c.mu.Lock()
	if len(c.pending) >= 32 {
		c.mu.Unlock()
		return nil, errors.New("too many pending agent requests")
	}
	c.pending[id] = waiting
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	if err = c.send(ctx, Message{ID: json.RawMessage(id), Method: method, Params: data}); err != nil {
		return nil, err
	}
	select {
	case response := <-waiting:
		if len(response.Error) > 0 && string(response.Error) != "null" {
			return nil, errors.New("agent rejected request")
		}
		return response.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClosed
	}
}
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.send(ctx, Message{Method: method, Params: data})
}
func (c *Client) Respond(ctx context.Context, id json.RawMessage, result any) error {
	if len(id) == 0 || !json.Valid(id) {
		return errors.New("invalid approval identifier")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.send(ctx, Message{ID: id, Result: data})
}
