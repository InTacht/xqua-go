package demo

import (
	"context"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport/errors"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/memory"
	itemsvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/item"
	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

type getDemoItemIn struct {
	ID int64 `path:"id" minimum:"1"`
}

type listDemoItemsIn struct {
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" default:"10" minimum:"1" maximum:"50"`
}

type patchDemoItemIn struct {
	ID   int64  `path:"id" minimum:"1"`
	Name string `json:"name" required:"true" example:"gamma"`
}

type deleteDemoItemIn struct {
	ID int64 `path:"id" minimum:"1"`
}

type createDemoItemIn struct {
	Name  string `json:"name" example:"widget"`
	Email string `json:"email" example:"widget@example.com"`
}

type searchIn struct {
	Query string `json:"query" required:"true" example:"alp"`
}

type prefsIn struct {
	Locale string `header:"Accept-Language" default:"en"`
	Theme  string `cookie:"theme" default:"light"`
}

type demoItemOut struct {
	openapi.Response
	Data struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
}

type demoItemsOut struct {
	openapi.Response
	Data struct {
		Items []domain.Item `json:"items"`
	} `json:"data"`
}

type prefsOut struct {
	openapi.Response
	Data struct {
		Locale string `json:"locale"`
		Theme  string `json:"theme"`
	} `json:"data"`
}

type searchOut struct {
	openapi.Response
	Data struct {
		Items []domain.Item `json:"items"`
	} `json:"data"`
}

type uploadIn struct {
	Title string                `form:"title" required:"true"`
	File  *multipart.FileHeader `form:"file" required:"true"`
}

type uploadOut struct {
	openapi.Response
	Data struct {
		Title    string `json:"title"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	} `json:"data"`
}

type ackOut struct {
	openapi.Response
}

type rawPing struct {
	OK bool `json:"ok"`
}

// Register mounts in-memory demo routes under /demo plus imperative escape hatches.
func Register(api *openapi.Generator, svc *itemsvc.Service, t *http.Transport, log runtime.Logger) {
	api.Routes("/demo", func(r *openapi.Router) {
		demo := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(422, errors.ErrValidation),
		})
		demo.Route("/items").Get(openapi.Route{
			Handler:   listDemoItems(svc, log),
			Summary:   "List demo items (cursor pagination)",
			Responses: openapi.Returns().Err(500, errors.ErrDemoFetch),
		})
		demo.Route("/items/:id").Get(openapi.Route{
			Handler:   getDemoItem(svc, log),
			Summary:   "Fetch demo item (catalog mapping)",
			Responses: openapi.Returns().Err(404, errors.ErrDemoNotFound).Err(500, errors.ErrDemoFetch),
		})
		demo.Route("/items/:id").Patch(openapi.Route{
			Handler:   patchDemoItem(svc, log),
			Summary:   "Patch demo item name",
			Responses: openapi.Returns().Err(404, errors.ErrDemoNotFound).Err(500, errors.ErrDemoFetch),
		})
		demo.Route("/items/:id").Delete(openapi.Route{
			Handler:   deleteDemoItem(svc, log),
			Summary:   "Delete demo item",
			Responses: openapi.Returns().Err(404, errors.ErrDemoNotFound).Err(500, errors.ErrDemoFetch),
		})
		demo.Route("/items").Post(openapi.Route{
			Handler: createDemoItem(svc, log),
			Summary: "Create demo item",
			Responses: openapi.Returns().
				Err(422, errors.ErrDemoNameRequired, errors.ErrDemoEmailInvalid).
				Err(500, errors.ErrDemoCreate),
			Requests: []openapi.ContentUnit{{
				Required:    true,
				ContentType: "application/json",
				Structure:   createDemoItemIn{},
				Example: map[string]any{
					"name":  "widget",
					"email": "widget@example.com",
				},
			}},
		})
		demo.Route("/search").Query(openapi.Route{
			Handler:   searchItems(svc, log),
			Summary:   "Search demo items (HTTP QUERY)",
			Responses: openapi.Returns().Err(500, errors.ErrDemoFetch),
			Requests: []openapi.ContentUnit{{
				Required:    true,
				ContentType: "application/json",
				Structure:   searchIn{},
			}},
		})
		demo.Route("/preferences").Get(openapi.Route{
			Handler:   preferences,
			Summary:   "Read demo preferences from header and cookie",
			Responses: openapi.Returns(),
		})
		demo.Route("/upload").Post(openapi.Route{
			Handler:   uploadAsset,
			Summary:   "Multipart upload",
			Responses: openapi.Returns(),
			Requests: []openapi.ContentUnit{{
				Required:    true,
				ContentType: "multipart/form-data",
				Structure:   uploadIn{},
				Encoding: map[string]string{
					"file": "application/pdf, image/png, image/jpeg",
				},
			}},
		})
		r.Describe("/ws").Get(openapi.Route{
			Summary: "WebSocket handshake (documentation only)",
			Extra: []openapi.ResponseDecl{
				{Status: 101, Description: "Switching Protocols"},
			},
		})
		r.Describe("/stream").Post(openapi.Route{
			Summary: "SSE token stream (documentation only)",
			Requests: []openapi.ContentUnit{{
				Required: true,
				Structure: &openapi.Schema{
					Type: "object",
					Properties: map[string]*openapi.Schema{
						"prompt": {Type: "string"},
					},
				},
			}},
			Extra: []openapi.ResponseDecl{{
				Status:      200,
				Description: "Token feed",
				ItemBody:    &openapi.Schema{Ref: "#/components/schemas/TokenChunk"},
			}},
		})
		r.Describe("/export/:id").Get(openapi.Route{
			Summary:     "Binary export (documented; served imperatively)",
			Description: "Live handler is registered on Fiber at GET /demo/export/:id.",
			Extra: []openapi.ResponseDecl{{
				Status:      200,
				Description: "Exported bytes",
				ContentType: "application/octet-stream",
			}},
		})
	})

	t.Fiber().Get("/demo/raw", rawPingHandler)
	t.Fiber().Get("/demo/export/:id", exportItem(svc))
	t.Fiber().Get("/demo/leak", leakInternal)
	t.Fiber().Get("/demo/plain", plainBoom)
}

func listDemoItems(svc *itemsvc.Service, log runtime.Logger) func(context.Context, listDemoItemsIn) (demoItemsOut, error) {
	return func(ctx context.Context, in listDemoItemsIn) (demoItemsOut, error) {
		items, cursor, err := svc.ListCursor(ctx, in.Cursor, in.Limit)
		if err != nil {
			log.ErrorCtx(ctx, err, "list demo items")
			return demoItemsOut{}, mapItemErr(err, errors.ErrDemoFetch)
		}
		var out demoItemsOut
		out.Message = "items listed"
		out.Data.Items = items
		if cursor.Next != "" {
			out.Cursor = &cursor
		}
		return out, nil
	}
}

func getDemoItem(svc *itemsvc.Service, log runtime.Logger) func(context.Context, getDemoItemIn) (demoItemOut, error) {
	return func(ctx context.Context, in getDemoItemIn) (demoItemOut, error) {
		item, err := svc.Get(ctx, in.ID)
		if err != nil {
			log.ErrorCtx(ctx, err, "fetch demo item")
			return demoItemOut{}, mapItemErr(err, errors.ErrDemoFetch)
		}
		var out demoItemOut
		out.Message = "ok"
		out.Data.ID = item.ID
		out.Data.Name = item.Name
		return out, nil
	}
}

func patchDemoItem(svc *itemsvc.Service, log runtime.Logger) func(context.Context, patchDemoItemIn) (demoItemOut, error) {
	return func(ctx context.Context, in patchDemoItemIn) (demoItemOut, error) {
		item, err := svc.Update(ctx, in.ID, strings.TrimSpace(in.Name))
		if err != nil {
			log.ErrorCtx(ctx, err, "patch demo item", strconv.FormatInt(in.ID, 10))
			return demoItemOut{}, mapItemErr(err, errors.ErrDemoFetch)
		}
		var out demoItemOut
		out.Message = "patched"
		out.Data.ID = item.ID
		out.Data.Name = item.Name
		return out, nil
	}
}

func deleteDemoItem(svc *itemsvc.Service, log runtime.Logger) func(context.Context, deleteDemoItemIn) (ackOut, error) {
	return func(ctx context.Context, in deleteDemoItemIn) (ackOut, error) {
		if err := svc.Delete(ctx, in.ID); err != nil {
			log.ErrorCtx(ctx, err, "delete demo item", strconv.FormatInt(in.ID, 10))
			return ackOut{}, mapItemErr(err, errors.ErrDemoFetch)
		}
		return ackOut{Response: openapi.Response{Message: "deleted"}}, nil
	}
}

func createDemoItem(svc *itemsvc.Service, log runtime.Logger) func(context.Context, createDemoItemIn) (demoItemOut, error) {
	return func(ctx context.Context, in createDemoItemIn) (demoItemOut, error) {
		name := strings.TrimSpace(in.Name)
		email := strings.TrimSpace(in.Email)
		var valErrs xerrors.Errors
		if name == "" {
			valErrs = append(valErrs, errors.ErrDemoNameRequired)
		}
		if email == "" || !strings.Contains(email, "@") {
			valErrs = append(valErrs, errors.ErrDemoEmailInvalid)
		}
		if len(valErrs) > 0 {
			return demoItemOut{}, valErrs
		}
		item, err := svc.Create(ctx, name)
		if err != nil {
			log.ErrorCtx(ctx, err, "create demo item")
			return demoItemOut{}, mapItemErr(err, errors.ErrDemoCreate)
		}
		var out demoItemOut
		out.Message = "created"
		out.Data.ID = item.ID
		out.Data.Name = item.Name
		return out, nil
	}
}

func searchItems(svc *itemsvc.Service, log runtime.Logger) func(context.Context, searchIn) (searchOut, error) {
	return func(ctx context.Context, in searchIn) (searchOut, error) {
		items, err := svc.Search(ctx, in.Query)
		if err != nil {
			log.ErrorCtx(ctx, err, "search demo items")
			return searchOut{}, mapItemErr(err, errors.ErrDemoFetch)
		}
		var out searchOut
		out.Message = "search complete"
		out.Data.Items = items
		return out, nil
	}
}

func preferences(_ context.Context, in prefsIn) (prefsOut, error) {
	var out prefsOut
	out.Message = "preferences read"
	out.Data.Locale = in.Locale
	out.Data.Theme = in.Theme
	return out, nil
}

func uploadAsset(_ context.Context, in uploadIn) (uploadOut, error) {
	var out uploadOut
	out.Message = "uploaded"
	out.Data.Title = in.Title
	out.Data.Filename = in.File.Filename
	out.Data.Size = in.File.Size
	return out, nil
}

func rawPingHandler(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send([]byte(`{"ok":true}`))
}

func exportItem(svc *itemsvc.Service) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.ParseInt(c.Params("id"), 10, 64)
		if err != nil || id < 1 {
			return fiber.NewError(fiber.StatusBadRequest, "invalid id")
		}
		item, err := svc.Get(c.Context(), id)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "item not found")
		}
		c.Set(fiber.HeaderContentType, "application/octet-stream")
		c.Set("Content-Disposition", "attachment; filename=item-"+strconv.FormatInt(id, 10)+".txt")
		return c.Send([]byte("item:" + item.Name))
	}
}

func mapItemErr(err error, fallback *xerrors.Error) error {
	return xerrors.MapOr(err, fallback,
		xerrors.Pair(memory.ErrItemNotFound, errors.ErrDemoNotFound),
	)
}

func leakInternal(c fiber.Ctx) error {
	return memory.ErrItemCorrupt
}

func plainBoom(c fiber.Ctx) error {
	return xerrors.NewPlain("driver exploded")
}
