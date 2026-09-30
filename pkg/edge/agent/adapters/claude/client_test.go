package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	client   *Client
	requests chan map[string]json.RawMessage
	output   *io.PipeWriter
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	f := &fixture{NewClient(inW, outR), make(chan map[string]json.RawMessage, 64), outW}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(f.requests)
		s := bufio.NewScanner(inR)
		for s.Scan() {
			var v map[string]json.RawMessage
			if json.Unmarshal(s.Bytes(), &v) != nil {
				return
			}
			select {
			case f.requests <- v:
			case <-f.client.Done():
				return
			}
		}
	}()
	t.Cleanup(func() {
		if err := f.client.Close(); err != nil {
			t.Error(err)
		}
		if err := inR.Close(); err != nil {
			t.Error(err)
		}
		if err := outW.Close(); err != nil {
			t.Error(err)
		}
		<-done
	})
	return f
}

func (f *fixture) emit(t *testing.T, line string) {
	t.Helper()
	if _, err := io.WriteString(f.output, line+"\n"); err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) request(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	select {
	case v := <-f.requests:
		return v
	case <-time.After(time.Second):
		t.Fatal("missing outbound request")
		return nil
	}
}
func nextEvent(t *testing.T, c *Client) Event {
	t.Helper()
	select {
	case e, ok := <-c.Events():
		if !ok {
			t.Fatalf("closed: %v", c.Err())
		}
		return e
	case <-time.After(time.Second):
		t.Fatal("missing event")
		return Event{}
	}
}
func awaitClosed(t *testing.T, c *Client) {
	t.Helper()
	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("protocol did not close")
	}
}

func TestInitializeAndInterrupt(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct {
		name string
		call func(context.Context) error
	}{{"initialize", f.client.Initialize}, {"interrupt", f.client.Interrupt}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- tc.call(ctx) }()
			r := f.request(t)
			if string(r["type"]) != `"control_request"` {
				t.Fatal("wrong envelope")
			}
			var req struct {
				Subtype string `json:"subtype"`
			}
			if err := json.Unmarshal(r["request"], &req); err != nil || req.Subtype != tc.name {
				t.Fatal("wrong request")
			}
			f.emit(t, `{"type":"stream_event","event":{"type":"message_start"}}`)
			f.emit(t, `{"type":"control_response","response":{"subtype":"success","request_id":`+string(r["request_id"])+`,"response":{}}}`)
			if err := <-result; err != nil {
				t.Fatal(err)
			}
			if nextEvent(t, f.client).Type != "stream_event" {
				t.Fatal("stream lost")
			}
		})
	}
}

func TestPermissionOneShot(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(map[bool]string{false: "deny", true: "allow"}[allow], func(t *testing.T) {
			f := newFixture(t)
			f.emit(t, `{"type":"control_request","request_id":"p1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"pwd"}}}`)
			e := nextEvent(t, f.client)
			if e.RequestID != "p1" {
				t.Fatal("missing approval")
			}
			if err := f.client.Decide(t.Context(), "p1", allow); err != nil {
				t.Fatal(err)
			}
			r := f.request(t)
			var response struct {
				Subtype  string `json:"subtype"`
				ID       string `json:"request_id"`
				Decision struct {
					Behavior string          `json:"behavior"`
					Input    json.RawMessage `json:"updatedInput"`
				} `json:"response"`
			}
			if err := json.Unmarshal(r["response"], &response); err != nil {
				t.Fatal(err)
			}
			want := "deny"
			if allow {
				want = "allow"
			}
			if response.ID != "p1" || response.Subtype != "success" || response.Decision.Behavior != want {
				t.Fatal("wrong permission response")
			}
			if allow && string(response.Decision.Input) != `{"command":"pwd"}` {
				t.Fatal("tool input changed")
			}
			if err := f.client.Decide(t.Context(), "p1", allow); !errors.Is(err, ErrApproval) {
				t.Fatal("approval replay allowed")
			}
		})
	}
}

func TestCancelledPermission(t *testing.T) {
	f := newFixture(t)
	f.emit(t, `{"type":"control_request","request_id":"p1","request":{"subtype":"can_use_tool","tool_name":"Read","input":{"file_path":"test.txt"}}}`)
	nextEvent(t, f.client)
	f.emit(t, `{"type":"control_cancel_request","request_id":"p1"}`)
	nextEvent(t, f.client)
	if err := f.client.Decide(t.Context(), "p1", true); !errors.Is(err, ErrApproval) {
		t.Fatal("cancelled approval accepted")
	}
}

func TestBadFramesFailClosed(t *testing.T) {
	for name, line := range map[string]string{
		"invalid": "not json", "empty": "{}", "null": "null",
		"unknown control":     `{"type":"control_request","request_id":"p","request":{"subtype":"new_permission"}}`,
		"missing input":       `{"type":"control_request","request_id":"p","request":{"subtype":"can_use_tool","tool_name":"Bash"}}`,
		"missing response id": `{"type":"control_response","response":{"subtype":"success"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.emit(t, line)
			awaitClosed(t, f.client)
			if !errors.Is(f.client.Err(), ErrProtocol) {
				t.Fatal(f.client.Err())
			}
		})
	}
}

func TestOverflow(t *testing.T) {
	f := newFixture(t)
	for n := 0; n < 65; n++ {
		f.emit(t, `{"type":"assistant","message":{"content":[]}}`)
	}
	awaitClosed(t, f.client)
	if !errors.Is(f.client.Err(), ErrOverflow) {
		t.Fatal(f.client.Err())
	}
}

func TestSafeControlError(t *testing.T) {
	f := newFixture(t)
	result := make(chan error, 1)
	go func() { result <- f.client.Initialize(t.Context()) }()
	r := f.request(t)
	f.emit(t, `{"type":"control_response","response":{"subtype":"error","request_id":`+string(r["request_id"])+`,"error":"provider-secret-not-for-browser"}}`)
	if err := <-result; !errors.Is(err, ErrRejected) {
		t.Fatal("raw error leaked or error missing")
	}
}

func TestControlTimeoutCloses(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- f.client.Initialize(ctx) }()
	f.request(t)
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	awaitClosed(t, f.client)
}

func TestSendAndFrameLimit(t *testing.T) {
	f := newFixture(t)
	if err := f.client.Send(t.Context(), "hello\nworld"); err != nil {
		t.Fatal(err)
	}
	r := f.request(t)
	if string(r["type"]) != `"user"` || string(r["message"]) != `{"role":"user","content":"hello\nworld"}` {
		t.Fatal("wrong user message")
	}
	if err := f.client.Send(t.Context(), strings.Repeat("x", maxFrame)); !errors.Is(err, ErrOverflow) {
		t.Fatal("oversized prompt accepted")
	}
}

func TestBlockedWriteCancellation(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := NewClient(inW, outR)
	defer func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
		if err := inR.Close(); err != nil {
			t.Error(err)
		}
		if err := outW.Close(); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := c.Send(ctx, "blocked"); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	awaitClosed(t, c)
}

func TestApprovalCapacityAndDuplicates(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "capacity", true: "duplicate"}[duplicate], func(t *testing.T) {
			f := newFixture(t)
			count := 33
			if duplicate {
				count = 2
			}
			for i := 0; i < count; i++ {
				id := strconv.Itoa(i)
				if duplicate {
					id = "same"
				}
				f.emit(t, `{"type":"control_request","request_id":"`+id+`","request":{"subtype":"can_use_tool","tool_name":"Read","input":{}}}`)
				if i < count-1 {
					nextEvent(t, f.client)
				}
			}
			awaitClosed(t, f.client)
			if !errors.Is(f.client.Err(), ErrOverflow) {
				t.Fatal(f.client.Err())
			}
			if err := f.client.Decide(t.Context(), "0", true); !errors.Is(err, ErrApproval) {
				t.Fatal("closed transport retained approval")
			}
		})
	}
}

func TestOversizedInboundFrame(t *testing.T) {
	f := newFixture(t)
	// Scanner intentionally closes before consuming the whole oversized write.
	if _, err := io.WriteString(f.output, strings.Repeat("x", maxFrame+1)+"\n"); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	awaitClosed(t, f.client)
	if !errors.Is(f.client.Err(), ErrProtocol) {
		t.Fatal(f.client.Err())
	}
}

func TestPendingControlsBounded(t *testing.T) {
	f := newFixture(t)
	f.client.mu.Lock()
	for i := 0; i < 32; i++ {
		f.client.pending["held-"+strconv.Itoa(i)] = make(chan response, 1)
	}
	f.client.mu.Unlock()
	if err := f.client.Initialize(t.Context()); !errors.Is(err, ErrOverflow) {
		t.Fatal("unbounded pending controls")
	}
}

func TestCloseReleasesControlWaiter(t *testing.T) {
	f := newFixture(t)
	result := make(chan error, 1)
	go func() { result <- f.client.Initialize(t.Context()) }()
	f.request(t)
	if err := f.client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, ErrClosed) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("control waiter leaked")
	}
}
