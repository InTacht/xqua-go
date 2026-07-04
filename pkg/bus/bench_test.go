package bus_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/InTacht/xqua-go/pkg/bus"
)

// BenchmarkPublishFanout measures publish throughput to N fan-out subscribers
// that each drain their own mailbox.
func BenchmarkPublishFanout(b *testing.B) {
	for _, subs := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("subs=%d", subs), func(b *testing.B) {
			bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 1024})
			defer bus0.Close()

			var wg sync.WaitGroup
			h := func(ctx context.Context, msg bus.Message) (bus.Message, error) {
				wg.Done()
				return bus.Message{}, nil
			}
			for range subs {
				if _, err := bus0.Subscribe("bench.fanout", h); err != nil {
					b.Fatal(err)
				}
			}

			ctx := context.Background()
			msg := bus.Message{Subject: "bench.fanout", Data: []byte("payload")}
			b.ResetTimer()
			for range b.N {
				wg.Add(subs)
				if err := bus0.Publish(ctx, msg); err != nil {
					b.Fatal(err)
				}
			}
			wg.Wait()
		})
	}
}

// BenchmarkQueueGroup measures competing-consumer throughput across a queue
// group of N workers.
func BenchmarkQueueGroup(b *testing.B) {
	for _, workers := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 1024})
			defer bus0.Close()

			var wg sync.WaitGroup
			h := func(ctx context.Context, msg bus.Message) (bus.Message, error) {
				wg.Done()
				return bus.Message{}, nil
			}
			for range workers {
				if _, err := bus0.QueueSubscribe("bench.queue", "g", h); err != nil {
					b.Fatal(err)
				}
			}

			ctx := context.Background()
			msg := bus.Message{Subject: "bench.queue", Data: []byte("payload")}
			b.ResetTimer()
			for range b.N {
				wg.Add(1)
				if err := bus0.Publish(ctx, msg); err != nil {
					b.Fatal(err)
				}
			}
			wg.Wait()
		})
	}
}

// BenchmarkRequestReply measures round-trip request/reply latency.
func BenchmarkRequestReply(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 1024})
	defer bus0.Close()

	if _, err := bus0.Subscribe("bench.rr", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: msg.Data}, nil
	}); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "bench.rr", Data: []byte("ping")}
	b.ResetTimer()
	for range b.N {
		if _, err := bus0.Request(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPublishParallelSubjects proves the concurrent registry + atomic
// round-robin design: many goroutines publishing to distinct subjects should
// not serialize on any shared lock.
func BenchmarkPublishParallelSubjects(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 4096})
	defer bus0.Close()

	const subjects = 16
	var wg sync.WaitGroup
	h := func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		wg.Done()
		return bus.Message{}, nil
	}
	// Precompute subjects so the loop measures the bus, not fmt.Sprintf.
	subj := make([]string, subjects)
	for i := range subjects {
		subj[i] = fmt.Sprintf("bench.p.%d", i)
		if _, err := bus0.Subscribe(subj[i], h); err != nil {
			b.Fatal(err)
		}
	}

	ctx := context.Background()
	data := []byte("x")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			wg.Add(1)
			if err := bus0.Publish(ctx, bus.Message{Subject: subj[i%subjects], Data: data}); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
	wg.Wait()
}

// BenchmarkRequestReplyParallel drives request/reply from many goroutines at
// once. The waiter-in-delivery design has no shared write state on the request
// path, so this should scale with GOMAXPROCS instead of serializing.
func BenchmarkRequestReplyParallel(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 1024})
	defer bus0.Close()

	if _, err := bus0.Subscribe("bench.rrp", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{Data: msg.Data}, nil
	}); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "bench.rrp", Data: []byte("ping")}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := bus0.Request(ctx, msg); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkPublishWithWildcardsPresent proves that publishing to an exact
// subject stays cheap even while wildcard subscriptions exist: the exact match
// is a lock-free map load and the wildcard scan walks the subject with no
// allocation.
func BenchmarkPublishWithWildcardsPresent(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 4096})
	defer bus0.Close()

	var wg sync.WaitGroup
	h := func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		wg.Done()
		return bus.Message{}, nil
	}
	// One exact subscriber that receives traffic, plus wildcard taps that do
	// not match the published subject (they must cost only a walk, no delivery).
	if _, err := bus0.Subscribe("bench.w.exact", h); err != nil {
		b.Fatal(err)
	}
	for _, p := range []string{"other.*", "metrics.>", "bench.other.*"} {
		if _, err := bus0.Subscribe(p, func(ctx context.Context, msg bus.Message) (bus.Message, error) {
			return bus.Message{}, nil
		}); err != nil {
			b.Fatal(err)
		}
	}

	ctx := context.Background()
	msg := bus.Message{Subject: "bench.w.exact", Data: []byte("x")}
	b.ResetTimer()
	for range b.N {
		wg.Add(1)
		if err := bus0.Publish(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}
	wg.Wait()
}

// BenchmarkPublishNoSubscribers measures the cost of a publish that matches
// nothing: a lock-free miss that must not allocate.
func BenchmarkPublishNoSubscribers(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{})
	defer bus0.Close()

	ctx := context.Background()
	msg := bus.Message{Subject: "bench.absent", Data: []byte("x")}
	b.ResetTimer()
	for range b.N {
		if err := bus0.Publish(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEphemeralChurn measures the per-session subscribe -> publish ->
// unsubscribe cycle that ephemeral workloads generate. Subscription pooling
// should keep this alloc-light and O(subs on that subject), never O(registry).
func BenchmarkEphemeralChurn(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 8})
	defer bus0.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			done := make(chan struct{})
			sub, err := bus0.Subscribe("bench.churn", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
				close(done)
				return bus.Message{}, nil
			})
			if err != nil {
				b.Fatal(err)
			}
			if err := bus0.Publish(ctx, bus.Message{Subject: "bench.churn", Data: []byte("x")}); err != nil {
				b.Fatal(err)
			}
			<-done
			if err := sub.Unsubscribe(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkEphemeralChurnWithSteadyTraffic runs the same churn while a steady
// publisher hammers a separate subject, proving churn on one subject does not
// degrade the read path of unrelated traffic.
func BenchmarkEphemeralChurnWithSteadyTraffic(b *testing.B) {
	bus0 := bus.NewLocal(bus.LocalConfig{BufferSize: 1024})
	defer bus0.Close()

	if _, err := bus0.Subscribe("bench.steady", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
		return bus.Message{}, nil
	}); err != nil {
		b.Fatal(err)
	}

	stop := make(chan struct{})
	var noise sync.WaitGroup
	noise.Add(1)
	go func() {
		defer noise.Done()
		ctx := context.Background()
		msg := bus.Message{Subject: "bench.steady", Data: []byte("n")}
		for {
			select {
			case <-stop:
				return
			default:
				_ = bus0.Publish(ctx, msg)
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sub, err := bus0.Subscribe("bench.churn2", func(ctx context.Context, msg bus.Message) (bus.Message, error) {
				return bus.Message{}, nil
			})
			if err != nil {
				b.Fatal(err)
			}
			if err := sub.Unsubscribe(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()
	close(stop)
	noise.Wait()
}
