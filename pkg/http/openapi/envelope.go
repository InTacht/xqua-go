package openapi

import (
	"reflect"

	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/wire"
)

// Response is the success envelope shell. Handlers anonymous-embed this struct.
type Response = wire.Response

// Pagination describes page-based list metadata.
type Pagination = http.Pagination

// Cursor describes cursor-based list metadata.
type Cursor = http.Cursor

// ErrorDetail is the JSON wire shape of a single API error entry.
type ErrorDetail = http.ErrorDetail

// ErrorEnvelope is the fixed error response shape.
type ErrorEnvelope = wire.ErrorEnvelope

// Enveloped reports whether t anonymously embeds openapi.Response.
func Enveloped(t reflect.Type) bool {
	return wire.Enveloped(t)
}
