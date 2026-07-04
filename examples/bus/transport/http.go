package transport

import (
	"errors"

	"github.com/InTacht/xqua-go/examples/bus/worker"
	"github.com/InTacht/xqua-go/pkg/bus"
	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var catalog = xerrors.NewCatalog("bus-demo")

var (
	errQueryRequired = catalog.Define(xerrors.Def{
		Kind: xerrors.KindValidation, Code: "10001", Message: "q is required", Source: "query.q",
	})
	errNoWorkers = catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10002", Message: "no workers available",
	})
	errWorkTimeout = catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10003", Message: "work timed out",
	})
	errWorkFailed = catalog.Define(xerrors.Def{
		Kind: xerrors.KindInternal, Code: "10004", Message: "work failed",
	})
)

// Deps is the subset of process state the HTTP unit needs. main narrows the
// app context at registration; this package never imports it.
type Deps struct {
	Name string
	Host string
	Port int
	Bus  bus.Bus
}

// HTTP builds the HTTP unit. POST /work and GET /work?q= request work over the bus.
func HTTP(d Deps, log runtime.Logger) runtime.Unit {
	return http.New(http.Config{
		Host:        d.Host,
		Port:        d.Port,
		Logger:      log,
		Catalog:     catalog,
		FiberConfig: fiber.Config{ServerHeader: d.Name},
	}).Routes("/", func(r *http.Router) {
		r.Get("/work", doWork(d.Bus, log))
		r.Post("/work", doWork(d.Bus, log))
	})
}

func doWork(b bus.Bus, log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		q := c.Query("q")
		if q == "" {
			q = string(c.Body())
		}
		if q == "" {
			return errQueryRequired
		}

		reply, err := b.Request(c.Context(), bus.Message{
			Subject: worker.SubjectWork,
			Data:    []byte(q),
		})
		if err != nil {
			log.ErrorCtx(c.Context(), err, "work request failed")
			return mapBusErr(err)
		}

		return http.RES(c).
			Message("work complete").
			Data("result", string(reply.Data)).
			Ok()
	}
}

func mapBusErr(err error) error {
	switch {
	case errors.Is(err, bus.ErrNoResponders):
		return errNoWorkers
	case errors.Is(err, bus.ErrTimeout):
		return errWorkTimeout
	case errors.Is(err, bus.ErrClosed):
		return errWorkFailed
	default:
		return errWorkFailed
	}
}
