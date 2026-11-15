package tcpwindow

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRegistryWindowAndReplay(t *testing.T) {
	r, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	lease, err := r.Grant(now, "request-00000001", 1, DefaultDuration)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := r.Grant(now.Add(time.Second), lease.ID, 1, DefaultDuration)
	if err != nil || retry != lease {
		t.Fatalf("retry extended lease: %v %v", retry, err)
	}
	for _, tc := range []struct {
		name    string
		at      time.Time
		allowed bool
	}{
		{"open", now, true},
		{"before", now.Add(-time.Second), false},
		{"boundary", lease.ExpiresAt, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := r.Admit(tc.at); ok != tc.allowed {
				t.Fatalf("allowed=%v", ok)
			}
		})
	}
	if _, err := r.Grant(lease.ExpiresAt, lease.ID, 1, DefaultDuration); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired replay: %v", err)
	}
	if _, err := r.Get(now, lease.ID, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner lookup: %v", err)
	}
	if err := r.Revoke(lease.ID, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner revoke: %v", err)
	}
	if err := r.Revoke(lease.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Admit(now); ok {
		t.Fatal("revoked grant accepted")
	}
	if _, err := r.Grant(now, lease.ID, 1, DefaultDuration); !errors.Is(err, ErrExpired) {
		t.Fatalf("revoked replay: %v", err)
	}
}

func TestRegistryIndependentLeasesAndCapacity(t *testing.T) {
	r, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	a, err := r.Grant(now, "request-00000001", 1, MinDuration)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Grant(now, "request-00000002", 2, DefaultDuration)
	if err != nil {
		t.Fatal(err)
	}
	if expiry, ok := r.NextExpiry(now); !ok || !expiry.Equal(a.ExpiresAt) {
		t.Fatal("wrong next expiry")
	}
	if err := r.Revoke(a.ID, 1); err != nil {
		t.Fatal(err)
	}
	if lease, ok := r.Admit(now); !ok || lease.ID != b.ID {
		t.Fatal("revoking one affected other lease")
	}
	if _, err := r.Grant(now, "request-00000003", 1, DefaultDuration); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity: %v", err)
	}
	if _, err := r.Grant(now, b.ID, 1, DefaultDuration); !errors.Is(err, ErrConflict) {
		t.Fatalf("reused id: %v", err)
	}
	if _, err := r.Grant(b.ExpiresAt.Add(ReplayRetention), "request-00000003", 1, DefaultDuration); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if _, ok := r.Admit(now); ok {
		t.Fatal("closed accepted")
	}
	if _, err := r.Grant(now, "request-00000004", 1, DefaultDuration); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed grant: %v", err)
	}
}

func TestRegistryConcurrentGrantRevoke(t *testing.T) {
	r, err := New(DefaultCapacity)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.Grant(now, "request-00000001", 1, DefaultDuration)
			if err != nil && !errors.Is(err, ErrExpired) {
				t.Errorf("grant: %v", err)
			}
			r.Admit(now)
			r.NextExpiry(now)
			if err := r.Revoke("request-00000001", 1); err != nil {
				t.Errorf("revoke: %v", err)
			}
		}()
	}
	wg.Wait()
}
