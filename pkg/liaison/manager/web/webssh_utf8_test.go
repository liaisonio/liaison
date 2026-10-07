package web

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestWebSSHDecodeOutput_AllSplitBoundaries(t *testing.T) {
	want := "ASCII\r\n中文目录：你好 世界 🌏\x1b[32m绿色\x1b[0m"
	for split := 0; split <= len(want); split++ {
		first, rest := webSSHDecodeOutput([]byte(want[:split]), false)
		next, rest := webSSHDecodeOutput(append(rest, []byte(want[split:])...), true)
		if first+next != want || len(rest) != 0 || !utf8.ValidString(first) {
			t.Fatalf("split %d: got %q + %q, rest %x", split, first, next, rest)
		}
		wire, err := json.Marshal(first + next)
		if err != nil {
			t.Fatal(err)
		}
		var got string
		if err := json.Unmarshal(wire, &got); err != nil || got != want {
			t.Fatalf("JSON roundtrip: %q, %v", got, err)
		}
	}
}

func TestWebSSHDecodeOutput_BytewiseAndIncompleteEOF(t *testing.T) {
	var pending []byte
	var got strings.Builder
	for _, b := range []byte("测试中文😀") {
		pending = append(pending, b)
		text, rest := webSSHDecodeOutput(pending, false)
		got.WriteString(text)
		pending = append(pending[:0], rest...)
	}
	if got.String() != "测试中文😀" || len(pending) != 0 {
		t.Fatalf("got %q, pending %x", got.String(), pending)
	}
	text, rest := webSSHDecodeOutput([]byte{0xe4, 0xb8}, false)
	if text != "" || len(rest) != 2 {
		t.Fatal("partial character was emitted")
	}
	text, rest = webSSHDecodeOutput(rest, true)
	if text != "\uFFFD" || len(rest) != 0 {
		t.Fatal("incomplete EOF was not replaced")
	}
	text, rest = webSSHDecodeOutput([]byte{0xff, 'a'}, false)
	if text != "\uFFFDa" || len(rest) != 0 {
		t.Fatal("invalid input did not progress")
	}
}

// Force each read across the timer flush boundary, as slow SSH output can do.
func TestWebSSHOutput_UTF8AcrossTimedFlushes(t *testing.T) {
	want := "连接成功：中文😀\r\n"
	completed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(completed)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		reader := &webSSHSlowBytes{data: []byte(want)}
		(&web{}).copyWebSSHOutput(&webSSHWSWriter{conn: conn}, reader, make(chan struct{}), nil)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for {
		var message webSSHServerMessage
		if err := conn.ReadJSON(&message); err != nil {
			break
		}
		output.WriteString(message.Data)
	}
	<-completed
	if output.String() != want {
		t.Fatalf("got %q; want %q", output.String(), want)
	}
}

type webSSHSlowBytes struct{ data []byte }

func (r *webSSHSlowBytes) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	time.Sleep(2 * webSSHOutputFlush)
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}
