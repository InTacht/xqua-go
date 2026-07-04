package bus

import (
	"context"
	"errors"
)

// Sentinel errors returned by Bus operations.
var (
	// ErrClosed is returned when an operation is attempted on a closed or
	// draining bus, and to unblock in-flight requests when the bus stops.
	ErrClosed = errors.New("bus: closed")

	// ErrNoResponders is returned by Request when no subscriber matches the
	// subject at the moment the request is made.
	ErrNoResponders = errors.New("bus: no responders")

	// ErrTimeout is returned by Request when the deadline expires before a
	// reply arrives (either the caller's context deadline or the bus-wide
	// LocalConfig.DefaultRequestTimeout).
	ErrTimeout = errors.New("bus: timeout")
)

// Message is a single bus payload.
//
// Data is opaque bytes so the wire shape stays language-agnostic; use
// EncodeJSON / DecodeJSON when a struct payload is convenient. Headers carry
// out-of-band metadata such as correlation IDs.
//
// Zero-copy contract: the bus does not copy Data or Headers on delivery. A
// publisher must not mutate a Message's Data or Headers after passing it to
// Publish or Request, and a handler must treat the received Data and Headers as
// read-only. Reply messages a handler returns are owned by that handler.
type Message struct {
	Subject string
	Data    []byte
	Headers map[string]string
	// Reply is the inbox subject set by the bus on request delivery. It is
	// empty for pure pub/sub. Handlers may read it but do not need to reply
	// manually — the Message returned from the handler is sent as the reply.
	Reply string
}

// Handler processes a delivered message.
//
// The returned Message is used only for request/reply: when the delivered
// message has a non-empty Reply, the handler's returned Message (its Data and
// Headers) is sent back to the requester. For pure pub/sub the returned Message
// is ignored — return the zero Message.
//
// A returned error is delivered to the requester for request/reply; for pub/sub
// it is reported to LocalConfig.OnError. A panic in the handler is recovered and
// treated the same way as a returned error.
type Handler func(ctx context.Context, msg Message) (Message, error)

// Bus is the inter-unit message bus. Implementations must be safe for
// concurrent use.
type Bus interface {
	// Publish delivers msg to every matching fan-out subscriber and to exactly
	// one subscriber per matching queue group. Delivery is asynchronous; when a
	// subscriber's mailbox is full Publish blocks (backpressure) until space is
	// available, the context is done, or the bus stops.
	Publish(ctx context.Context, msg Message) error

	// Request delivers msg with an ephemeral reply inbox and waits for one
	// response. It returns ErrNoResponders when nothing matches the subject,
	// ErrTimeout when the deadline expires, ctx.Err() on cancellation, or the
	// handler's error when the responder fails. The reply Message carries the
	// responder's Data and Headers.
	Request(ctx context.Context, msg Message) (Message, error)

	// Subscribe registers a fan-out handler: every message matching subject is
	// delivered to h. Subjects may be exact ("a.b.c") or use wildcards where
	// "*" matches one token and ">" matches one or more trailing tokens.
	Subscribe(subject string, h Handler) (Subscription, error)

	// QueueSubscribe registers a competing consumer in queue. Messages matching
	// subject are delivered to exactly one subscriber in the group. Run more
	// queue subscriptions to scale consumption; each subscription is a single
	// FIFO worker.
	QueueSubscribe(subject, queue string, h Handler) (Subscription, error)

	// Drain stops accepting new work, waits for already-queued messages to be
	// handled (bounded by ctx), then closes the bus. Use it for graceful
	// shutdown. If ctx expires first, remaining work is abandoned like Close.
	Drain(ctx context.Context) error

	// Close stops the bus immediately: it rejects new operations, unblocks
	// in-flight requests with ErrClosed, and abandons queued messages (the
	// message a worker is already handling still completes). Close is
	// idempotent.
	Close() error
}

// Subscription is a live handler registration.
type Subscription interface {
	// Unsubscribe removes the subscription and stops its worker. It is
	// idempotent and a no-op (nil error) once the bus is closed.
	Unsubscribe() error
}
