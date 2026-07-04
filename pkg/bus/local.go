package bus

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const defaultBufferSize = 64

var (
	errSubjectRequired = errors.New("bus: subject is required")
	errQueueRequired   = errors.New("bus: queue is required")
	errHandlerRequired = errors.New("bus: handler is required")
)

// LocalConfig configures a Local bus. The zero value is valid.
type LocalConfig struct {
	// BufferSize is the per-subscription mailbox capacity. Values <= 0 use the
	// default (64). Larger buffers absorb bursts; smaller buffers apply
	// backpressure sooner and bound memory more tightly.
	BufferSize int

	// DefaultRequestTimeout caps a Request whose context carries no deadline.
	// Zero means no cap: such a request waits until a reply or bus close. A
	// non-zero value turns a silent responder into a timely ErrTimeout.
	DefaultRequestTimeout time.Duration

	// OnError observes handler errors and recovered panics on pub/sub delivery.
	// (Request/reply delivers handler errors to the requester instead.) The bus
	// has no logger of its own; wire this to your logger if you want
	// visibility. Optional.
	OnError func(msg Message, err error)
}

// Local is a same-process Bus tuned for every workload: read-dominated pub/sub,
// request/reply storms, and high-frequency ephemeral subscriptions.
//
// The registry is a concurrent map (subject -> entry) with per-entry immutable
// snapshots behind an atomic pointer, so Publish/Request read it with zero locks
// and zero shared writes, while Subscribe/Unsubscribe copy-on-write only the one
// affected subject. Subscriptions (struct, mailbox channel, worker goroutine)
// are pooled so churn does not allocate the mailbox repeatedly. Each delivery is
// generation-tagged so a straggling publisher can never route a message to a
// recycled subscription.
type Local struct {
	cfg LocalConfig

	closed   atomic.Bool
	draining atomic.Bool
	closedCh chan struct{} // closed by Close; unblocks requests and blocked publishers

	inboxSeq atomic.Uint64

	index sync.Map // subject string -> *subjectEntry

	wildMu sync.Mutex
	wild   atomic.Pointer[[]*subjectEntry] // copy-on-write list of wildcard entries

	// subs tracks active subscriptions so Close/Drain can reach every worker.
	// Keyed by the *subscription pointer: pointer-to-interface boxing is
	// allocation-free, unlike the integer key it replaced, so churn stays lean.
	subs sync.Map // *subscription -> struct{} (active only)
	wg   sync.WaitGroup

	subPool    sync.Pool
	waiterPool sync.Pool
}

// subjectEntry holds all subscriptions registered under one pattern. Its state
// pointer is swapped copy-on-write; readers load it locklessly.
type subjectEntry struct {
	pattern string
	isWild  bool
	tokens  []string // split pattern, set once for wildcard entries

	mu    sync.Mutex // serializes writers to this subject only
	dead  bool       // entry has been removed from the index
	state atomic.Pointer[subState]
}

// subState is an immutable snapshot of the subscribers on a subject.
type subState struct {
	fanout []subRef
	queues map[string]*queueGroup
}

type queueGroup struct {
	subs []subRef
	next atomic.Uint64 // round-robin cursor
}

// subRef is a generation-stamped reference captured in a registry snapshot. It
// carries everything the publish path needs so it never dereferences the live
// subscription (which may be concurrently recycled).
type subRef struct {
	sub     *subscription
	gen     uint32
	stop    chan struct{}
	mailbox chan delivery
}

type subscription struct {
	bus *Local

	// Set on each activation, before the worker starts, immutable during the
	// generation. gen is atomic because Unsubscribe reads it to guard stale
	// handles.
	gen        atomic.Uint32
	pattern    string
	tokens     []string
	isWild     bool
	queue      string
	handler    Handler
	mailbox    chan delivery // reused across activations
	stop       chan struct{} // fresh per activation
	stopClosed atomic.Bool
	once       atomic.Bool // per-activation Unsubscribe guard
}

type delivery struct {
	ctx    context.Context
	msg    Message
	waiter *waiter
	gen    uint32 // subscription generation this delivery targets
	reqGen uint64 // waiter generation (request/reply only)
}

const (
	stArmed uint64 = 1
	stDone  uint64 = 2
)

// waiter is a pooled request/reply rendezvous. state packs a generation in the
// high bits and armed/done in the low bits so a slow responder from a previous
// request can never complete a recycled waiter.
//
// inbox is assigned once at creation and reused across checkouts. Replies route
// by the waiter pointer carried in the delivery, never by subject, so the inbox
// is purely a cosmetic Reply value; a stable per-waiter string keeps the
// request path allocation-free while staying unique among in-flight requests
// (a waiter serves one request at a time).
type waiter struct {
	ch    chan reqResult
	inbox string
	gen   uint64
	state atomic.Uint64
}

type reqResult struct {
	msg Message
	err error
}

// subHandle is the Subscription returned to callers. It pins the generation so
// a stale handle cannot unsubscribe a recycled subscription.
type subHandle struct {
	sub *subscription
	gen uint32
}

func (h *subHandle) Unsubscribe() error {
	if h == nil || h.sub == nil {
		return nil
	}
	return h.sub.bus.unsubscribe(h.sub, h.gen)
}

// NewLocal returns a ready-to-use same-process bus. The caller owns Close.
func NewLocal(cfg LocalConfig) *Local {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = defaultBufferSize
	}
	return &Local{
		cfg:      cfg,
		closedCh: make(chan struct{}),
	}
}

// Publish implements Bus.
func (b *Local) Publish(ctx context.Context, msg Message) error {
	if msg.Subject == "" {
		return errSubjectRequired
	}
	if b.closed.Load() || b.draining.Load() {
		return ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Handlers see the publisher's values but not its cancellation; skip the
	// WithoutCancel wrapper entirely when the context cannot be cancelled.
	dctx := ctx
	if ctx.Done() != nil {
		dctx = context.WithoutCancel(ctx)
	}

	// Fuse matching and delivery: enqueue straight from the immutable snapshot
	// so no target slice is ever materialized. Fan-out of any size stays
	// allocation-free (no [8]subRef stack buffer to overflow onto the heap).
	if v, ok := b.index.Load(msg.Subject); ok {
		if e := v.(*subjectEntry); !e.isWild {
			if st := e.state.Load(); st != nil {
				if err := b.deliverState(st, dctx, ctx, msg); err != nil {
					return err
				}
			}
		}
	}
	if wp := b.wild.Load(); wp != nil {
		for _, e := range *wp {
			if !matchPattern(e.tokens, msg.Subject) {
				continue
			}
			if st := e.state.Load(); st != nil {
				if err := b.deliverState(st, dctx, ctx, msg); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// deliverState enqueues msg to every fan-out subscriber and one member per queue
// group of an immutable snapshot. It allocates nothing.
func (b *Local) deliverState(st *subState, dctx, blockCtx context.Context, msg Message) error {
	for i := range st.fanout {
		ref := st.fanout[i]
		if _, err := b.enqueue(ref, blockCtx, delivery{ctx: dctx, msg: msg, gen: ref.gen}); err != nil {
			return err
		}
	}
	for _, qg := range st.queues {
		n := uint64(len(qg.subs))
		if n == 0 {
			continue
		}
		ref := qg.subs[(qg.next.Add(1)-1)%n]
		if _, err := b.enqueue(ref, blockCtx, delivery{ctx: dctx, msg: msg, gen: ref.gen}); err != nil {
			return err
		}
	}
	return nil
}

// Request implements Bus.
func (b *Local) Request(ctx context.Context, msg Message) (Message, error) {
	if msg.Subject == "" {
		return Message{}, errSubjectRequired
	}
	if b.closed.Load() || b.draining.Load() {
		return Message{}, ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	waitCtx := ctx
	if b.cfg.DefaultRequestTimeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			waitCtx, cancel = context.WithTimeout(ctx, b.cfg.DefaultRequestTimeout)
			defer cancel()
		}
	}

	w := b.getWaiter()
	g := w.gen
	req := msg
	req.Reply = w.inbox

	var buf [8]subRef
	delivered := false
	for attempt := 0; attempt < 4 && !delivered; attempt++ {
		refs := b.collect(msg.Subject, buf[:0])
		if len(refs) == 0 {
			b.putWaiter(w)
			return Message{}, ErrNoResponders
		}
		for i := range refs {
			d := delivery{ctx: waitCtx, msg: req, waiter: w, gen: refs[i].gen, reqGen: g}
			ok, err := b.enqueue(refs[i], waitCtx, d)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					err = ErrTimeout
				}
				return b.finishReq(w, g, Message{}, err)
			}
			if ok {
				delivered = true
			}
		}
	}
	if !delivered {
		b.putWaiter(w)
		return Message{}, ErrNoResponders
	}

	select {
	case res := <-w.ch:
		b.putWaiter(w)
		// If the wait deadline/cancellation fired in the same instant the
		// responder replied with an error, prefer the canonical bus error:
		// the handler shares waitCtx, so its error is just the timeout echoed.
		if res.err != nil {
			if cerr := waitCtx.Err(); cerr != nil {
				if errors.Is(cerr, context.DeadlineExceeded) {
					return Message{}, ErrTimeout
				}
				return Message{}, cerr
			}
		}
		return res.msg, res.err
	case <-b.closedCh:
		return b.finishReq(w, g, Message{}, ErrClosed)
	case <-waitCtx.Done():
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			return b.finishReq(w, g, Message{}, ErrTimeout)
		}
		return b.finishReq(w, g, Message{}, waitCtx.Err())
	}
}

// Subscribe implements Bus.
func (b *Local) Subscribe(subject string, h Handler) (Subscription, error) {
	return b.addSub(subject, "", h)
}

// QueueSubscribe implements Bus.
func (b *Local) QueueSubscribe(subject, queue string, h Handler) (Subscription, error) {
	if queue == "" {
		return nil, errQueueRequired
	}
	return b.addSub(subject, queue, h)
}

// Drain implements Bus.
func (b *Local) Drain(ctx context.Context) error {
	if b.closed.Load() {
		return nil
	}
	b.draining.Store(true)
	if ctx == nil {
		ctx = context.Background()
	}

	b.subs.Range(func(k, _ any) bool {
		b.closeStop(k.(*subscription))
		return true
	})

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return b.Close()
	case <-ctx.Done():
		b.Close()
		return ctx.Err()
	}
}

// Close implements Bus.
func (b *Local) Close() error {
	if b.closed.Swap(true) {
		return nil
	}
	close(b.closedCh)
	b.subs.Range(func(k, _ any) bool {
		b.closeStop(k.(*subscription))
		return true
	})
	b.wg.Wait()
	b.wild.Store(nil)
	return nil
}

func (b *Local) nextInbox() string {
	n := b.inboxSeq.Add(1)
	var buf [24]byte
	p := append(buf[:0], "_INBOX."...)
	p = strconv.AppendUint(p, n, 10)
	return string(p)
}

// --- registration -----------------------------------------------------------

func (b *Local) addSub(pattern, queue string, h Handler) (Subscription, error) {
	if pattern == "" {
		return nil, errSubjectRequired
	}
	if h == nil {
		return nil, errHandlerRequired
	}
	if b.closed.Load() || b.draining.Load() {
		return nil, ErrClosed
	}

	s := b.getSub()
	s.pattern = pattern
	s.queue = queue
	s.handler = h
	s.isWild = isWildcard(pattern)
	if s.isWild {
		s.tokens = strings.Split(pattern, ".")
	} else {
		s.tokens = nil
	}
	gen := s.gen.Add(1)
	s.stop = make(chan struct{})
	s.stopClosed.Store(false)
	s.once.Store(false)

	ref := subRef{sub: s, gen: gen, stop: s.stop, mailbox: s.mailbox}
	b.register(pattern, s.isWild, s.tokens, queue, ref)

	b.subs.Store(s, struct{}{})
	// Re-check after publishing to the lifecycle set so a concurrent
	// Close/Drain cannot miss this subscription.
	if b.closed.Load() || b.draining.Load() {
		b.subs.Delete(s)
		b.deregister(pattern, s.isWild, queue, ref)
		b.recycleSub(s)
		return nil, ErrClosed
	}

	b.wg.Add(1)
	go runWorker(s)
	return &subHandle{sub: s, gen: gen}, nil
}

func (b *Local) unsubscribe(s *subscription, gen uint32) error {
	if s.gen.Load() != gen {
		return nil // stale handle: subscription already recycled
	}
	if !s.once.CompareAndSwap(false, true) {
		return nil
	}
	ref := subRef{sub: s, gen: gen, stop: s.stop, mailbox: s.mailbox}
	b.deregister(s.pattern, s.isWild, s.queue, ref)
	b.subs.Delete(s)
	b.closeStop(s)
	return nil
}

func (b *Local) register(pattern string, isWild bool, tokens []string, queue string, ref subRef) {
	for {
		// Load first: an existing entry (the common case under any sustained
		// traffic) costs a lock-free lookup with no key boxing and no wasted
		// subjectEntry. Only a genuine miss pays for LoadOrStore.
		v, ok := b.index.Load(pattern)
		if !ok {
			v, _ = b.index.LoadOrStore(pattern, &subjectEntry{pattern: pattern, isWild: isWild})
		}
		e := v.(*subjectEntry)
		e.mu.Lock()
		if e.dead {
			e.mu.Unlock()
			continue // entry being removed; retry with a fresh one
		}
		old := e.state.Load()
		if isWild && e.tokens == nil {
			e.tokens = tokens
		}
		e.state.Store(cloneAdd(old, queue, ref))
		firstWild := isWild && old == nil
		e.mu.Unlock()
		if firstWild {
			b.addWild(e)
		}
		return
	}
}

func (b *Local) deregister(pattern string, isWild bool, queue string, ref subRef) {
	v, ok := b.index.Load(pattern)
	if !ok {
		return
	}
	e := v.(*subjectEntry)
	e.mu.Lock()
	ns, empty := cloneRemove(e.state.Load(), queue, ref)
	if empty {
		e.dead = true
		e.state.Store(nil)
		b.index.Delete(pattern)
		e.mu.Unlock()
		if isWild {
			b.removeWild(e)
		}
		return
	}
	e.state.Store(ns)
	e.mu.Unlock()
}

func cloneAdd(old *subState, queue string, ref subRef) *subState {
	if old == nil {
		ns := &subState{}
		if queue == "" {
			ns.fanout = []subRef{ref}
		} else {
			ns.queues = map[string]*queueGroup{queue: {subs: []subRef{ref}}}
		}
		return ns
	}
	ns := &subState{fanout: old.fanout, queues: old.queues}
	if queue == "" {
		ns.fanout = append(append(make([]subRef, 0, len(old.fanout)+1), old.fanout...), ref)
		return ns
	}
	ns.queues = make(map[string]*queueGroup, len(old.queues)+1)
	for k, v := range old.queues {
		ns.queues[k] = v
	}
	nqg := &queueGroup{}
	if oldQg := old.queues[queue]; oldQg != nil {
		nqg.subs = append(append(make([]subRef, 0, len(oldQg.subs)+1), oldQg.subs...), ref)
		nqg.next.Store(oldQg.next.Load())
	} else {
		nqg.subs = []subRef{ref}
	}
	ns.queues[queue] = nqg
	return ns
}

func cloneRemove(old *subState, queue string, ref subRef) (ns *subState, empty bool) {
	if old == nil {
		return nil, true
	}
	if queue == "" {
		// Ephemeral hot path: sole fan-out member leaves — no snapshot needed.
		if len(old.fanout) == 1 && len(old.queues) == 0 &&
			old.fanout[0].sub == ref.sub && old.fanout[0].gen == ref.gen {
			return nil, true
		}
		nf := removeRef(old.fanout, ref)
		if len(nf) == 0 && len(old.queues) == 0 {
			return nil, true
		}
		return &subState{fanout: nf, queues: old.queues}, false
	}

	oldQg := old.queues[queue]
	if oldQg == nil {
		if len(old.fanout) == 0 && len(old.queues) == 0 {
			return nil, true
		}
		return &subState{fanout: old.fanout, queues: old.queues}, false
	}
	nsubs := removeRef(oldQg.subs, ref)
	nqueues := make(map[string]*queueGroup, len(old.queues))
	for k, v := range old.queues {
		if k != queue {
			nqueues[k] = v
		}
	}
	if len(nsubs) > 0 {
		nqg := &queueGroup{subs: nsubs}
		nqg.next.Store(oldQg.next.Load())
		nqueues[queue] = nqg
	}
	if len(old.fanout) == 0 && len(nqueues) == 0 {
		return nil, true
	}
	return &subState{fanout: old.fanout, queues: nqueues}, false
}

func removeRef(refs []subRef, target subRef) []subRef {
	for i := range refs {
		if refs[i].sub == target.sub && refs[i].gen == target.gen {
			out := make([]subRef, 0, len(refs)-1)
			out = append(out, refs[:i]...)
			out = append(out, refs[i+1:]...)
			return out
		}
	}
	return refs
}

func (b *Local) addWild(e *subjectEntry) {
	b.wildMu.Lock()
	old := b.wild.Load()
	var ns []*subjectEntry
	if old != nil {
		ns = make([]*subjectEntry, 0, len(*old)+1)
		ns = append(ns, *old...)
	}
	ns = append(ns, e)
	b.wild.Store(&ns)
	b.wildMu.Unlock()
}

func (b *Local) removeWild(e *subjectEntry) {
	b.wildMu.Lock()
	old := b.wild.Load()
	if old != nil {
		ns := make([]*subjectEntry, 0, len(*old))
		for _, x := range *old {
			if x != e {
				ns = append(ns, x)
			}
		}
		b.wild.Store(&ns)
	}
	b.wildMu.Unlock()
}

// --- delivery ----------------------------------------------------------------

// collect appends the targets for subject: matching fan-out subscribers plus one
// member per matching queue group. Lock-free.
func (b *Local) collect(subject string, out []subRef) []subRef {
	if v, ok := b.index.Load(subject); ok {
		if e := v.(*subjectEntry); !e.isWild {
			if st := e.state.Load(); st != nil {
				out = appendRefs(out, st)
			}
		}
	}
	if wp := b.wild.Load(); wp != nil {
		for _, e := range *wp {
			if matchPattern(e.tokens, subject) {
				if st := e.state.Load(); st != nil {
					out = appendRefs(out, st)
				}
			}
		}
	}
	return out
}

func appendRefs(out []subRef, st *subState) []subRef {
	out = append(out, st.fanout...)
	for _, qg := range st.queues {
		n := uint64(len(qg.subs))
		if n == 0 {
			continue
		}
		idx := (qg.next.Add(1) - 1) % n
		out = append(out, qg.subs[idx])
	}
	return out
}

// enqueue delivers d to a target mailbox. It reports whether the delivery was
// accepted (false means the target was torn down). It blocks with backpressure
// when the mailbox is full, honoring blockCtx and bus close.
func (b *Local) enqueue(ref subRef, blockCtx context.Context, d delivery) (bool, error) {
	select {
	case <-ref.stop:
		return false, nil
	default:
	}
	select {
	case ref.mailbox <- d:
		return true, nil
	default:
	}
	select {
	case ref.mailbox <- d:
		return true, nil
	case <-ref.stop:
		return false, nil
	case <-blockCtx.Done():
		return false, blockCtx.Err()
	case <-b.closedCh:
		return false, ErrClosed
	}
}

func runWorker(s *subscription) {
	b := s.bus
	g := s.gen.Load()
	stop := s.stop
	mailbox := s.mailbox
	for {
		select {
		case d := <-mailbox:
			if d.gen == g {
				b.dispatch(s, d)
			}
		case <-stop:
			if b.draining.Load() {
				drainProcess(b, s, mailbox, g)
			} else {
				drainDiscard(mailbox)
			}
			if !b.closed.Load() && !b.draining.Load() {
				b.recycleSub(s)
			}
			b.wg.Done()
			return
		}
	}
}

func drainProcess(b *Local, s *subscription, mailbox chan delivery, g uint32) {
	for {
		select {
		case d := <-mailbox:
			if d.gen == g {
				b.dispatch(s, d)
			}
		default:
			return
		}
	}
}

func drainDiscard(mailbox chan delivery) {
	for {
		select {
		case <-mailbox:
		default:
			return
		}
	}
}

func (b *Local) dispatch(s *subscription, d delivery) {
	var (
		reply Message
		err   error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("bus: handler panic: %v", r)
			}
		}()
		reply, err = s.handler(d.ctx, d.msg)
	}()

	if d.waiter != nil {
		d.waiter.deliver(d.reqGen, reply, err)
		return
	}
	if err != nil && b.cfg.OnError != nil {
		b.cfg.OnError(d.msg, err)
	}
}

func (b *Local) closeStop(s *subscription) {
	if s.stopClosed.CompareAndSwap(false, true) {
		close(s.stop)
	}
}

// --- pooling -----------------------------------------------------------------

func (b *Local) getSub() *subscription {
	if v := b.subPool.Get(); v != nil {
		return v.(*subscription)
	}
	return &subscription{bus: b, mailbox: make(chan delivery, b.cfg.BufferSize)}
}

func (b *Local) recycleSub(s *subscription) {
	s.handler = nil
	b.subPool.Put(s)
}

func (b *Local) getWaiter() *waiter {
	if v := b.waiterPool.Get(); v != nil {
		w := v.(*waiter)
		select {
		case <-w.ch:
		default:
		}
		g := (w.state.Load() >> 2) + 1
		w.gen = g
		w.state.Store(g<<2 | stArmed)
		return w
	}
	w := &waiter{ch: make(chan reqResult, 1), inbox: b.nextInbox(), gen: 1}
	w.state.Store(1<<2 | stArmed)
	return w
}

func (b *Local) putWaiter(w *waiter) {
	b.waiterPool.Put(w)
}

// deliver hands a reply to the waiter iff it is still armed at reqGen. The CAS
// makes exactly one of {a responder, the requester's abandon} win.
func (w *waiter) deliver(reqGen uint64, msg Message, err error) {
	if w.state.CompareAndSwap(reqGen<<2|stArmed, reqGen<<2|stDone) {
		w.ch <- reqResult{msg: msg, err: err}
	}
}

// finishReq is the requester's exit when no reply arrived on w.ch. It tries to
// claim the waiter; if a responder already claimed it, the result is (or will
// be) on the channel, so read it to keep the pooled channel clean.
func (b *Local) finishReq(w *waiter, g uint64, m Message, e error) (Message, error) {
	if w.state.CompareAndSwap(g<<2|stArmed, g<<2|stDone) {
		b.putWaiter(w)
		return m, e
	}
	res := <-w.ch
	b.putWaiter(w)
	// A reply raced in with the terminal condition. A successful reply wins
	// (we got the answer in time); an errored reply yields to the terminal
	// reason (timeout/cancel/close) the caller actually hit.
	if res.err != nil {
		return m, e
	}
	return res.msg, res.err
}

// --- subject matching --------------------------------------------------------

func isWildcard(pattern string) bool {
	for _, tok := range strings.Split(pattern, ".") {
		if tok == "*" || tok == ">" {
			return true
		}
	}
	return false
}

// matchPattern reports whether a pre-tokenized wildcard pattern matches a
// concrete subject. It walks the subject in place (no allocation): "*" matches
// exactly one token, ">" matches one or more trailing tokens.
func matchPattern(tokens []string, subject string) bool {
	ti := 0
	start := 0
	n := len(subject)
	for i := 0; i <= n; i++ {
		if i < n && subject[i] != '.' {
			continue
		}
		if ti >= len(tokens) {
			return false
		}
		tok := tokens[ti]
		if tok == ">" {
			return true
		}
		if tok != "*" && tok != subject[start:i] {
			return false
		}
		ti++
		start = i + 1
	}
	return ti == len(tokens)
}
