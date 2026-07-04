package bus_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/InTacht/xqua-go/pkg/bus"
)

func newBus(t *testing.T) *bus.Local {
	t.Helper()
	b := bus.NewLocal(bus.LocalConfig{})
	t.Cleanup(func() { b.Close() })
	return b
}

func pub(subject string, data []byte) bus.Message {
	return bus.Message{Subject: subject, Data: data}
}

func TestPublishSubscribeFanout(t *testing.T) {
	b := newBus(t)

	var got1, got2 atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)

	handler := func(id *atomic.Int32) bus.Handler {
		return func(ctx context.Context, msg bus.Message) (bus.Message, error) {
			defer wg.Done()
			if string(msg.Data) != "hello" {
				t.Errorf("unexpected data: %q", msg.Data)
			}
			id.Add(1)
			return bus.Message{}, nil
		}
	}

	if _, err := b.Subscribe("demo.fanout", handler(&got1)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Subscribe("demo.fanout", handler(&got2)); err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(context.Background(), pub("demo.fanout", []byte("hello"))); err != nil {
		t.Fatal(err)
	}

	waitDone(t, &wg)
	if got1.Load() != 1 || got2.Load() != 1 {
		t.Fatalf("want both subscribers to receive once, got %d and %d", got1.Load(), got2.Load())
	}
}

func TestQueueSubscribeCompeting(t *testing.T) {
	b := newBus(t)

	var got1, got2 atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	makeHandler := func(id *atomic.Int32) bus.Handler {
		return func(ctx context.Context, msg bus.Message) (bus.Message, error) {
			id.Add(1)
			wg.Done()
			return bus.Message{}, nil
		}
	}

	if _, err := b.QueueSubscribe("demo.work", "workers", makeHandler(&got1)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.QueueSubscribe("demo.work", "workers", makeHandler(&got2)); err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(context.Background(), pub("demo.work", []byte("job"))); err != nil {
		t.Fatal(err)
	}

	waitDone(t, &wg)
	time.Sleep(20 * time.Millisecond) // allow a mistaken second delivery to surface

	total := got1.Load() + got2.Load()
	if total != 1 {
		t.Fatalf("want exactly one receiver, got total=%d (a=%d b=%d)", total, got1.Load(), got2.Load())
	}
}

func TestRequestReply(t *testing.T) {
	b := newBus(t)

	_, err := b.Subscribe("demo.echo", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: append([]byte("pong:"), msg.Data...)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	reply, err := b.Request(context.Background(), pub("demo.echo", []byte("ping")))
	if err != nil {
		t.Fatal(err)
	}
	if string(reply.Data) != "pong:ping" {
		t.Fatalf("unexpected reply: %q", reply.Data)
	}
}

func TestRequestReplyHeaders(t *testing.T) {
	b := newBus(t)

	_, err := b.Subscribe("demo.hdr", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		if msg.Headers["x-request-id"] != "req-1" {
			t.Errorf("request header not propagated: %v", msg.Headers)
		}
		return bus.Message{
			Data:    []byte("ok"),
			Headers: map[string]string{"x-trace": "t-1"},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	reply, err := b.Request(context.Background(), bus.Message{
		Subject: "demo.hdr",
		Data:    []byte("go"),
		Headers: map[string]string{"x-request-id": "req-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.Headers["x-trace"] != "t-1" {
		t.Fatalf("reply header missing: %v", reply.Headers)
	}
}

func TestRequestHandlerError(t *testing.T) {
	b := newBus(t)

	sentinel := errors.New("boom")
	_, err := b.Subscribe("demo.fail", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{}, sentinel
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = b.Request(context.Background(), pub("demo.fail", nil))
	if !errors.Is(err, sentinel) {
		t.Fatalf("want handler error propagated to requester, got %v", err)
	}
}

func TestRequestNoResponders(t *testing.T) {
	b := newBus(t)

	_, err := b.Request(context.Background(), pub("demo.none", []byte("x")))
	if !errors.Is(err, bus.ErrNoResponders) {
		t.Fatalf("want ErrNoResponders, got %v", err)
	}
}

func TestRequestCancelled(t *testing.T) {
	b := newBus(t)

	started := make(chan struct{})
	_, err := b.Subscribe("demo.slow", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		return bus.Message{Data: []byte("late")}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	_, err = b.Request(ctx, pub("demo.slow", []byte("x")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRequestTimeout(t *testing.T) {
	b := newBus(t)

	_, err := b.Subscribe("demo.slow", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		time.Sleep(200 * time.Millisecond)
		return bus.Message{Data: []byte("late")}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = b.Request(ctx, pub("demo.slow", []byte("x")))
	if !errors.Is(err, bus.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func TestRequestDefaultTimeout(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{DefaultRequestTimeout: 20 * time.Millisecond})
	t.Cleanup(func() { b.Close() })

	// Responder never replies (returns Reply-less, but this is request so a
	// reply IS produced from the returned message; force a hang by blocking).
	_, err := b.Subscribe("demo.hang", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		<-ctx.Done()
		return bus.Message{}, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}

	// Caller context has no deadline; the bus-wide default must cap it.
	_, err = b.Request(context.Background(), pub("demo.hang", nil))
	if !errors.Is(err, bus.ErrTimeout) {
		t.Fatalf("want ErrTimeout from default timeout, got %v", err)
	}
}

func TestHandlerPanicRecovered(t *testing.T) {
	var reported atomic.Int32
	errCh := make(chan error, 1)
	b := bus.NewLocal(bus.LocalConfig{
		OnError: func(msg bus.Message, err error) {
			reported.Add(1)
			select {
			case errCh <- err:
			default:
			}
		},
	})
	t.Cleanup(func() { b.Close() })

	var wg sync.WaitGroup
	wg.Add(1)
	if _, err := b.Subscribe("demo.panic", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		defer wg.Done()
		panic("kaboom")
	}); err != nil {
		t.Fatal(err)
	}
	// A healthy subscriber must keep receiving after the panic.
	var ok atomic.Int32
	if _, err := b.Subscribe("demo.panic", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		ok.Add(1)
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(context.Background(), pub("demo.panic", nil)); err != nil {
		t.Fatal(err)
	}
	waitDone(t, &wg)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected panic error reported")
		}
	case <-time.After(time.Second):
		t.Fatal("OnError not called for panic")
	}
}

func TestPerSubscriptionOrdering(t *testing.T) {
	b := newBus(t)

	const n = 200
	got := make([]int, 0, n)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)

	if _, err := b.Subscribe("demo.order", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		defer wg.Done()
		var v int
		fmt.Sscanf(string(msg.Data), "%d", &v)
		mu.Lock()
		got = append(got, v)
		mu.Unlock()
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	for i := range n {
		if err := b.Publish(context.Background(), pub("demo.order", []byte(fmt.Sprintf("%d", i)))); err != nil {
			t.Fatal(err)
		}
	}
	waitDone(t, &wg)

	mu.Lock()
	defer mu.Unlock()
	for i, v := range got {
		if v != i {
			t.Fatalf("out-of-order delivery at %d: got %d", i, v)
		}
	}
}

func TestBackpressureBlocksThenCancels(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 1})
	t.Cleanup(func() { b.Close() })

	release := make(chan struct{})
	if _, err := b.Subscribe("demo.slow", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		<-release
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	defer close(release)

	// First message occupies the worker, second fills the buffer (cap 1). The
	// third publish must block until ctx cancellation.
	_ = b.Publish(context.Background(), pub("demo.slow", nil))
	_ = b.Publish(context.Background(), pub("demo.slow", nil))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := b.Publish(ctx, pub("demo.slow", nil))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded from blocked publish, got %v", err)
	}
}

func TestDrainProcessesQueued(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})

	var handled atomic.Int32
	if _, err := b.Subscribe("demo.drain", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		time.Sleep(5 * time.Millisecond)
		handled.Add(1)
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	const n = 20
	for range n {
		if err := b.Publish(context.Background(), pub("demo.drain", nil)); err != nil {
			t.Fatal(err)
		}
	}

	if err := b.Drain(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if handled.Load() != n {
		t.Fatalf("drain should process all queued: got %d want %d", handled.Load(), n)
	}
}

func TestCloseRejectsNewOps(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(context.Background(), pub("x", []byte("y"))); !errors.Is(err, bus.ErrClosed) {
		t.Fatalf("Publish: want ErrClosed, got %v", err)
	}
	if _, err := b.Request(context.Background(), pub("x", []byte("y"))); !errors.Is(err, bus.ErrClosed) {
		t.Fatalf("Request: want ErrClosed, got %v", err)
	}
	noop := func(context.Context, bus.Message) (bus.Message, error) { return bus.Message{}, nil }
	if _, err := b.Subscribe("x", noop); !errors.Is(err, bus.ErrClosed) {
		t.Fatalf("Subscribe: want ErrClosed, got %v", err)
	}
	if _, err := b.QueueSubscribe("x", "q", noop); !errors.Is(err, bus.ErrClosed) {
		t.Fatalf("QueueSubscribe: want ErrClosed, got %v", err)
	}
}

func TestCloseCancelsInFlightRequest(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})

	started := make(chan struct{})
	_, err := b.Subscribe("demo.slow", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		close(started)
		time.Sleep(time.Second)
		return bus.Message{Data: []byte("late")}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := b.Request(context.Background(), pub("demo.slow", []byte("x")))
		errCh <- err
	}()

	<-started
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, bus.ErrClosed) {
			t.Fatalf("want ErrClosed, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not unblock after Close")
	}
}

func TestUnsubscribeAfterCloseNoop(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})
	sub, err := b.Subscribe("demo.x", func(context.Context, bus.Message) (bus.Message, error) {
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("Unsubscribe after Close should be nil, got %v", err)
	}
	// Idempotent second call.
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("second Unsubscribe should be nil, got %v", err)
	}
}

func TestQueueSubscribeRoundRobin(t *testing.T) {
	b := newBus(t)

	var a, c atomic.Int32
	var wg sync.WaitGroup
	wg.Add(4)

	makeHandler := func(id *atomic.Int32) bus.Handler {
		return func(ctx context.Context, msg bus.Message) (bus.Message, error) {
			id.Add(1)
			wg.Done()
			return bus.Message{}, nil
		}
	}

	if _, err := b.QueueSubscribe("demo.rr", "workers", makeHandler(&a)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.QueueSubscribe("demo.rr", "workers", makeHandler(&c)); err != nil {
		t.Fatal(err)
	}

	for range 4 {
		if err := b.Publish(context.Background(), pub("demo.rr", []byte("x"))); err != nil {
			t.Fatal(err)
		}
	}

	waitDone(t, &wg)
	if a.Load() != 2 || c.Load() != 2 {
		t.Fatalf("want even split 2/2, got %d/%d", a.Load(), c.Load())
	}
}

func TestWildcardMatching(t *testing.T) {
	cases := []struct {
		pattern string
		subject string
		match   bool
	}{
		{"a.*", "a.b", true},
		{"a.*", "a.b.c", false},
		{"a.*", "a", false},
		{"a.>", "a.b", true},
		{"a.>", "a.b.c", true},
		{"a.>", "a", false},
		{"*.b", "a.b", true},
		{"a.*.c", "a.x.c", true},
		{"a.*.c", "a.x.d", false},
		{"a.b", "a.b", true},
	}

	for _, tc := range cases {
		t.Run(tc.pattern+"~"+tc.subject, func(t *testing.T) {
			b := newBus(t)
			var got atomic.Int32
			var wg sync.WaitGroup
			if tc.match {
				wg.Add(1)
			}
			if _, err := b.Subscribe(tc.pattern, func(ctx context.Context, msg bus.Message) (bus.Message, error) {
				got.Add(1)
				if tc.match {
					wg.Done()
				}
				return bus.Message{}, nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := b.Publish(context.Background(), pub(tc.subject, nil)); err != nil {
				t.Fatal(err)
			}
			if tc.match {
				waitDone(t, &wg)
				if got.Load() != 1 {
					t.Fatalf("pattern %q subject %q: want 1 delivery, got %d", tc.pattern, tc.subject, got.Load())
				}
			} else {
				time.Sleep(20 * time.Millisecond)
				if got.Load() != 0 {
					t.Fatalf("pattern %q subject %q: want no delivery, got %d", tc.pattern, tc.subject, got.Load())
				}
			}
		})
	}
}

func TestWildcardAndExactBothReceive(t *testing.T) {
	b := newBus(t)

	var exact, wild atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)
	if _, err := b.Subscribe("a.b", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		exact.Add(1)
		wg.Done()
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Subscribe("a.>", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		wild.Add(1)
		wg.Done()
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(context.Background(), pub("a.b", nil)); err != nil {
		t.Fatal(err)
	}
	waitDone(t, &wg)
	if exact.Load() != 1 || wild.Load() != 1 {
		t.Fatalf("want both exact and wildcard to receive, got exact=%d wild=%d", exact.Load(), wild.Load())
	}
}

func TestEncodeDecodeJSON(t *testing.T) {
	type payload struct {
		Q string `json:"q"`
	}
	data, err := bus.EncodeJSON(payload{Q: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	var out payload
	if err := bus.DecodeJSON(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Q != "hi" {
		t.Fatalf("got %#v", out)
	}
}

func waitDone(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handlers")
	}
}

// --- concurrency / correctness stress --------------------------------------

// TestConcurrentRequestStress races many requesters against replies, short
// timeouts, and a churning responder. It must never panic, deadlock, or cross
// replies (checked via -race and result matching).
func TestConcurrentRequestStress(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 64, DefaultRequestTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { b.Close() })

	// Responder echoes the request data so replies can be matched to requests.
	if _, err := b.Subscribe("stress.rr", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: msg.Data}, nil
	}); err != nil {
		t.Fatal(err)
	}

	const workers = 32
	const perWorker = 500
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(w int) {
			defer wg.Done()
			for i := range perWorker {
				want := []byte(fmt.Sprintf("%d-%d", w, i))
				var ctx context.Context
				var cancel context.CancelFunc
				if i%3 == 0 {
					ctx, cancel = context.WithTimeout(context.Background(), time.Duration(i%5+1)*time.Microsecond)
				} else {
					ctx, cancel = context.WithCancel(context.Background())
				}
				reply, err := b.Request(ctx, bus.Message{Subject: "stress.rr", Data: want})
				cancel()
				if err != nil {
					// Timeouts and cancellations are acceptable outcomes here.
					if !errors.Is(err, bus.ErrTimeout) && !errors.Is(err, context.Canceled) &&
						!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, bus.ErrClosed) {
						t.Errorf("unexpected request error: %v", err)
						return
					}
					continue
				}
				if string(reply.Data) != string(want) {
					t.Errorf("crossed reply: want %q got %q", want, reply.Data)
					return
				}
			}
		}(w)
	}
	waitDone(t, &wg)
}

// TestEphemeralChurnStress races subscribe/publish/unsubscribe cycles against a
// steady publisher on the same subject. Generation tagging must guarantee a
// recycled subscription never receives a straggler meant for a prior use.
func TestEphemeralChurnStress(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 16})
	t.Cleanup(func() { b.Close() })

	stop := make(chan struct{})
	var pubWG sync.WaitGroup
	pubWG.Add(1)
	go func() {
		defer pubWG.Done()
		ctx := context.Background()
		for {
			select {
			case <-stop:
				return
			default:
				_ = b.Publish(ctx, bus.Message{Subject: "churn.race", Data: []byte("x")})
			}
		}
	}()

	const workers = 16
	const perWorker = 300
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range perWorker {
				sub, err := b.Subscribe("churn.race", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
					return bus.Message{}, nil
				})
				if err != nil {
					t.Errorf("subscribe: %v", err)
					return
				}
				if err := sub.Unsubscribe(); err != nil {
					t.Errorf("unsubscribe: %v", err)
					return
				}
			}
		}()
	}
	waitDone(t, &wg)
	close(stop)
	pubWG.Wait()
}

// TestNoDeliveryAfterUnsubscribe is the correctness invariant: once Unsubscribe
// returns, the old handler receives nothing, even with publishers hammering the
// subject.
func TestNoDeliveryAfterUnsubscribe(t *testing.T) {
	b := newBus(t)

	var received atomic.Int64
	sub, err := b.Subscribe("gone.subj", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		received.Add(1)
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sub.Unsubscribe(); err != nil {
		t.Fatal(err)
	}

	after := received.Load()
	ctx := context.Background()
	for range 1000 {
		if err := b.Publish(ctx, pub("gone.subj", []byte("x"))); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if got := received.Load(); got != after {
		t.Fatalf("handler received %d messages after unsubscribe", got-after)
	}
}

// TestPooledReuseRoutesCorrectly forces a subscription object to be recycled and
// reused on a different subject, then proves messages route by the live
// registration, never by a stale reference held in a snapshot.
func TestPooledReuseRoutesCorrectly(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 8})
	t.Cleanup(func() { b.Close() })

	var firstHits atomic.Int64
	first, err := b.Subscribe("subj.first", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		firstHits.Add(1)
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	// Let the worker recycle the object into the pool.
	time.Sleep(20 * time.Millisecond)

	var secondHits atomic.Int64
	var wg sync.WaitGroup
	second, err := b.Subscribe("subj.second", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		secondHits.Add(1)
		wg.Done()
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Unsubscribe() })

	baseFirst := firstHits.Load()
	ctx := context.Background()
	// Publishing to the old subject must reach nobody.
	for range 100 {
		if err := b.Publish(ctx, pub("subj.first", []byte("x"))); err != nil {
			t.Fatal(err)
		}
	}
	// Publishing to the new subject must reach the reused subscription.
	wg.Add(1)
	if err := b.Publish(ctx, pub("subj.second", []byte("y"))); err != nil {
		t.Fatal(err)
	}
	waitDone(t, &wg)
	time.Sleep(10 * time.Millisecond)

	if got := firstHits.Load(); got != baseFirst {
		t.Fatalf("old subject delivered %d messages after unsubscribe", got-baseFirst)
	}
	if secondHits.Load() == 0 {
		t.Fatal("reused subscription did not receive on its new subject")
	}
}

// --- validation and lifecycle edge cases ------------------------------------

func TestSubscribeValidation(t *testing.T) {
	b := newBus(t)
	noop := func(context.Context, bus.Message) (bus.Message, error) { return bus.Message{}, nil }

	if _, err := b.Subscribe("", noop); err == nil {
		t.Fatal("Subscribe with empty subject should error")
	}
	if _, err := b.Subscribe("s", nil); err == nil {
		t.Fatal("Subscribe with nil handler should error")
	}
	if _, err := b.QueueSubscribe("s", "", noop); err == nil {
		t.Fatal("QueueSubscribe with empty queue should error")
	}
	if err := b.Publish(context.Background(), pub("", nil)); err == nil {
		t.Fatal("Publish with empty subject should error")
	}
	if _, err := b.Request(context.Background(), pub("", nil)); err == nil {
		t.Fatal("Request with empty subject should error")
	}
}

func TestDoubleUnsubscribeIsNoop(t *testing.T) {
	b := newBus(t)
	sub, err := b.Subscribe("dbl.subj", func(context.Context, bus.Message) (bus.Message, error) {
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("first unsubscribe: %v", err)
	}
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("second unsubscribe should be a no-op, got %v", err)
	}
}

// TestFanoutUnsubscribeOne removes one of two fan-out subscribers and verifies
// the survivor keeps receiving (exercises copy-on-write removal of a fan-out
// ref that leaves the entry non-empty).
func TestFanoutUnsubscribeOne(t *testing.T) {
	b := newBus(t)

	var aHits, bHits atomic.Int64
	subA, err := b.Subscribe("fan.subj", func(context.Context, bus.Message) (bus.Message, error) {
		aHits.Add(1)
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	if _, err := b.Subscribe("fan.subj", func(context.Context, bus.Message) (bus.Message, error) {
		bHits.Add(1)
		wg.Done()
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := subA.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	// Give the removal + worker exit a moment to settle.
	time.Sleep(10 * time.Millisecond)
	baseA := aHits.Load()

	ctx := context.Background()
	const n = 50
	wg.Add(n)
	for range n {
		if err := b.Publish(ctx, pub("fan.subj", []byte("x"))); err != nil {
			t.Fatal(err)
		}
	}
	waitDone(t, &wg)
	time.Sleep(10 * time.Millisecond)

	if got := aHits.Load(); got != baseA {
		t.Fatalf("unsubscribed fan-out subscriber still received %d", got-baseA)
	}
	if bHits.Load() != n {
		t.Fatalf("survivor got %d, want %d", bHits.Load(), n)
	}
}

// TestQueueGroupUnsubscribe removes queue members one at a time: a partial
// removal shrinks the group (non-empty copy-on-write) and the final removal
// empties it (entry deleted).
func TestQueueGroupUnsubscribe(t *testing.T) {
	b := newBus(t)

	var total atomic.Int64
	var wg sync.WaitGroup
	mk := func() (bus.Subscription, error) {
		return b.QueueSubscribe("q.subj", "g", func(context.Context, bus.Message) (bus.Message, error) {
			total.Add(1)
			wg.Done()
			return bus.Message{}, nil
		})
	}
	s1, err := mk()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mk(); err != nil {
		t.Fatal(err)
	}
	if _, err := mk(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// All three members present: 6 messages are delivered to the group.
	wg.Add(6)
	for range 6 {
		if err := b.Publish(ctx, pub("q.subj", nil)); err != nil {
			t.Fatal(err)
		}
	}
	waitDone(t, &wg)

	// Remove one member; the group still exists and keeps delivering.
	if err := s1.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	wg.Add(4)
	for range 4 {
		if err := b.Publish(ctx, pub("q.subj", nil)); err != nil {
			t.Fatal(err)
		}
	}
	waitDone(t, &wg)

	if got := total.Load(); got != 10 {
		t.Fatalf("queue group delivered %d, want 10", got)
	}
}

// TestWildcardUnsubscribeRemovesTap unsubscribes a wildcard subscription and
// verifies it stops matching (exercises removeWild).
func TestWildcardUnsubscribeRemovesTap(t *testing.T) {
	b := newBus(t)

	var hits atomic.Int64
	var wg sync.WaitGroup
	sub, err := b.Subscribe("w.*", func(context.Context, bus.Message) (bus.Message, error) {
		hits.Add(1)
		wg.Done()
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	wg.Add(1)
	if err := b.Publish(ctx, pub("w.x", nil)); err != nil {
		t.Fatal(err)
	}
	waitDone(t, &wg)

	if err := sub.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	base := hits.Load()
	for range 100 {
		if err := b.Publish(ctx, pub("w.x", nil)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if got := hits.Load(); got != base {
		t.Fatalf("removed wildcard tap still received %d", got-base)
	}
}

// TestWildcardUnsubscribeKeepsOthers removes one wildcard tap while another
// remains (exercises removeWild's filtering that preserves survivors).
func TestWildcardUnsubscribeKeepsOthers(t *testing.T) {
	b := newBus(t)

	var aHits, bHits atomic.Int64
	var wg sync.WaitGroup
	subA, err := b.Subscribe("w.*", func(context.Context, bus.Message) (bus.Message, error) {
		aHits.Add(1)
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Subscribe("m.>", func(context.Context, bus.Message) (bus.Message, error) {
		bHits.Add(1)
		wg.Done()
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := subA.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()
	// Surviving wildcard still matches.
	wg.Add(1)
	if err := b.Publish(ctx, pub("m.foo.bar", nil)); err != nil {
		t.Fatal(err)
	}
	waitDone(t, &wg)
	// Removed wildcard no longer matches.
	baseA := aHits.Load()
	for range 50 {
		if err := b.Publish(ctx, pub("w.y", nil)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(20 * time.Millisecond)

	if got := aHits.Load(); got != baseA {
		t.Fatalf("removed wildcard still received %d", got-baseA)
	}
	if bHits.Load() == 0 {
		t.Fatal("surviving wildcard did not receive")
	}
}

// TestDrainTimeoutAbandons drives the graceful-drain path past its deadline: an
// in-flight handler outlasts the drain context, so Drain abandons and returns
// the context error once the handler finally completes.
func TestDrainTimeoutAbandons(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})

	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	if _, err := b.Subscribe("drain.slow", func(context.Context, bus.Message) (bus.Message, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := b.Publish(context.Background(), pub("drain.slow", nil)); err != nil {
		t.Fatal(err)
	}
	<-entered // handler is now blocking in-flight

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- b.Drain(ctx) }()

	// Let the drain deadline elapse, then release the handler so the abandoned
	// Close can finish waiting on the in-flight message.
	time.Sleep(30 * time.Millisecond)
	close(release)

	select {
	case err := <-errCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded from abandoned drain, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Drain did not return")
	}
}

// TestDrainNoopAfterClose covers Drain returning early on an already-closed bus.
func TestDrainNoopAfterClose(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Drain(context.Background()); err != nil {
		t.Fatalf("Drain after Close should be nil, got %v", err)
	}
}

// TestCloseIdempotent covers the second Close returning immediately.
func TestCloseIdempotent(t *testing.T) {
	b := bus.NewLocal(bus.LocalConfig{})
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close should be nil, got %v", err)
	}
}

// TestRequestQueueGroup issues a request against a queue-group subject: exactly
// one member replies (exercises the queue-group target selection on the request
// path).
func TestRequestQueueGroup(t *testing.T) {
	b := newBus(t)

	var served atomic.Int64
	reply := func(who byte) bus.Handler {
		return func(ctx context.Context, msg bus.Message) (bus.Message, error) {
			served.Add(1)
			return bus.Message{Data: append([]byte{who, ':'}, msg.Data...)}, nil
		}
	}
	if _, err := b.QueueSubscribe("rr.q", "workers", reply('a')); err != nil {
		t.Fatal(err)
	}
	if _, err := b.QueueSubscribe("rr.q", "workers", reply('b')); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for range 8 {
		res, err := b.Request(ctx, pub("rr.q", []byte("ping")))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Data) == 0 {
			t.Fatal("empty reply")
		}
	}
	if served.Load() != 8 {
		t.Fatalf("queue responders served %d, want 8 (one per request)", served.Load())
	}
}

// TestRequestWildcardResponder issues a request whose subject only matches via a
// wildcard subscription (exercises the wildcard scan on the request path).
func TestRequestWildcardResponder(t *testing.T) {
	b := newBus(t)

	if _, err := b.Subscribe("svc.*", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: append([]byte("echo:"), msg.Data...)}, nil
	}); err != nil {
		t.Fatal(err)
	}

	res, err := b.Request(context.Background(), pub("svc.ping", []byte("hi")))
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Data) != "echo:hi" {
		t.Fatalf("got %q, want %q", res.Data, "echo:hi")
	}
}
