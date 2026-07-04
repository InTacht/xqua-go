// Package bus is a headless inter-unit message bus.
//
// Units never call each other by Go pointers as the public contract; they talk
// through a Bus. The same API is intended for a future cluster backend; this
// package currently ships a same-process implementation only (NewLocal).
//
// The bus is standalone: it is not created by runtime. The caller owns the bus —
// typically constructing it in main and closing over it from unit factories.
//
// # Message model
//
// Message is the single currency: Publish takes a Message and Request takes and
// returns a Message, so headers (correlation IDs) ride along naturally. Data is
// opaque []byte. Subjects may be exact ("a.b.c") or use wildcards, where "*"
// matches exactly one token and ">" matches one or more trailing tokens.
//
// Zero-copy contract: the bus does not copy Data or Headers on delivery.
// Publishers must not mutate a Message after handing it off, and handlers must
// treat received Data/Headers as read-only.
//
// # Operations
//
//   - Publish fans out to every matching non-queue subscriber and delivers to
//     exactly one member per matching queue group (round-robin).
//   - Subscribe registers a fan-out handler.
//   - QueueSubscribe registers a competing consumer in a named queue group.
//     Scale consumption by adding more queue subscriptions — each subscription
//     is one FIFO worker; there is no per-subscription worker knob.
//   - Request creates an ephemeral inbox, delivers with Reply set, and waits for
//     one response. It returns ErrNoResponders, ErrTimeout, ctx.Err(), or the
//     responder's own error.
//
// # Ordering, concurrency, and backpressure
//
// Each subscription owns a bounded mailbox drained by a single worker
// goroutine. This gives per-subscription FIFO ordering, bounded memory, and
// panic isolation: a handler panic is recovered and reported via
// LocalConfig.OnError (pub/sub) or returned to the requester (request/reply),
// never crashing the process. When a mailbox is full, Publish/Request block
// (backpressure) until space frees, the context is done, or the bus stops.
//
// The registry is a concurrent map (subject -> entry) with per-entry immutable
// snapshots behind an atomic pointer. Publish and Request read it with zero
// locks and zero shared writes and pick queue members with an atomic cursor, so
// concurrent publishers and requesters do not serialize on anything; only
// Subscribe/Unsubscribe copy-on-write the one affected subject (never the whole
// registry), which keeps high-frequency ephemeral subscriptions cheap. Pub/sub
// handlers receive a context carrying the publisher's values but not its
// cancellation; request handlers receive a context linked to the requester so a
// caller timeout cancels the handler's work.
//
// Requests carry the reply rendezvous inside the delivery itself — there is no
// shared pending-request table and no lock on the request path. Subscriptions
// (their struct, mailbox channel, and worker) are pooled and each delivery is
// generation-tagged, so churn does not repeatedly allocate the mailbox and a
// straggling publisher can never route a message to a recycled subscription.
//
// # Shutdown
//
// Close stops the bus immediately: it rejects new operations, unblocks in-flight
// requests with ErrClosed, and abandons queued messages (a message already being
// handled still completes). Drain is the graceful path: it stops intake, lets
// workers finish everything already queued (bounded by the given context), then
// closes. Unsubscribe is idempotent and a no-op once the bus is closed.
//
// # Errors
//
// Sentinel errors (ErrClosed, ErrNoResponders, ErrTimeout) are plain package
// errors so bus stays independent of pkg/errors. Units map them to catalog
// entries at their boundary.
//
// Import path: github.com/InTacht/xqua-go/pkg/bus
package bus
