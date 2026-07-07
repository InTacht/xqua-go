package builder

import (
	"fmt"
	"net/http"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func SetOperation(item *spec.PathItem, method string, op *spec.Operation, version string) error {
	switch method {
	case http.MethodGet:
		if item.Get != nil {
			return validate.Errorf("duplicate GET operation")
		}
		item.Get = op
	case http.MethodPut:
		if item.Put != nil {
			return validate.Errorf("duplicate PUT operation")
		}
		item.Put = op
	case http.MethodPost:
		if item.Post != nil {
			return validate.Errorf("duplicate POST operation")
		}
		item.Post = op
	case http.MethodDelete:
		if item.Delete != nil {
			return validate.Errorf("duplicate DELETE operation")
		}
		item.Delete = op
	case http.MethodOptions:
		if item.Options != nil {
			return validate.Errorf("duplicate OPTIONS operation")
		}
		item.Options = op
	case http.MethodHead:
		if item.Head != nil {
			return validate.Errorf("duplicate HEAD operation")
		}
		item.Head = op
	case http.MethodPatch:
		if item.Patch != nil {
			return validate.Errorf("duplicate PATCH operation")
		}
		item.Patch = op
	case http.MethodTrace:
		if item.Trace != nil {
			return validate.Errorf("duplicate TRACE operation")
		}
		item.Trace = op
	case "QUERY":
		if item.Query != nil {
			return validate.Errorf("duplicate QUERY operation")
		}
		item.Query = op
	default:
		if version != spec.Version320 {
			return validate.Errorf("unsupported HTTP method %q", method)
		}
		if item.AdditionalOperations == nil {
			item.AdditionalOperations = map[string]*spec.Operation{}
		}
		if _, exists := item.AdditionalOperations[method]; exists {
			return validate.Errorf("duplicate %s operation", method)
		}
		item.AdditionalOperations[method] = op
	}
	return nil
}

type ResponseKey struct {
	Status      int
	ContentType string
	IsDefault   bool
}

func MergeResponses(responses []*spec.ContentUnit) []*spec.ContentUnit {
	type group struct {
		key   ResponseKey
		items []*spec.ContentUnit
	}
	groups := map[ResponseKey]*group{}
	var order []ResponseKey
	for _, resp := range responses {
		key := ResponseKey{Status: resp.HTTPStatus, ContentType: ContentType(resp), IsDefault: resp.IsDefault}
		if _, ok := groups[key]; !ok {
			groups[key] = &group{key: key}
			order = append(order, key)
		}
		groups[key].items = append(groups[key].items, resp)
	}

	out := make([]*spec.ContentUnit, 0, len(order))
	for _, key := range order {
		items := groups[key].items
		if len(items) == 1 {
			out = append(out, items[0])
			continue
		}
		merged := *items[0]
		values := make([]any, 0, len(items))
		for _, item := range items {
			values = append(values, item.Structure)
		}
		merged.Structure = OneOf(values...)
		out = append(out, &merged)
	}
	return out
}

func ContentType(cu *spec.ContentUnit) string {
	if cu != nil && cu.ContentType != "" {
		return cu.ContentType
	}
	if cu != nil && cu.Structure != nil {
		if ct := reflect.InferContentType(cu.Structure); ct != "" {
			return ct
		}
	}
	return "application/json"
}

func ResponseDescription(cu *spec.ContentUnit) string {
	if cu.Description != "" {
		return cu.Description
	}
	if cu.IsDefault {
		return "Default response"
	}
	if text := http.StatusText(cu.HTTPStatus); text != "" {
		return text
	}
	return fmt.Sprintf("HTTP %d response", cu.HTTPStatus)
}

type oneOfValue struct {
	values []any
}

func (ov oneOfValue) GetValues() []any {
	return ov.values
}

func OneOf(values ...any) any {
	return oneOfValue{values: values}
}
