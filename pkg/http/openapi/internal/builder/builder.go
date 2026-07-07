package builder

import (
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

type Builder struct {
	Config    *spec.Config
	Doc       *spec.Document
	Reflector *reflect.Reflector
}

var pathParamTemplateRe = regexp.MustCompile(`\{([^{}]+)\}`)

func NewBuilder(cfg *spec.Config, doc *spec.Document) *Builder {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Builder{
		Config:    cfg,
		Doc:       doc,
		Reflector: reflect.NewReflector(cfg),
	}
}

func (b *Builder) AddOperation(method, path string, cfg OperationConfig) error {
	return b.AddOperationTo(method, path, cfg, b.Doc.Paths)
}

func (b *Builder) AddWebhookOperation(method, name string, cfg OperationConfig) error {
	b.Config.Logger.Debug("adding webhook operation", "method", method, "name", name)
	if reflect.IsOpenAPI30(b.Config.OpenAPIVersion) {
		return validate.Errorf("webhooks require OpenAPI 3.1.x or 3.2.0")
	}
	if b.Doc.Webhooks == nil {
		b.Doc.Webhooks = map[string]*spec.PathItem{}
	}
	return b.AddOperationTo(method, name, cfg, b.Doc.Webhooks)
}

func (b *Builder) AddOperationTo(
	method, target string,
	cfg OperationConfig,
	items map[string]*spec.PathItem,
) error {
	b.Config.Logger.Debug("adding operation", "method", method, "target", target)
	if cfg.Hide {
		return nil
	}

	method = strings.ToUpper(method)
	if method == "QUERY" && b.Config.OpenAPIVersion != spec.Version320 {
		return validate.Errorf("method QUERY requires OpenAPI 3.2.0")
	}

	op := &spec.Operation{Responses: map[string]*spec.Response{}}
	op.OperationID = cfg.OperationID
	op.Summary = cfg.Summary
	op.Description = cfg.Description
	op.ExternalDocs = cfg.ExternalDocs
	op.Deprecated = cfg.Deprecated
	op.Tags = append(op.Tags, cfg.Tags...)
	if cfg.SecurityPublic {
		op.Security = []spec.SecurityRequirement{}
	} else {
		for _, sec := range cfg.Security {
			op.Security = append(op.Security, SecurityRequirement(sec.Name, sec.Scopes))
		}
	}

	for _, req := range cfg.Requests {
		if err := b.AddRequest(op, req); err != nil {
			return validate.Errorf("%s %s request: %w", method, target, err)
		}
	}
	if len(cfg.Responses) == 0 {
		op.Responses["default"] = &spec.Response{Description: "Default response"}
	}
	for _, resp := range MergeResponses(cfg.Responses) {
		if err := b.AddResponse(op, resp); err != nil {
			return validate.Errorf("%s %s response: %w", method, target, err)
		}
	}
	for _, customize := range cfg.Customizers {
		customize(op)
	}
	b.ensurePathParameters(target, op)

	item := items[target]
	if item == nil {
		item = &spec.PathItem{}
		items[target] = item
	}
	return SetOperation(item, method, op, b.Config.OpenAPIVersion)
}

func (b *Builder) Finish() {
	if b.Doc.Components == nil {
		b.Doc.Components = &spec.Components{}
	}
	if len(b.Reflector.Components) > 0 {
		if b.Doc.Components.Schemas == nil {
			b.Doc.Components.Schemas = map[string]*spec.Schema{}
		}
		for name, schema := range b.Reflector.Components {
			b.Doc.Components.Schemas[name] = schema
		}
	}
	if ComponentsEmpty(b.Doc.Components) {
		b.Doc.Components = nil
	}
}

func ComponentsEmpty(components *spec.Components) bool {
	if components == nil {
		return true
	}
	return len(components.Schemas) == 0 &&
		len(components.SecuritySchemes) == 0 &&
		len(components.Responses) == 0 &&
		len(components.Parameters) == 0 &&
		len(components.Examples) == 0 &&
		len(components.RequestBodies) == 0 &&
		len(components.Headers) == 0 &&
		len(components.Links) == 0 &&
		len(components.Callbacks) == 0 &&
		len(components.PathItems) == 0 &&
		len(components.MediaTypes) == 0
}

func SecurityRequirement(name string, scopes []string) spec.SecurityRequirement {
	if scopes == nil {
		scopes = []string{}
	}
	return spec.SecurityRequirement{name: scopes}
}

func (b *Builder) ensurePathParameters(target string, op *spec.Operation) {
	if !strings.HasPrefix(target, "/") {
		return
	}
	matches := pathParamTemplateRe.FindAllStringSubmatch(target, -1)
	if len(matches) == 0 {
		return
	}
	existing := map[string]struct{}{}
	hasComponentParamRef := false
	for _, p := range op.Parameters {
		if p == nil {
			continue
		}
		if p.Ref != "" {
			if strings.HasPrefix(p.Ref, "#/components/parameters/") {
				hasComponentParamRef = true
			}
			continue
		}
		if p.In == string(spec.ParameterInPath) && p.Name != "" {
			existing[p.Name] = struct{}{}
		}
	}
	if hasComponentParamRef {
		return
	}
	for _, m := range matches {
		name := m[1]
		if _, ok := existing[name]; ok {
			continue
		}
		b.Config.Logger.Debug("auto-injecting path parameter", "target", target, "param", name)
		op.Parameters = append(op.Parameters, &spec.Parameter{
			Name:     name,
			In:       string(spec.ParameterInPath),
			Required: true,
			Schema:   &spec.Schema{Type: "string"},
		})
		existing[name] = struct{}{}
	}
}
