// Package openapi is the declarative REST engine for the pkg/http transport. It
// is the gold-standard way to build HTTP APIs on xqua-go: routes are declared as
// plain structs, registered through path-first typed handlers, and simultaneously
// described as one or more OpenAPI 3.2 documents served from the same
// declarations.
//
// Import path: github.com/InTacht/xqua-go/pkg/http/openapi
//
// # Attaching the engine
//
// New(t, cfg) reads the transport's Fiber app, public catalog, logger, and
// default version/status, and auto-registers the configured document endpoints:
//
//	t := http.New(http.Config{Logger: log, Catalog: catalog, Version: "1.2.3"})
//	api := openapi.New(t, openapi.Config{
//	    Specs: []openapi.Spec{{Path: "/openapi.json", Title: "Users API"}},
//	})
//	api.Routes("/api/v1", func(r *openapi.Router) {
//	    users := r.Group(openapi.GroupConfig{
//	        Prefix:    "/users",
//	        Responses: openapi.Returns().Err(422, errValidation),
//	    })
//	    users.Route("/:id").Get(openapi.Route{
//	        Handler:   getUser,
//	        Summary:   "Fetch one user",
//	        Responses: openapi.Returns().Err(404, errNotFound),
//	    })
//	})
//
// Config.Specs is tri-state: nil serves a single document at /openapi.json; an
// empty (non-nil) slice serves none (drive generation via Generator.Document);
// a non-empty slice serves exactly those documents. Each Spec filters the engine
// manifest by Path prefix (segment-aware) and membership tags (Route.Specs).
//
// # Typed handlers
//
// OpenAPI routes use a single handler shape:
//
//	func(ctx context.Context, in In) (Out, error)
//
// The engine binds In from path/query/header/cookie/json tags, calls the handler,
// and writes success or ErrorEnvelope responses. Bind failures return clones of
// catalog entries declared under Responses.Err(422, ...) (merged from group +
// route), matched by error Source when declared. Routes with bindable input must
// declare Err(422, ...) on the route or inherit it from a group. Handlers that
// anonymous-embed openapi.Response receive transport-owned envelope fields on
// success; other return types are marshaled as raw JSON. Success is always HTTP 200.
//
// Imperative Fiber handlers (health checks, streaming, demos) register on
// Router.Fiber() or Transport.Fiber() and are never included in OpenAPI output.
//
// # Response contract
//
// Responses declares public error cases only; success is inferred from the
// handler return type (or from Returns(T{}) on Describe routes):
//
//	Responses: openapi.Returns().Err(404, errNotFound).Err(500, errInternal)
//
// GroupConfig.Responses contributes error cases to child routes. Route
// Responses.Err cases override nothing — they merge with the group.
//
// # Route declarations
//
// Register with path-first builders only:
//
//	r.Route("/users/:id").Get(openapi.Route{...}).Put(openapi.Route{...})
//	r.Describe("/upload").Post(openapi.Route{...}) // docs only, Handler must be nil
//
// A route is Documented — eligible for OpenAPI output — when any of Summary,
// Description, OperationID, Request, Requests, Responses with explicit success
// or Err cases, or Extra is set. Bare Handler + Returns() alone does not
// document a route.
//
// Request bodies and responses are described by reflection (Go struct tags) or
// by explicit *openapi.Schema values. Multipart uploads bind at runtime when
// input fields use form tags with *multipart.FileHeader, []*multipart.FileHeader,
// or multipart.File. OpenAPI 3.2 extras include itemSchema streaming responses
// (ResponseDecl.ItemBody), the QUERY method, multipart form-data, and full
// security scheme metadata (apiKey, http, oauth2, openIdConnect, mutualTLS).
//
// # Security
//
// Register schemes once on Config.Schemes. Each Scheme carries OpenAPI metadata
// plus a Verify hook that returns any identity value (stored on context.Context):
//
//	api := openapi.New(t, openapi.Config{
//	    Schemes: map[string]openapi.Scheme{
//	        "BearerAuth": openapi.BearerScheme(openapi.BearerOptions{
//	            Verify: func(ctx context.Context, cred openapi.Credential) (openapi.Identity, error) {
//	                user, err := auth.VerifyToken(cred.Raw)
//	                return user, err
//	            },
//	        }),
//	        "ApiKey": openapi.APIKeyScheme(openapi.APIKeyOptions{
//	            Name: "X-API-Token", In: openapi.InHeader, Verify: auth.VerifyAPIKey,
//	        }),
//	    },
//	    Security: openapi.RequireSecurity("BearerAuth"), // engine default
//	})
//
// Declare requirements per group or route with SecuritySpec helpers:
//
//	Security: openapi.PublicSecurity()                              // explicitly open
//	Security: openapi.RequireSecurity("BearerAuth")                  // one scheme
//	Security: openapi.RequireAnySecurity("BearerAuth", "ApiKey")     // OpenAPI OR
//	Security: openapi.InheritSecurity()                             // zero value; inherit
//
// Secured live routes must declare Responses.Err(401, ...) and Err(403, ...) when
// scopes are required. The guard runs before binding; handlers read identity with
// openapi.IdentityFrom(ctx) or openapi.IdentityAs[T](ctx).
//
// # Documentation-only operations and webhooks
//
// Describe(path).Method(route) records an operation into the manifest without
// registering anything on Fiber. Describe routes require Returns(T{}) or Extra.
// Spec.Webhooks declares OpenAPI webhook operations (outbound calls).
//
// # Generation
//
// Generator.Document(spec) renders an OpenAPI 3.2 Document from the current
// manifest — pure, for CLI/tests — and is the same code path the served
// endpoints use. Catalog errors are modeled under x-xqua-errors; envelope
// component schemas are included automatically. MarshalJSON and MarshalYAML
// serialize a document deterministically.
package openapi
