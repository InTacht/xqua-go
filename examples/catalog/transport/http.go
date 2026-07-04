package transport

import (
	"github.com/InTacht/xqua-go/examples/catalog/store"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

// Public catalog: the only errors allowed on the wire.
var catalog = errors.NewCatalog("catalog-demo")

var (
	errIDRequired = catalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10001", Message: "id is required", Source: "params.id",
	})
	errNameRequired = catalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10002", Message: "name is required", Source: "body.name",
	})
	errEmailInvalid = catalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10003", Message: "email is invalid", Source: "body.email",
	})
	errItemNotFound = catalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "10004", Message: "item not found", Source: "params.id",
	})
	errFetchFailed = catalog.Define(errors.Def{
		Kind: errors.KindInternal, Code: "10005", Message: "fetch failed",
	})
)

var items = store.NewItems()

// HTTP demonstrates public catalog discipline and kind→status defaults.
//
//	curl http://127.0.0.1:8080/items/1          # 200
//	curl http://127.0.0.1:8080/items/9          # 404 public not_found
//	curl http://127.0.0.1:8080/items/99         # 500 mapped internal (not store.ErrCorrupt)
//	curl -X POST http://127.0.0.1:8080/items    # 422 validation collection
//	curl http://127.0.0.1:8080/leak             # 500 unhandled (internal catalog blocked)
//	curl http://127.0.0.1:8080/plain            # 500 unhandled (plain error)
func HTTP(log runtime.Logger) runtime.Unit {
	return http.New(http.Config{
		Logger:      log,
		Catalog:     catalog,
		FiberConfig: fiber.Config{ServerHeader: "catalog-demo"},
	}).Routes("/", func(r *http.Router) {
		r.Get("/items/:id", getItem(log))
		r.Post("/items", createItem)
		r.Get("/leak", leakInternal)
		r.Get("/plain", plainBoom)
	})
}

func getItem(log runtime.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := http.ParamInt64(c, "id")
		if err != nil || id <= 0 {
			return errIDRequired
		}
		name, err := items.Get(c.Context(), id)
		if err != nil {
			log.ErrorCtx(c.Context(), err, "fetch item")
			// Map internal store errors → public catalog. Unmapped store
			// errors become errFetchFailed (never leak store.ErrCorrupt codes).
			return errors.MapOr(err, errFetchFailed,
				errors.Pair(store.ErrNotFound, errItemNotFound),
			)
		}
		return http.RES(c).Message("ok").Data("item", map[string]any{
			"id": id, "name": name,
		}).Ok()
	}
}

func createItem(c fiber.Ctx) error {
	// Demo only: always fail with a horizontal validation collection.
	// KindValidation → HTTP 422 via DefaultKindStatuses.
	return errors.Errors{errNameRequired, errEmailInvalid}
}

func leakInternal(c fiber.Ctx) error {
	// Returning an internal-catalog error on purpose. The global handler
	// replaces it with Fallbacks.Unhandled so store codes never appear.
	return store.ErrCorrupt
}

func plainBoom(c fiber.Ctx) error {
	return errors.NewPlain("driver exploded")
}
