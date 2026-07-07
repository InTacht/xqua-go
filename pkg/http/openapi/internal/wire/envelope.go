package wire

import (
	"reflect"

	xhttp "github.com/InTacht/xqua-go/pkg/http"
)

const SuccessStatus = "success"

// Response is the success envelope shell embedded in handler output types.
type Response struct {
	Status          string            `json:"status"`
	Message         string            `json:"message,omitempty"`
	RequestID       string            `json:"request_id,omitempty"`
	ClientRequestID string            `json:"client_request_id,omitempty"`
	Pagination      *xhttp.Pagination `json:"pagination,omitempty"`
	Cursor          *xhttp.Cursor     `json:"cursor,omitempty"`
}

// ErrorEnvelope is the fixed error JSON shape.
type ErrorEnvelope struct {
	Status          string              `json:"status"`
	Message         string              `json:"message,omitempty"`
	RequestID       string              `json:"request_id,omitempty"`
	ClientRequestID string              `json:"client_request_id,omitempty"`
	Errors          []xhttp.ErrorDetail `json:"errors"`
}

var responseShellType = reflect.TypeFor[Response]()

// Enveloped reports whether t anonymously embeds wire.Response.
func Enveloped(t reflect.Type) bool {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return false
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.Anonymous {
			continue
		}
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft == responseShellType {
			return true
		}
		if ft.Kind() == reflect.Struct && Enveloped(ft) {
			return true
		}
	}
	return false
}

// FillSuccessEmbed sets transport-owned fields on an anonymous Response embed.
func FillSuccessEmbed(out any, requestID, clientRequestID string) {
	v := reflect.ValueOf(out)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	fillResponseField(v, requestID, clientRequestID)
}

func fillResponseField(v reflect.Value, requestID, clientRequestID string) {
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		fv := v.Field(i)
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft == responseShellType {
				if fv.Kind() == reflect.Pointer {
					if fv.IsNil() {
						fv.Set(reflect.New(responseShellType))
					}
					fv = fv.Elem()
				}
				if fv.CanSet() {
					if fv.FieldByName("Status").CanSet() {
						fv.FieldByName("Status").SetString(SuccessStatus)
					}
					if requestID != "" && fv.FieldByName("RequestID").CanSet() {
						fv.FieldByName("RequestID").SetString(requestID)
					}
					if clientRequestID != "" && fv.FieldByName("ClientRequestID").CanSet() {
						fv.FieldByName("ClientRequestID").SetString(clientRequestID)
					}
				}
				return
			}
			if ft.Kind() == reflect.Struct {
				nested := fv
				if nested.Kind() == reflect.Pointer {
					if nested.IsNil() {
						continue
					}
					nested = nested.Elem()
				}
				fillResponseField(nested, requestID, clientRequestID)
			}
		}
	}
}
