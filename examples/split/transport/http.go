package transport

import (
	"context"
	"encoding/json"
	stderrors "errors"

	"github.com/InTacht/xqua-go/examples/split/storage"
	"github.com/InTacht/xqua-go/pkg/bus"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var catalog = errors.NewCatalog("split")

var (
	errKeyRequired = catalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10001", Message: "key is required", Source: "params.key",
	})
	errValueRequired = catalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10002", Message: "value is required", Source: "body",
	})
	errNotFound = catalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "10003", Message: "key not found", Source: "params.key",
	})
	errStorage = catalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "10004", Message: "storage unavailable",
	})
)

type Deps struct {
	Name string
	Host string
	Port int
	Bus  bus.Bus
}

type kvKeyIn struct {
	Key string `path:"key"`
}

type kvPutIn struct {
	Key   string `path:"key"`
	Value string `json:"value"`
}

type kvEntryOut struct {
	openapi.Response
	Data struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"data"`
}

type kvStoredOut struct {
	openapi.Response
	Data struct {
		Key string `json:"key"`
		OK  bool   `json:"ok"`
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
		r.Route("/kv/:key").
			Get(openapi.Route{
				Handler:   getKey(d.Bus, log),
				Summary:   "Fetch a key",
				Responses: openapi.Returns().Err(422, errKeyRequired).Err(404, errNotFound).Err(500, errStorage),
			}).
			Put(openapi.Route{
				Handler:   putKey(d.Bus, log),
				Summary:   "Store a key",
				Responses: openapi.Returns().Err(422, errKeyRequired, errValueRequired).Err(500, errStorage),
			})
	})
	return t
}

func getKey(b bus.Bus, log runtime.Logger) func(context.Context, kvKeyIn) (kvEntryOut, error) {
	return func(ctx context.Context, in kvKeyIn) (kvEntryOut, error) {
		if in.Key == "" {
			return kvEntryOut{}, errKeyRequired
		}
		payload, err := json.Marshal(map[string]string{"key": in.Key})
		if err != nil {
			return kvEntryOut{}, errStorage
		}
		reply, err := b.Request(ctx, bus.Message{Subject: storage.SubjectGet, Data: payload})
		if err != nil {
			log.ErrorCtx(ctx, err, "storage get failed")
			return kvEntryOut{}, mapBusErr(err)
		}
		var res struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			Found bool   `json:"found"`
		}
		if err := json.Unmarshal(reply.Data, &res); err != nil {
			return kvEntryOut{}, errStorage
		}
		if !res.Found {
			return kvEntryOut{}, errNotFound
		}
		var out kvEntryOut
		out.Message = "ok"
		out.Data.Key = res.Key
		out.Data.Value = res.Value
		return out, nil
	}
}

func putKey(b bus.Bus, log runtime.Logger) func(context.Context, kvPutIn) (kvStoredOut, error) {
	return func(ctx context.Context, in kvPutIn) (kvStoredOut, error) {
		if in.Key == "" {
			return kvStoredOut{}, errKeyRequired
		}
		if in.Value == "" {
			return kvStoredOut{}, errValueRequired
		}
		payload, err := json.Marshal(map[string]string{"key": in.Key, "value": in.Value})
		if err != nil {
			return kvStoredOut{}, errStorage
		}
		reply, err := b.Request(ctx, bus.Message{Subject: storage.SubjectPut, Data: payload})
		if err != nil {
			log.ErrorCtx(ctx, err, "storage put failed")
			return kvStoredOut{}, mapBusErr(err)
		}
		var res struct {
			Key string `json:"key"`
			OK  bool   `json:"ok"`
		}
		if err := json.Unmarshal(reply.Data, &res); err != nil {
			return kvStoredOut{}, errStorage
		}
		var out kvStoredOut
		out.Message = "stored"
		out.Data.Key = res.Key
		out.Data.OK = res.OK
		return out, nil
	}
}

func mapBusErr(err error) error {
	switch {
	case stderrors.Is(err, bus.ErrNoResponders),
		stderrors.Is(err, bus.ErrTimeout),
		stderrors.Is(err, bus.ErrClosed):
		return errStorage
	default:
		return errStorage
	}
}
