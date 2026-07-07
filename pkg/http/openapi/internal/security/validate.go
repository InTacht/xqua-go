package security

import (
	"fmt"
	"net/http"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/compile"
)

// ValidateRoute panics when a live route's security error contract is invalid.
func ValidateRoute(prefix string, requirements []Requirement, errCases []compile.ErrCase, describe bool) {
	if describe || len(requirements) == 0 {
		return
	}
	for i, req := range requirements {
		if len(req.Names) != 1 {
			panic(fmt.Sprintf("%s: security requirement[%d] must name exactly one scheme (got %d)", prefix, i, len(req.Names)))
		}
	}
	if !hasStatus(errCases, http.StatusUnauthorized) {
		panic(fmt.Sprintf("%s: secured route requires Responses.Err(401, ...)", prefix))
	}
	if requiresScopes(requirements) && !hasStatus(errCases, http.StatusForbidden) {
		panic(fmt.Sprintf("%s: secured route with scopes requires Responses.Err(403, ...)", prefix))
	}
}

func hasStatus(cases []compile.ErrCase, status int) bool {
	for _, c := range cases {
		if c.Status == status && len(c.Errors) > 0 {
			for _, e := range c.Errors {
				if e != nil {
					return true
				}
			}
		}
	}
	return false
}

func requiresScopes(requirements []Requirement) bool {
	for _, req := range requirements {
		if len(req.Scopes) > 0 {
			return true
		}
	}
	return false
}

// ResolveAuthStatus maps a verify error to a declared HTTP status when possible.
func ResolveAuthStatus(err error, errCases []compile.ErrCase) (int, bool) {
	entries := errors.AsErrors(err)
	if len(entries) == 0 {
		return 0, false
	}
	best := 0
	for _, e := range entries {
		st, ok := declaredStatus(e, errCases)
		if !ok {
			return 0, false
		}
		if st > best {
			best = st
		}
	}
	return best, true
}

func declaredStatus(e *errors.Error, errCases []compile.ErrCase) (int, bool) {
	for _, c := range errCases {
		for _, tmpl := range c.Errors {
			if errors.Is(e, tmpl) {
				return c.Status, true
			}
		}
	}
	return 0, false
}
