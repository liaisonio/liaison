package assistance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type generatorFunc func(context.Context, Binding, Input) (string, error)

func (f generatorFunc) Suggest(ctx context.Context, b Binding, i Input) (string, error) {
	return f(ctx, b, i)
}

type guardFunc func(context.Context, Binding) error

func (f guardFunc) Check(ctx context.Context, b Binding) error { return f(ctx, b) }

func newTestSession(t *testing.T, g generatorFunc) *Session {
	t.Helper()
	s, err := NewSession(Binding{OwnerID: 1, HandleID: "live-ssh", Protocol: "ssh"}, g, guardFunc(func(context.Context, Binding) error { return nil }))
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return s
}

func TestSuggest_RevisionAndControlCharacters(t *testing.T) {
	for _, text := range []string{"pwd\n", "\x1b[31m", "\r", "\t", "\x00"} {
		t.Run(text, func(t *testing.T) {
			s := newTestSession(t, func(context.Context, Binding, Input) (string, error) { return text, nil })
			_, err := s.Suggest(context.Background(), Input{Revision: 1})
			require.ErrorIs(t, err, ErrSuggestion)
		})
	}
	s := newTestSession(t, func(_ context.Context, b Binding, i Input) (string, error) {
		require.Equal(t, "live-ssh", b.HandleID)
		return "wd", nil
	})
	result, err := s.Suggest(context.Background(), Input{Revision: 1, Text: "p", Cursor: 1})
	require.NoError(t, err)
	require.Equal(t, Suggestion{Revision: 1, Cursor: 1, Text: "wd"}, result)
	_, err = s.Suggest(context.Background(), Input{Revision: 1})
	require.ErrorIs(t, err, ErrStale)
	s.Close()
	_, err = s.Suggest(context.Background(), Input{Revision: 2})
	require.ErrorIs(t, err, ErrClosed)
}

func TestSuggest_NewInputCancelsPrevious(t *testing.T) {
	started := make(chan struct{})
	s := newTestSession(t, func(ctx context.Context, _ Binding, i Input) (string, error) {
		if i.Revision == 1 {
			close(started)
			<-ctx.Done()
			// 模拟忽略取消并返回迟到候选的模型。
			return "obsolete", nil
		}
		return "current", nil
	})
	done := make(chan error, 1)
	go func() { _, err := s.Suggest(context.Background(), Input{Revision: 1}); done <- err }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("generator did not start")
	}
	result, err := s.Suggest(context.Background(), Input{Revision: 2})
	require.NoError(t, err)
	require.Equal(t, "current", result.Text)
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("request not cancelled")
	}
}

func TestSuggest_RevokedPermissionDiscardsResult(t *testing.T) {
	denied := errors.New("revoked")
	checks := 0
	s, err := NewSession(Binding{OwnerID: 1, HandleID: "db", Protocol: "mysql"}, generatorFunc(func(context.Context, Binding, Input) (string, error) {
		return "SELECT 1", nil
	}), guardFunc(func(context.Context, Binding) error {
		checks++
		if checks == 2 {
			return denied
		}
		return nil
	}))
	require.NoError(t, err)
	t.Cleanup(s.Close)
	result, err := s.Suggest(context.Background(), Input{Revision: 1})
	require.ErrorIs(t, err, denied)
	require.Empty(t, result.Text)
}

func TestSuggest_InvalidCursorNeverCallsModel(t *testing.T) {
	s := newTestSession(t, func(context.Context, Binding, Input) (string, error) {
		t.Fatal("unexpected model call")
		return "", nil
	})
	for _, input := range []Input{{Revision: 0}, {Revision: 1, Cursor: -1}, {Revision: 1, Text: "中文", Cursor: 1}} {
		_, err := s.Suggest(context.Background(), input)
		require.ErrorIs(t, err, ErrInvalid)
	}
}

func TestSuggest_EditOrDisconnectCancelsWithoutNewModelCall(t *testing.T) {
	for _, action := range []string{"edit", "disconnect"} {
		t.Run(action, func(t *testing.T) {
			started := make(chan struct{})
			s := newTestSession(t, func(ctx context.Context, _ Binding, _ Input) (string, error) {
				close(started)
				<-ctx.Done()
				return "late", nil
			})
			done := make(chan error, 1)
			go func() {
				_, err := s.Suggest(context.Background(), Input{Revision: 1})
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("generator did not start")
			}
			if action == "edit" {
				require.NoError(t, s.Invalidate(2))
			} else {
				s.Close()
			}
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("request not cancelled")
			}
		})
	}
}
