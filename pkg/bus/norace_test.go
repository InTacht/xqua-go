//go:build !race

package bus_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/InTacht/xqua-go/pkg/bus"
)

// Measurements that are unreliable under the race detector (allocation counts,
// heap bytes, goroutine accounting) live here. //go:build !race excludes them
// from -race builds instead of a raceEnabled const pair, so default gopls
// (no -race) can analyze this file without a conflicting race-tagged twin.

// TestAllocsPublishSingleSub locks in the zero-alloc publish fast path: exact
// subject, single subscriber, non-cancellable context.
func TestAllocsPublishSingleSub(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 1 << 16})
	t.Cleanup(func() { b.Close() })

	var n atomic.Int64
	if _, err := b.Subscribe("a.b.c", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		n.Add(1)
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "a.b.c", Data: []byte("payload")}
	allocs := testing.AllocsPerRun(2000, func() {
		if err := b.Publish(ctx, msg); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Fatalf("publish allocs/op = %v, want 0", allocs)
	}
}

// TestAllocsPublishWithWildcardPresent proves an unrelated wildcard tap does not
// add allocations to exact-subject publishes (subject is walked, not split).
func TestAllocsPublishWithWildcardPresent(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 1 << 16})
	t.Cleanup(func() { b.Close() })

	if _, err := b.Subscribe("a.b.c", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Subscribe("x.>", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "a.b.c", Data: []byte("payload")}
	allocs := testing.AllocsPerRun(2000, func() {
		if err := b.Publish(ctx, msg); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Fatalf("publish-with-wildcard allocs/op = %v, want 0", allocs)
	}
}

// TestAllocsRequestReply bounds the request/reply path. The waiter is pooled and
// the pending map is gone, so only the inbox string should allocate.
func TestAllocsRequestReply(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 1024})
	t.Cleanup(func() { b.Close() })

	if _, err := b.Subscribe("rr.alloc", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: msg.Data}, nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "rr.alloc", Data: []byte("ping")}
	allocs := testing.AllocsPerRun(2000, func() {
		if _, err := b.Request(ctx, msg); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("request/reply allocs/op = %v", allocs)
	if allocs > 0 {
		t.Fatalf("request/reply allocs/op = %v, want 0 (pooled waiter + stable inbox)", allocs)
	}
}

// TestAllocsPublishLargeFanout proves the fused publish path has no target-slice
// heap escape: broadcasting to many subscribers stays allocation-free.
func TestAllocsPublishLargeFanout(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 1 << 16})
	t.Cleanup(func() { b.Close() })

	for range 32 {
		if _, err := b.Subscribe("a.b.c", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
			return bus.Message{}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "a.b.c", Data: []byte("payload")}
	allocs := testing.AllocsPerRun(2000, func() {
		if err := b.Publish(ctx, msg); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Fatalf("publish fan-out=32 allocs/op = %v, want 0", allocs)
	}
}

// TestChurnReusesMailbox proves subscription pooling: a subscribe -> unsubscribe
// cycle must not reallocate the ~KB mailbox channel each time. We measure heap
// bytes per cycle and require it well under one mailbox's worth.
func TestChurnReusesMailbox(t *testing.T) {
	// A large mailbox makes reuse-vs-reallocation unmistakable: one mailbox is
	// bufSize * sizeof(delivery) bytes (delivery embeds a Message, > 64 bytes),
	// so a per-cycle reallocation would cost far more than the small copy-on-
	// write registry allocs a pooled cycle pays.
	const bufSize = 1024
	b := bus.NewLocal(bus.LocalConfig{BufferSize: bufSize})
	t.Cleanup(func() { b.Close() })

	h := func(ctx context.Context, msg bus.Message) (bus.Message, error) { return bus.Message{}, nil }
	cycle := func() {
		sub, err := b.Subscribe("churn.subj", h)
		if err != nil {
			t.Fatal(err)
		}
		if err := sub.Unsubscribe(); err != nil {
			t.Fatal(err)
		}
	}

	// Warm the pool so the first mailbox allocation is amortised away.
	for range 200 {
		cycle()
	}
	// Give recycling workers a moment to return objects to the pool.
	time.Sleep(20 * time.Millisecond)

	// A single reallocated mailbox is at least bufSize*64 bytes; a pooled cycle
	// pays only the small registry/handle allocs. The threshold sits far below
	// one mailbox yet well above the pooled cost.
	const mailboxLowerBound = bufSize * 64
	bytesPerOp := bytesPerRun(5000, cycle)
	t.Logf("ephemeral churn bytes/op = %.0f (one mailbox is >= %d bytes)", bytesPerOp, mailboxLowerBound)
	if bytesPerOp > mailboxLowerBound {
		t.Fatalf("churn bytes/op = %.0f exceeds one mailbox (%d): pooling is not reusing the mailbox",
			bytesPerOp, mailboxLowerBound)
	}
}

func bytesPerRun(runs int, f func()) float64 {
	f()
	runtime.GC()
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	for range runs {
		f()
	}
	runtime.ReadMemStats(&m2)
	return float64(m2.TotalAlloc-m1.TotalAlloc) / float64(runs)
}

// TestNoGoroutineLeakAfterChurnAndClose verifies subscription workers actually
// exit: after heavy churn and Close, the goroutine count returns to baseline.
func TestNoGoroutineLeakAfterChurnAndClose(t *testing.T) {
	runtime.GC()
	base := runtime.NumGoroutine()

	b := bus.NewLocal(bus.LocalConfig{BufferSize: 8})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for range stressStep {
				sub, err := b.Subscribe("leak.subj", func(context.Context, bus.Message) (bus.Message, error) {
					return bus.Message{}, nil
				})
				if err != nil {
					t.Errorf("subscribe: %v", err)
					return
				}
				_ = b.Publish(ctx, pub("leak.subj", []byte("x")))
				sub.Unsubscribe()
			}
		}()
	}
	wg.Wait()
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	// Workers should wind down to baseline; poll to avoid scheduler flakiness.
	deadline := time.Now().Add(2 * time.Second)
	for {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n <= base+5 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutine leak: baseline=%d, now=%d", base, n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
