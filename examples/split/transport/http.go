package transport

import (
	"encoding/json"
	stderrors "errors"

	"github.com/InTacht/xqua-go/examples/split/storage"
	"github.com/InTacht/xqua-go/pkg/bus"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
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

// Deps is the subset of process state the HTTP unit needs. main narrows the
// app context at registration; this package never imports it.
type Deps struct {
	Name string
	Host string
	Port int
	Bus  bus.Bus
}

// HTTP is the compute edge: it never holds storage state, only the bus.
//
//	curl http://127.0.0.1:8080/kv/greeting
//	curl -X PUT -d 'world' http://127.0.0.1:8080/kv/greeting
func HTTP(d Deps, log runtime.Logger) runtime.Unit {
	return http.New(http.Config{
		Host:        d.Host,
		Port:        d.Port,
		Logger:      log,
		Catalog:     catalog,
		FiberConfig: fiber.Config{ServerHeader: d.Name},
	}).Routes("/", func(r *http.Router) {
		r.Get("/kv/:key", getKey(d.Bus, log))
		r.Put("/kv/:key", putKey(d.Bus, log))
	})
}

func getKey(b bus.Bus, log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := c.Params("key")
		if key == "" {
			return errKeyRequired
		}
		payload, err := json.Marshal(map[string]string{"key": key})
		if err != nil {
			return errStorage
		}
		reply, err := b.Request(c.Context(), bus.Message{Subject: storage.SubjectGet, Data: payload})
		if err != nil {
			log.ErrorCtx(c.Context(), err, "storage get failed")
			return mapBusErr(err)
		}
		var res struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			Found bool   `json:"found"`
		}
		if err := json.Unmarshal(reply.Data, &res); err != nil {
			return errStorage
		}
		if !res.Found {
			return errNotFound
		}
		return http.RES(c).Message("ok").Data("entry", res).Ok()
	}
}

func putKey(b bus.Bus, log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := c.Params("key")
		if key == "" {
			return errKeyRequired
		}
		value := string(c.Body())
		if value == "" {
			return errValueRequired
		}
		payload, err := json.Marshal(map[string]string{"key": key, "value": value})
		if err != nil {
			return errStorage
		}
		reply, err := b.Request(c.Context(), bus.Message{Subject: storage.SubjectPut, Data: payload})
		if err != nil {
			log.ErrorCtx(c.Context(), err, "storage put failed")
			return mapBusErr(err)
		}
		var res struct {
			Key string `json:"key"`
			OK  bool   `json:"ok"`
		}
		if err := json.Unmarshal(reply.Data, &res); err != nil {
			return errStorage
		}
		return http.RES(c).Message("stored").Data("entry", res).Ok()
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
