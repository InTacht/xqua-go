package bus_test

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/InTacht/xqua-go/pkg/bus"
)

// These tests hammer the local bus from many goroutines at once. They are meant
// to be run with the race detector (`go test -race`) to surface data races in
// the registry, pooling, and request paths, and to prove the bus never
// deadlocks, panics, crosses replies, or double-delivers under contention.
//
// Every test bounds its own runtime and joins all goroutines it spawns so a
// hang shows up as a test timeout rather than a leaked goroutine.

const stressStep = 500

// TestStressSubscribeUnsubscribePublish races registry writes (subscribe /
// unsubscribe on the same subject) against a stream of publishers. It asserts
// only that nothing races, panics, or hangs, and that once the churn settles a
// fresh subscriber still receives — i.e. the registry is left consistent.
func TestStressSubscribeUnsubscribePublish(t *testing.T) {
	t.Parallel()
	b := newBus(t)

	const subject = "stress.reg"
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Publishers.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for {
				select {
				case <-stop:
					return
				default:
					_ = b.Publish(ctx, pub(subject, []byte("x")))
				}
			}
		}()
	}
	// Subscribers churning on the same subject.
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range stressStep {
				sub, err := b.Subscribe(subject, func(context.Context, bus.Message) (bus.Message, error) {
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

	// Let the churn run, then stop publishers and join everything.
	time.Sleep(50 * time.Millisecond)
	close(stop)
	waitDone(t, &wg)

	// Registry must still be usable and consistent.
	var got atomic.Int64
	var done sync.WaitGroup
	done.Add(1)
	sub, err := b.Subscribe(subject, func(context.Context, bus.Message) (bus.Message, error) {
		got.Add(1)
		done.Done()
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	if err := b.Publish(context.Background(), pub(subject, []byte("final"))); err != nil {
		t.Fatal(err)
	}
	waitDone(t, &done)
	if got.Load() == 0 {
		t.Fatal("subscriber added after churn received nothing")
	}
}

// TestStressQueueGroupChurn proves the queue-group invariant under churn: a
// message delivered to a group reaches at most one member, even while members
// join and leave. A permanent member keeps the group non-empty.
func TestStressQueueGroupChurn(t *testing.T) {
	t.Parallel()
	b := newBus(t)

	const subject = "stress.queue"
	var seen sync.Map // msgID -> struct{}; detects double delivery
	var delivered atomic.Int64
	record := func(msg bus.Message) {
		if len(msg.Data) < 8 {
			return
		}
		id := binary.LittleEndian.Uint64(msg.Data)
		if _, dup := seen.LoadOrStore(id, struct{}{}); dup {
			t.Errorf("message %d delivered more than once within queue group", id)
			return
		}
		delivered.Add(1)
	}

	// Permanent member.
	perm, err := b.QueueSubscribe(subject, "g", func(_ context.Context, msg bus.Message) (bus.Message, error) {
		record(msg)
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer perm.Unsubscribe()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Churning members.
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub, err := b.QueueSubscribe(subject, "g", func(_ context.Context, msg bus.Message) (bus.Message, error) {
					record(msg)
					return bus.Message{}, nil
				})
				if err != nil {
					t.Errorf("queue subscribe: %v", err)
					return
				}
				sub.Unsubscribe()
			}
		}()
	}

	// Publishers with unique message IDs.
	var idSeq atomic.Uint64
	var published atomic.Int64
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for {
				select {
				case <-stop:
					return
				default:
				}
				data := make([]byte, 8)
				binary.LittleEndian.PutUint64(data, idSeq.Add(1))
				if err := b.Publish(ctx, pub(subject, data)); err != nil {
					return
				}
				published.Add(1)
			}
		}()
	}

	time.Sleep(80 * time.Millisecond)
	close(stop)
	waitDone(t, &wg)

	// Drain any queued work into the permanent member.
	time.Sleep(20 * time.Millisecond)
	if delivered.Load() == 0 {
		t.Fatal("queue group delivered nothing")
	}
	if delivered.Load() > published.Load() {
		t.Fatalf("delivered %d > published %d", delivered.Load(), published.Load())
	}
}

// TestStressWildcardChurn races wildcard subscribe/unsubscribe (which mutate the
// copy-on-write wildcard list) against publishers hitting matching subjects.
func TestStressWildcardChurn(t *testing.T) {
	t.Parallel()
	b := newBus(t)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	patterns := []string{"a.*", "a.b.>", "*.b.c", ">"}
	for _, p := range patterns {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub, err := b.Subscribe(p, func(context.Context, bus.Message) (bus.Message, error) {
					return bus.Message{}, nil
				})
				if err != nil {
					t.Errorf("subscribe %q: %v", p, err)
					return
				}
				sub.Unsubscribe()
			}
		}()
	}

	subjects := []string{"a.b", "a.b.c", "a.b.c.d", "x.b.c", "z"}
	for _, s := range subjects {
		s := s
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for {
				select {
				case <-stop:
					return
				default:
					_ = b.Publish(ctx, pub(s, []byte("x")))
				}
			}
		}()
	}

	time.Sleep(80 * time.Millisecond)
	close(stop)
	waitDone(t, &wg)
}

// TestStressRequestResponderChurn fires requests while responders churn. Every
// successful reply must match its request (no crossed replies); other outcomes
// (no responders / timeout) are acceptable transient states.
func TestStressRequestResponderChurn(t *testing.T) {
	t.Parallel()
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 32, DefaultRequestTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { b.Close() })

	const subject = "stress.rr.churn"
	echo := func(_ context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: msg.Data}, nil
	}
	// Keep one permanent responder so most requests succeed.
	perm, err := b.Subscribe(subject, echo)
	if err != nil {
		t.Fatal(err)
	}
	defer perm.Unsubscribe()

	stop := make(chan struct{})
	var churnWG, reqWG sync.WaitGroup

	// Responder churn runs until the bounded requesters finish.
	for range 4 {
		churnWG.Add(1)
		go func() {
			defer churnWG.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub, err := b.Subscribe(subject, echo)
				if err != nil {
					if errors.Is(err, bus.ErrClosed) {
						return
					}
					t.Errorf("subscribe: %v", err)
					return
				}
				sub.Unsubscribe()
			}
		}()
	}

	// Requesters (bounded). The permanent responder guarantees a reply, so a
	// successful reply must always match its request.
	for w := range 8 {
		w := w
		reqWG.Add(1)
		go func() {
			defer reqWG.Done()
			for i := range stressStep {
				payload := make([]byte, 8)
				binary.LittleEndian.PutUint64(payload, uint64(w)<<32|uint64(i))
				reply, err := b.Request(context.Background(), pub(subject, payload))
				if err != nil {
					if errors.Is(err, bus.ErrNoResponders) || errors.Is(err, bus.ErrTimeout) || errors.Is(err, bus.ErrClosed) {
						continue
					}
					t.Errorf("request: %v", err)
					return
				}
				if len(reply.Data) != 8 || binary.LittleEndian.Uint64(reply.Data) != binary.LittleEndian.Uint64(payload) {
					t.Errorf("crossed reply: sent %x got %x", payload, reply.Data)
					return
				}
			}
		}()
	}

	waitDone(t, &reqWG)
	close(stop)
	churnWG.Wait()
}

// TestStressPerPublisherOrdering proves per-subscription FIFO: with a single
// subscriber and many concurrent publishers, each publisher's messages arrive
// in the order that publisher sent them.
func TestStressPerPublisherOrdering(t *testing.T) {
	t.Parallel()
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 128})
	t.Cleanup(func() { b.Close() })

	const publishers = 8
	const perPub = 400

	last := make([]int64, publishers)
	for i := range last {
		last[i] = -1
	}
	var ordering atomic.Bool // set true on violation
	var wg sync.WaitGroup
	wg.Add(publishers * perPub)

	_, err := b.Subscribe("order.subj", func(_ context.Context, msg bus.Message) (bus.Message, error) {
		defer wg.Done()
		p := binary.LittleEndian.Uint32(msg.Data[0:4])
		seq := int64(binary.LittleEndian.Uint32(msg.Data[4:8]))
		if seq <= last[p] {
			ordering.Store(true)
		}
		last[p] = seq
		return bus.Message{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var pwg sync.WaitGroup
	for p := range publishers {
		p := p
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			ctx := context.Background()
			for s := range perPub {
				data := make([]byte, 8)
				binary.LittleEndian.PutUint32(data[0:4], uint32(p))
				binary.LittleEndian.PutUint32(data[4:8], uint32(s))
				if err := b.Publish(ctx, bus.Message{Subject: "order.subj", Data: data}); err != nil {
					t.Errorf("publish: %v", err)
					return
				}
			}
		}()
	}
	pwg.Wait()
	waitDone(t, &wg)

	if ordering.Load() {
		t.Fatal("per-publisher FIFO ordering violated")
	}
}

// TestStressBackpressureManyPublishers points many time-bounded publishers at a
// slow single-slot subscriber. Every publish must return promptly (enqueued or
// deadline), and the process must not deadlock.
func TestStressBackpressureManyPublishers(t *testing.T) {
	t.Parallel()
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 1})
	t.Cleanup(func() { b.Close() })

	if _, err := b.Subscribe("bp.subj", func(context.Context, bus.Message) (bus.Message, error) {
		time.Sleep(200 * time.Microsecond)
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	var ok, timedOut atomic.Int64
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				err := b.Publish(ctx, pub("bp.subj", []byte("x")))
				cancel()
				switch {
				case err == nil:
					ok.Add(1)
				case errors.Is(err, context.DeadlineExceeded):
					timedOut.Add(1)
				default:
					t.Errorf("unexpected publish error: %v", err)
					return
				}
			}
		}()
	}
	waitDone(t, &wg)

	if ok.Load() == 0 {
		t.Fatal("no publishes succeeded under backpressure")
	}
	t.Logf("backpressure: %d enqueued, %d timed out", ok.Load(), timedOut.Load())
}

// TestStressCloseUnderLoad calls Close (concurrently, several times) while
// publishers, requesters, and subscribers are all in flight. Every operation
// must return cleanly (success or ErrClosed) and Close must not deadlock.
func TestStressCloseUnderLoad(t *testing.T) {
	t.Parallel()
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 8})

	if _, err := b.Subscribe("load.subj", func(_ context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: msg.Data}, nil
	}); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	okErr := func(err error) bool {
		return err == nil || errors.Is(err, bus.ErrClosed) || errors.Is(err, bus.ErrTimeout) ||
			errors.Is(err, bus.ErrNoResponders) || errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded)
	}

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if err := b.Publish(ctx, pub("load.subj", []byte("x"))); !okErr(err) {
					t.Errorf("publish: %v", err)
					return
				}
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				_, err := b.Request(ctx, pub("load.subj", []byte("y")))
				cancel()
				if !okErr(err) {
					t.Errorf("request: %v", err)
					return
				}
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub, err := b.Subscribe("load.subj", func(context.Context, bus.Message) (bus.Message, error) {
					return bus.Message{}, nil
				})
				if err != nil {
					if !okErr(err) {
						t.Errorf("subscribe: %v", err)
					}
					continue
				}
				sub.Unsubscribe()
			}
		}()
	}

	// Let the load build, then close from several goroutines at once.
	time.Sleep(30 * time.Millisecond)
	var closeWG sync.WaitGroup
	for range 4 {
		closeWG.Add(1)
		go func() {
			defer closeWG.Done()
			if err := b.Close(); err != nil {
				t.Errorf("close: %v", err)
			}
		}()
	}
	closeWG.Wait()
	close(stop)
	waitDone(t, &wg)
}

// TestStressDrainUnderLoad races Drain against active publishers. Drain must
// return (either draining cleanly or abandoning on deadline) without hanging.
func TestStressDrainUnderLoad(t *testing.T) {
	t.Parallel()
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 16})

	var handled atomic.Int64
	if _, err := b.Subscribe("drain.load", func(context.Context, bus.Message) (bus.Message, error) {
		handled.Add(1)
		return bus.Message{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for {
				select {
				case <-stop:
					return
				default:
					_ = b.Publish(ctx, pub("drain.load", []byte("x")))
				}
			}
		}()
	}

	time.Sleep(30 * time.Millisecond)
	drainErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		drainErr <- b.Drain(ctx)
	}()

	select {
	case err := <-drainErr:
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("drain returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Drain did not return under load")
	}
	close(stop)
	waitDone(t, &wg)
}

// TestStressConcurrentUnsubscribeSameHandle hits one subscription's Unsubscribe
// from many goroutines. The once/CAS guard must make it exactly-once and never
// panic (e.g. double-close of the stop channel).
func TestStressConcurrentUnsubscribeSameHandle(t *testing.T) {
	t.Parallel()
	b := newBus(t)

	for range 200 {
		sub, err := b.Subscribe("multi.unsub", func(context.Context, bus.Message) (bus.Message, error) {
			return bus.Message{}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := sub.Unsubscribe(); err != nil {
					t.Errorf("unsubscribe: %v", err)
				}
			}()
		}
		wg.Wait()
	}
}

// TestStressMixedWorkload runs pub/sub, queue groups, wildcards, and
// request/reply simultaneously, then drains. It is a broad race-detector sweep
// over all delivery paths at once.
func TestStressMixedWorkload(t *testing.T) {
	t.Parallel()
	b := bus.NewLocal(bus.LocalConfig{BufferSize: 64, DefaultRequestTimeout: 100 * time.Millisecond})

	noop := func(context.Context, bus.Message) (bus.Message, error) { return bus.Message{}, nil }
	echo := func(_ context.Context, msg bus.Message) (bus.Message, error) { return bus.Message{Data: msg.Data}, nil }

	for _, s := range []string{"mix.a", "mix.b"} {
		if _, err := b.Subscribe(s, noop); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Subscribe("mix.>", noop); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := b.QueueSubscribe("mix.q", "g", noop); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Subscribe("mix.rr", echo); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	spawn := func(fn func(ctx context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for {
				select {
				case <-stop:
					return
				default:
					fn(ctx)
				}
			}
		}()
	}

	for range 3 {
		spawn(func(ctx context.Context) { _ = b.Publish(ctx, pub("mix.a", []byte("x"))) })
		spawn(func(ctx context.Context) { _ = b.Publish(ctx, pub("mix.b.c", []byte("x"))) })
		spawn(func(ctx context.Context) { _ = b.Publish(ctx, pub("mix.q", []byte("x"))) })
		spawn(func(ctx context.Context) {
			rctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			defer cancel()
			_, _ = b.Request(rctx, pub("mix.rr", []byte("ping")))
		})
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	waitDone(t, &wg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.Drain(ctx); err != nil {
		t.Fatalf("drain after mixed workload: %v", err)
	}
}
