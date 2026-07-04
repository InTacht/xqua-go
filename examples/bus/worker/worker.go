package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/InTacht/xqua-go/pkg/bus"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

const (
	SubjectWork = "demo.work"
	QueueName   = "workers"
)

// Worker is a runtime.Unit that competes for work on demo.work.
// Register several workers with different IDs in the same queue to see
// competing-consumer load balancing inside one process.
type Worker struct {
	id   string
	bus  bus.Bus
	log  runtime.Logger
	sub  bus.Subscription
	done chan struct{}

	mu     sync.Mutex
	closed bool
}

// New builds a named worker for the shared queue group. It takes only the
// dependencies it uses (the bus and a logger), so this package never imports
// the application context — main narrows the context at the registration site.
func New(id string, b bus.Bus, log runtime.Logger) *Worker {
	return &Worker{
		id:   id,
		bus:  b,
		log:  log,
		done: make(chan struct{}),
	}
}

func (w *Worker) Name() string { return "worker-" + w.id }

func (w *Worker) Serve(opts runtime.ServeOptions) error {
	sub, err := w.bus.QueueSubscribe(SubjectWork, QueueName, w.handle)
	if err != nil {
		return fmt.Errorf("worker %s: subscribe: %w", w.id, err)
	}
	w.sub = sub

	if opts.OnReady != nil {
		opts.OnReady()
	}
	w.log.Info("worker ready", w.id, SubjectWork, QueueName)

	<-w.done
	return nil
}

func (w *Worker) Shutdown(ctx context.Context) error {
	_ = ctx
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	// Unsubscribe is a no-op once the bus is draining/closed, so no ErrClosed
	// special-casing is needed.
	if w.sub != nil {
		if err := w.sub.Unsubscribe(); err != nil {
			w.log.Error(err, "unsubscribe failed")
		}
	}
	close(w.done)
	return nil
}

func (w *Worker) handle(ctx context.Context, msg bus.Message) (bus.Message, error) {
	_ = ctx
	q := strings.TrimSpace(string(msg.Data))
	if q == "" {
		q = "(empty)"
	}
	// Small delay so concurrent curls tend to land on different workers.
	time.Sleep(50 * time.Millisecond)
	w.log.Info("processing work", w.id, q)
	return bus.Message{Data: []byte(fmt.Sprintf("worker=%s processed: %s", w.id, q))}, nil
}
