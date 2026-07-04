package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/InTacht/xqua-go/pkg/bus"
	"github.com/InTacht/xqua-go/pkg/runtime"
)

// Subjects storage exposes on the bus. Compute talks only through these —
// never by holding a pointer to Storage.
const (
	SubjectGet = "storage.get"
	SubjectPut = "storage.put"
	QueueName  = "storage"
)

type getReq struct {
	Key string `json:"key"`
}

type getRes struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Found bool   `json:"found"`
}

type putReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type putRes struct {
	Key string `json:"key"`
	OK  bool   `json:"ok"`
}

// Storage is a runtime.Unit that owns in-memory state and serves it over the bus.
type Storage struct {
	bus  bus.Bus
	log  runtime.Logger
	subs []bus.Subscription
	done chan struct{}

	mu   sync.RWMutex
	data map[string]string

	closedMu sync.Mutex
	closed   bool
}

// New builds the storage unit. It takes only the bus and a logger, so this
// package never imports the application context — main narrows the context at
// the registration site.
func New(b bus.Bus, log runtime.Logger) *Storage {
	return &Storage{
		bus:  b,
		log:  log,
		done: make(chan struct{}),
		data: map[string]string{"greeting": "hello from storage"},
	}
}

func (s *Storage) Name() string { return "storage" }

func (s *Storage) Serve(opts runtime.ServeOptions) error {
	getSub, err := s.bus.QueueSubscribe(SubjectGet, QueueName, s.handleGet)
	if err != nil {
		return fmt.Errorf("storage: subscribe get: %w", err)
	}
	putSub, err := s.bus.QueueSubscribe(SubjectPut, QueueName, s.handlePut)
	if err != nil {
		_ = getSub.Unsubscribe()
		return fmt.Errorf("storage: subscribe put: %w", err)
	}
	s.subs = []bus.Subscription{getSub, putSub}

	if opts.OnReady != nil {
		opts.OnReady()
	}
	s.log.Info("storage ready", SubjectGet, SubjectPut)

	<-s.done
	return nil
}

func (s *Storage) Shutdown(ctx context.Context) error {
	_ = ctx
	s.closedMu.Lock()
	if s.closed {
		s.closedMu.Unlock()
		return nil
	}
	s.closed = true
	s.closedMu.Unlock()

	// Unsubscribe is idempotent and a no-op once the bus is draining/closed.
	for _, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			s.log.Error(err, "unsubscribe failed")
		}
	}
	close(s.done)
	return nil
}

func (s *Storage) handleGet(ctx context.Context, msg bus.Message) (bus.Message, error) {
	_ = ctx
	var req getReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return bus.Message{}, err
	}
	s.mu.RLock()
	v, ok := s.data[req.Key]
	s.mu.RUnlock()
	data, err := json.Marshal(getRes{Key: req.Key, Value: v, Found: ok})
	return bus.Message{Data: data}, err
}

func (s *Storage) handlePut(ctx context.Context, msg bus.Message) (bus.Message, error) {
	_ = ctx
	var req putReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return bus.Message{}, err
	}
	s.mu.Lock()
	s.data[req.Key] = req.Value
	s.mu.Unlock()
	s.log.Info("stored", req.Key)
	data, err := json.Marshal(putRes{Key: req.Key, OK: true})
	return bus.Message{Data: data}, err
}
