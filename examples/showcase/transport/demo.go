package transport

import (
	"context"
	"mime/multipart"

	demostore "github.com/InTacht/xqua-go/examples/showcase/demo/store"
	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

var demoItems = demostore.NewItems()

type getDemoItemIn struct {
	ID int64 `path:"id" minimum:"1"`
}

type createDemoItemIn struct{}

type demoItemOut struct {
	openapi.Response
	Data struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
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

func registerDemoRoutes(api *openapi.Generator, t *http.Transport, log runtime.Logger) {
	api.Routes("/demo", func(r *openapi.Router) {
		demo := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(422, errValidation),
		})
		demo.Route("/items/:id").Get(openapi.Route{
			Handler:   getDemoItem(log),
			Summary:   "Fetch demo item (catalog mapping)",
			Responses: openapi.Returns().Err(404, errDemoNotFound).Err(500, errDemoFetch),
		})
		demo.Route("/items").Post(openapi.Route{
			Handler:   createDemoItem,
			Summary:   "Validation collection demo",
			Responses: openapi.Returns().Err(422, errDemoNameRequired, errDemoEmailInvalid),
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
	})

	// Imperative escape hatches — never appear in OpenAPI.
	t.Fiber().Get("/demo/leak", leakInternal)
	t.Fiber().Get("/demo/plain", plainBoom)
}

func getDemoItem(log runtime.Logger) func(context.Context, getDemoItemIn) (demoItemOut, error) {
	return func(ctx context.Context, in getDemoItemIn) (demoItemOut, error) {
		name, err := demoItems.Get(ctx, in.ID)
		if err != nil {
			log.ErrorCtx(ctx, err, "fetch demo item")
			return demoItemOut{}, errors.MapOr(err, errDemoFetch,
				errors.Pair(demostore.ErrNotFound, errDemoNotFound),
			)
		}
		var out demoItemOut
		out.Message = "ok"
		out.Data.ID = in.ID
		out.Data.Name = name
		return out, nil
	}
}

func createDemoItem(_ context.Context, _ createDemoItemIn) (demoItemOut, error) {
	return demoItemOut{}, errors.Errors{errDemoNameRequired, errDemoEmailInvalid}
}

func uploadAsset(_ context.Context, in uploadIn) (uploadOut, error) {
	var out uploadOut
	out.Message = "uploaded"
	out.Data.Title = in.Title
	out.Data.Filename = in.File.Filename
	out.Data.Size = in.File.Size
	return out, nil
}

func leakInternal(c fiber.Ctx) error {
	return demostore.ErrCorrupt
}

func plainBoom(c fiber.Ctx) error {
	return errors.NewPlain("driver exploded")
}
