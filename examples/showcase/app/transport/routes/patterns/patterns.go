package patterns

import (
	"context"
	"sync"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/gofiber/fiber/v3"
)

// Register mounts canonical openapi patterns under /demo/patterns.
func Register(api *openapi.Generator) {
	api.Routes("/demo/patterns", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().
				Err(401, errors.ErrMissingBearer, errors.ErrTokenExpired, errors.ErrInvalidToken).
				Err(429, errors.ErrRateLimited).
				Err(422, errors.ErrValidation),
			Security: openapi.RequireSecurity("PatternsBearer"),
			AfterAuth: []openapi.Middleware{
				demoRateLimit(),
			},
		})

		v1.Route("/trigger").Post(openapi.Route{
			Handler:   trigger,
			Summary:   "Idempotent trigger (replay returns 200 with replayed flag)",
			Responses: openapi.Returns(),
		})

		v1.Route("/subscribers/:id/data").Put(openapi.Route{
			Handler:   replaceSubscriberData,
			Summary:   "Replace subscriber data (wrapped dynamic map)",
			Responses: openapi.Returns(),
		})
	})
}

type triggerIn struct {
	IdempotencyKey string `header:"Idempotency-Key" required:"true"`
	Payload        string `json:"payload" required:"true"`
}

type triggerOut struct {
	openapi.Response
	Data struct {
		RunID    string `json:"run_id"`
		Replayed bool   `json:"replayed,omitempty"`
	} `json:"data"`
}

type replaceDataIn struct {
	ID       string            `path:"id"`
	Metadata map[string]string `json:"metadata" required:"true"`
}

type replaceDataOut struct {
	openapi.Response
	Data struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
	} `json:"data"`
}

var (
	idempotencyMu sync.Mutex
	idempotency   = map[string]string{}
	rateLimitMu   sync.Mutex
	rateCounts    = map[string]int{}
)

func trigger(_ context.Context, in triggerIn) (triggerOut, error) {
	idempotencyMu.Lock()
	defer idempotencyMu.Unlock()

	if runID, ok := idempotency[in.IdempotencyKey]; ok {
		var out triggerOut
		out.Message = "duplicate idempotency key; returning existing run"
		out.Data.RunID = runID
		out.Data.Replayed = true
		return out, nil
	}

	runID := "run_" + in.IdempotencyKey
	idempotency[in.IdempotencyKey] = runID
	var out triggerOut
	out.Message = "trigger accepted"
	out.Data.RunID = runID
	return out, nil
}

func replaceSubscriberData(_ context.Context, in replaceDataIn) (replaceDataOut, error) {
	var out replaceDataOut
	out.Message = "subscriber data replaced"
	out.Data.ID = in.ID
	out.Data.Metadata = in.Metadata
	return out, nil
}

func demoRateLimit() openapi.Middleware {
	return func(c fiber.Ctx, ctx openapi.RouteContext) error {
		key := c.Get("Authorization")
		if key == "" {
			key = "anonymous"
		}
		rateLimitMu.Lock()
		rateCounts[key]++
		count := rateCounts[key]
		rateLimitMu.Unlock()
		if count > 5 {
			return ctx.WriteError(errors.ErrRateLimited)
		}
		return nil
	}
}

// Schemes returns security schemes for the patterns demo.
func Schemes() map[string]openapi.Scheme {
	return map[string]openapi.Scheme{
		"PatternsBearer": openapi.BearerScheme(openapi.BearerOptions{
			Desc: "Patterns demo bearer with distinct 401 codes",
			Verify: func(_ context.Context, cred openapi.Credential) (openapi.Identity, error) {
				switch cred.Raw {
				case "expired-token":
					return nil, errors.ErrTokenExpired
				case "invalid-token":
					return nil, errors.ErrInvalidToken
				case "good-token":
					return "patterns-user", nil
				default:
					return nil, errors.ErrInvalidToken
				}
			},
		}),
	}
}
