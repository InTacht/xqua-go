package parser

import (
	"regexp"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

// ColonParamParser converts framework parameters such as "/users/:id" and
// named catch-all parameters such as "/files/*filepath" to OpenAPI templates.
type ColonParamParser struct {
	colonRe    *regexp.Regexp
	wildcardRe *regexp.Regexp
}

var _ spec.PathParser = &ColonParamParser{}

// NewColonParamParser creates a new ColonParamParser instance.
func NewColonParamParser() *ColonParamParser {
	return &ColonParamParser{
		colonRe:    regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`),
		wildcardRe: regexp.MustCompile(`\*([a-zA-Z_][a-zA-Z0-9_]*)`),
	}
}

// Parse converts supported framework parameters to OpenAPI-style parameters.
func (p *ColonParamParser) Parse(colonParam string) (string, error) {
	parsed := p.colonRe.ReplaceAllString(colonParam, "{$1}")
	parsed = p.wildcardRe.ReplaceAllString(parsed, "{$1}")
	return parsed, nil
}
