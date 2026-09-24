package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func pipes(t *testing.T) (*Client, *io.PipeReader, *io.PipeWriter) {
	t.Helper()
	input, writer := io.Pipe()
	reader, output := io.Pipe()
	client := New(writer, reader)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
		if err := input.Close(); err != nil {
			t.Error(err)
		}
		if err := output.Close(); err != nil {
			t.Error(err)
		}
	})
	return client, input, output
}
func TestResponseAndEvent(t *testing.T) {
	c, in, out := pipes(t)
	serverDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(in)
		if !scanner.Scan() {
			serverDone <- io.EOF
			return
		}
		var req Message
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			serverDone <- err
			return
		}
		data, err := json.Marshal(Message{ID: req.ID, Result: json.RawMessage(`{"ok":true}`)})
		if err != nil {
			serverDone <- err
			return
		}
		_, err = out.Write(append(data, '\n'))
		if err == nil {
			_, err = out.Write([]byte("{\"method\":\"progress\",\"params\":{}}\n"))
		}
		serverDone <- err
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := c.Call(ctx, "test", map[string]string{})
	if err != nil || string(result) != `{"ok":true}` {
		t.Fatalf("response %s %v", result, err)
	}
	select {
	case event := <-c.Events():
		if event.Method != "progress" {
			t.Fatal(event)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}
func TestCancellationUnblocksWriter(t *testing.T) {
	c, _, _ := pipes(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Call(ctx, "blocked", nil); err == nil {
		t.Fatal("blocked call succeeded")
	}
	if time.Since(start) > time.Second {
		t.Fatal("writer leaked")
	}
}
func TestInvalidFrameFailsClosed(t *testing.T) {
	c, _, out := pipes(t)
	if _, err := out.Write([]byte("not-json\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.Done():
		if c.Err() == nil {
			t.Fatal("missing failure")
		}
	case <-time.After(time.Second):
		t.Fatal("not closed")
	}
}
func TestOverflowFailsClosed(t *testing.T) {
	c, _, out := pipes(t)
	for i := 0; i < 65; i++ {
		if _, err := out.Write([]byte("{\"method\":\"event\"}\n")); err != nil {
			break
		}
	}
	select {
	case <-c.Done():
		if !errors.Is(c.Err(), ErrOverflow) {
			t.Fatal(c.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded queue")
	}
}

func TestOversizedFrameFailsClosed(t *testing.T) {
	c, _, out := pipes(t)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		// A closed-pipe error is expected when the reader rejects the frame.
		_, err := io.Copy(out, strings.NewReader(strings.Repeat("x", maxFrame+1)+"\n"))
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Error(err)
		}
	}()
	select {
	case <-c.Done():
		if c.Err() == nil {
			t.Fatal("oversized frame accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded frame read")
	}
	<-finished
}
