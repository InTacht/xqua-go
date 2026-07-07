package transport

import (
	"context"
	stderrors "errors"

	"github.com/InTacht/xqua-go/examples/bus/worker"
	"github.com/InTacht/xqua-go/pkg/bus"
	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
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

type Deps struct {
	Name string
	Host string
	Port int
	Bus  bus.Bus
}

type workIn struct {
	Q string `query:"q"`
}

type workOut struct {
	openapi.Response
	Data struct {
		Result string `json:"result"`
	} `json:"data"`
}

func HTTP(d Deps, log runtime.Logger) runtime.Unit {
	t := http.New(http.Config{
		Host:        d.Host,
		Port:        d.Port,
		Logger:      log,
		Catalog:     catalog,
		FiberConfig: fiber.Config{ServerHeader: d.Name},
	})
	openapi.New(t, openapi.Config{
		Specs: []openapi.Spec{{Path: "/openapi.json", Title: d.Name}},
	}).Routes("/", func(r *openapi.Router) {
		r.Route("/work").
			Get(openapi.Route{
				Handler:   doWork(d.Bus, log),
				Summary:   "Run work over the bus (GET)",
				Responses: openapi.Returns().Err(422, errQueryRequired).Err(500, errNoWorkers, errWorkTimeout, errWorkFailed),
			}).
			Post(openapi.Route{
				Handler:   doWorkPost(d.Bus, log),
				Summary:   "Run work over the bus (POST)",
				Responses: openapi.Returns().Err(422, errQueryRequired).Err(500, errNoWorkers, errWorkTimeout, errWorkFailed),
			})
	})
	return t
}

func doWork(b bus.Bus, log runtime.Logger) func(context.Context, workIn) (workOut, error) {
	return func(ctx context.Context, in workIn) (workOut, error) {
		if in.Q == "" {
			return workOut{}, errQueryRequired
		}
		return runWork(ctx, b, log, in.Q)
	}
}

func doWorkPost(b bus.Bus, log runtime.Logger) func(context.Context, workBodyIn) (workOut, error) {
	return func(ctx context.Context, in workBodyIn) (workOut, error) {
		if in.Q == "" {
			return workOut{}, errQueryRequired
		}
		return runWork(ctx, b, log, in.Q)
	}
}

type workBodyIn struct {
	Q string `json:"q"`
}

func runWork(ctx context.Context, b bus.Bus, log runtime.Logger, q string) (workOut, error) {
	reply, err := b.Request(ctx, bus.Message{
		Subject: worker.SubjectWork,
		Data:    []byte(q),
	})
	if err != nil {
		log.ErrorCtx(ctx, err, "work request failed")
		return workOut{}, mapBusErr(err)
	}
	var out workOut
	out.Message = "work complete"
	out.Data.Result = string(reply.Data)
	return out, nil
}

func mapBusErr(err error) error {
	switch {
	case stderrors.Is(err, bus.ErrNoResponders):
		return errNoWorkers
	case stderrors.Is(err, bus.ErrTimeout):
		return errWorkTimeout
	case stderrors.Is(err, bus.ErrClosed):
		return errWorkFailed
	default:
		return errWorkFailed
	}
}
