package auth

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport/errors"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/memory"
	authsvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/auth"
	xerrors "github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

// Schemes returns the demo OpenAPI security schemes wired to the auth service.
func Schemes(svc *authsvc.Service) map[string]openapi.Scheme {
	verify := func(ctx context.Context, cred openapi.Credential) (openapi.Identity, error) {
		session, ok := svc.Lookup(ctx, cred.Raw)
		if !ok {
			return nil, errors.ErrUnauthorized
		}
		return session, nil
	}
	oauthVerify := func(ctx context.Context, cred openapi.Credential) (openapi.Identity, error) {
		session, ok := svc.Lookup(ctx, cred.Raw)
		if !ok {
			return nil, errors.ErrUnauthorized
		}
		for _, scope := range cred.Scopes {
			if scope == "demo:admin" {
				return nil, errors.ErrForbidden
			}
		}
		return session, nil
	}
	return map[string]openapi.Scheme{
		"DemoBearer": openapi.BearerScheme(openapi.BearerOptions{
			Desc:   "Bearer API key",
			Verify: verify,
		}),
		"DemoApiKey": openapi.APIKeyScheme(openapi.APIKeyOptions{
			Name:   "X-API-Token",
			In:     openapi.InHeader,
			Desc:   "API key header",
			Verify: verify,
		}),
		"DemoCookie": openapi.APIKeyScheme(openapi.APIKeyOptions{
			Name:   "demo_session",
			In:     openapi.InCookie,
			Desc:   "Session cookie API key",
			Verify: verify,
		}),
		"DemoOAuth": openapi.OAuth2Scheme(openapi.OAuth2Options{
			Desc: "Demo OAuth2 bearer token",
			Flows: &openapi.OAuthFlows{
				ClientCredentials: &openapi.OAuthFlow{
					TokenURL: "https://example.com/oauth/token",
					Scopes: map[string]string{
						"demo:read":  "Read demo resources",
						"demo:admin": "Admin-only demo operations",
					},
				},
			},
			Verify: oauthVerify,
		}),
	}
}

type loginIn struct {
	Username string `json:"username" required:"true"`
	Password string `json:"password" required:"true"`
}

type loginOut struct {
	openapi.Response
	Data struct {
		Token string         `json:"token"`
		User  domain.Session `json:"user"`
	} `json:"data"`
}

type meOut struct {
	openapi.Response
	Data struct {
		User domain.Session `json:"user"`
	} `json:"data"`
}

// Register mounts demo auth routes under /demo.
func Register(api *openapi.Generator, svc *authsvc.Service) {
	api.Routes("/demo", func(r *openapi.Router) {
		authGroup := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().
				Err(422, errors.ErrValidation).
				Err(401, errors.ErrUnauthorized).
				Err(403, errors.ErrForbidden),
		})

		authGroup.Route("/auth/login").Post(openapi.Route{
			Handler:   login(svc),
			Summary:   "Login and receive an API key",
			Security:  openapi.PublicSecurity(),
			Responses: openapi.Returns().Err(401, errors.ErrUnauthorized),
		})

		authGroup.Route("/me").Get(openapi.Route{
			Handler:  me,
			Summary:  "Current demo user (Bearer or X-API-Token)",
			Security: openapi.RequireAnySecurity("DemoBearer", "DemoApiKey"),
		})

		authGroup.Route("/session").Get(openapi.Route{
			Handler:  me,
			Summary:  "Current demo user (session cookie)",
			Security: openapi.RequireSecurity("DemoCookie"),
		})

		authGroup.Route("/admin").Get(openapi.Route{
			Handler:  adminOnly,
			Summary:  "Admin-only demo route (API key header)",
			Security: openapi.RequireSecurity("DemoApiKey"),
		})

		authGroup.Route("/scoped-admin").Get(openapi.Route{
			Handler:  me,
			Summary:  "OAuth scope demo (requires demo:admin)",
			Security: openapi.RequireSecurity("DemoOAuth", "demo:admin"),
		})
	})
}

func login(svc *authsvc.Service) func(context.Context, loginIn) (loginOut, error) {
	return func(ctx context.Context, in loginIn) (loginOut, error) {
		token, session, err := svc.Login(ctx, in.Username, in.Password)
		if err != nil {
			if xerrors.Is(err, authsvc.ErrInvalidCredentials) {
				return loginOut{}, errors.ErrUnauthorized
			}
			return loginOut{}, mapAuthErr(err, errors.ErrDemoFetch)
		}
		var out loginOut
		out.Message = "logged in"
		out.Data.Token = token
		out.Data.User = session
		return out, nil
	}
}

func me(ctx context.Context, _ struct{}) (meOut, error) {
	session, ok := openapi.IdentityAs[domain.Session](ctx)
	if !ok {
		return meOut{}, errors.ErrUnauthorized
	}
	var out meOut
	out.Message = "ok"
	out.Data.User = session
	return out, nil
}

func adminOnly(ctx context.Context, _ struct{}) (meOut, error) {
	session, ok := openapi.IdentityAs[domain.Session](ctx)
	if !ok {
		return meOut{}, errors.ErrUnauthorized
	}
	if session.Username != "demo" {
		return meOut{}, errors.ErrForbidden
	}
	var out meOut
	out.Message = "admin ok"
	out.Data.User = session
	return out, nil
}

func mapAuthErr(err error, fallback *xerrors.Error) error {
	return xerrors.MapOr(err, fallback,
		xerrors.Pair(memory.ErrIssueToken, errors.ErrDemoFetch),
	)
}
