package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func TestBuilder_AddOperation(t *testing.T) {
	cfg := &spec.Config{OpenAPIVersion: spec.Version304}
	doc := &spec.Document{Paths: map[string]*spec.PathItem{}}
	b := NewBuilder(cfg, doc)

	err := b.AddOperation("GET", "/test", OperationConfig{Summary: "Test Summary"})
	require.NoError(t, err)

	assert.NotNil(t, doc.Paths["/test"])
	assert.NotNil(t, doc.Paths["/test"].Get)
	assert.Equal(t, "Test Summary", doc.Paths["/test"].Get.Summary)
}

func TestBuilder_AddWebhookOperation(t *testing.T) {
	t.Run("OpenAPI 3.0.4", func(t *testing.T) {
		cfg := &spec.Config{OpenAPIVersion: spec.Version304}
		doc := &spec.Document{}
		b := NewBuilder(cfg, doc)

		err := b.AddWebhookOperation("POST", "webhook", OperationConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhooks require OpenAPI 3.1.x or 3.2.0")
	})

	t.Run("OpenAPI 3.1.2", func(t *testing.T) {
		cfg := &spec.Config{OpenAPIVersion: spec.Version312}
		doc := &spec.Document{}
		b := NewBuilder(cfg, doc)

		err := b.AddWebhookOperation("POST", "webhook", OperationConfig{})
		require.NoError(t, err)
		assert.NotNil(t, doc.Webhooks["webhook"])
	})
}

func TestBuilder_Finish(t *testing.T) {
	cfg := &spec.Config{OpenAPIVersion: spec.Version304}
	doc := &spec.Document{}
	b := NewBuilder(cfg, doc)

	// Simulate reflector having components
	b.Reflector.Components["User"] = &spec.Schema{Type: "object"}

	b.Finish()

	assert.NotNil(t, doc.Components)
	assert.NotNil(t, doc.Components.Schemas["User"])
}

func TestBuilder_ComponentsEmpty(t *testing.T) {
	assert.True(t, ComponentsEmpty(nil))
	assert.True(t, ComponentsEmpty(&spec.Components{}))
	assert.False(t, ComponentsEmpty(&spec.Components{Schemas: map[string]*spec.Schema{"S": {}}}))
}

func TestBuilder_SecurityRequirement(t *testing.T) {
	sr := SecurityRequirement("auth", []string{"read"})
	assert.Equal(t, []string{"read"}, sr["auth"])

	sr = SecurityRequirement("auth", nil)
	assert.Equal(t, []string{}, sr["auth"])
}

func TestBuilder_AddOperationTo_QueryVersionGuard(t *testing.T) {
	cfg := &spec.Config{OpenAPIVersion: spec.Version312}
	doc := &spec.Document{Paths: map[string]*spec.PathItem{}}
	b := NewBuilder(cfg, doc)

	err := b.AddOperationTo("QUERY", "/search", OperationConfig{}, doc.Paths)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires OpenAPI 3.2.0")
}

func TestBuilder_AddOperationTo_HideOptionSkipsOperation(t *testing.T) {
	cfg := &spec.Config{OpenAPIVersion: spec.Version320}
	doc := &spec.Document{Paths: map[string]*spec.PathItem{}}
	b := NewBuilder(cfg, doc)

	err := b.AddOperation("GET", "/hidden", OperationConfig{Hide: true})
	require.NoError(t, err)
	assert.NotContains(t, doc.Paths, "/hidden")
}

func TestBuilder_EnsurePathParametersBehavior(t *testing.T) {
	cfg := &spec.Config{OpenAPIVersion: spec.Version304}
	doc := &spec.Document{Paths: map[string]*spec.PathItem{}}
	b := NewBuilder(cfg, doc)

	t.Run("auto-add missing path params and deduplicate", func(t *testing.T) {
		op := &spec.Operation{
			Parameters: []*spec.Parameter{
				nil,
				{
					Name:     "id",
					In:       string(spec.ParameterInPath),
					Required: true,
					Schema:   &spec.Schema{Type: "string"},
				},
			},
		}
		b.ensurePathParameters("/users/{id}/orders/{orderID}", op)
		require.Len(t, op.Parameters, 3)
		assert.Equal(t, "orderID", op.Parameters[2].Name)
		assert.Equal(t, string(spec.ParameterInPath), op.Parameters[2].In)
		assert.True(t, op.Parameters[2].Required)
	})

	t.Run("skip when path parameter component ref exists", func(t *testing.T) {
		op := &spec.Operation{
			Parameters: []*spec.Parameter{{Ref: "#/components/parameters/UserID"}},
		}
		b.ensurePathParameters("/users/{id}", op)
		require.Len(t, op.Parameters, 1)
		assert.Equal(t, "#/components/parameters/UserID", op.Parameters[0].Ref)
	})

	t.Run("skip for non-http-style target", func(t *testing.T) {
		op := &spec.Operation{}
		b.ensurePathParameters("user.created", op)
		assert.Empty(t, op.Parameters)
	})
}
