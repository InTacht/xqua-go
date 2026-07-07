package reflect_test

import (
	std_reflect "reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

type SchemaExposerType struct{}

func (SchemaExposerType) OpenAPISchema(version string) *spec.Schema {
	return &spec.Schema{Type: "integer", Description: "Exposed"}
}

type staticExposerType struct{}

func (staticExposerType) OpenAPISchema() *spec.Schema {
	return &spec.Schema{Type: "integer", Description: "Static Exposed"}
}

func TestReflector_ExposerBranches(t *testing.T) {
	r := reflect.NewReflector(&spec.Config{OpenAPIVersion: spec.Version312})

	assert.Nil(t, r.SchemaFromValueExposer(nil))
	require.NotNil(t, r.SchemaFromValueExposer(SchemaExposerType{}))
	assert.Equal(t, "Exposed", r.SchemaFromValueExposer(SchemaExposerType{}).Description)
	require.NotNil(t, r.SchemaFromValueExposer(staticExposerType{}))
	assert.Equal(t, "Static Exposed", r.SchemaFromValueExposer(staticExposerType{}).Description)

	assert.Nil(t, r.SchemaFromTypeExposer(nil))
	assert.Nil(t, r.SchemaFromTypeExposer(std_reflect.TypeFor[any]()))
	require.NotNil(t, r.SchemaFromTypeExposer(std_reflect.TypeFor[SchemaExposerType]()))
	assert.Equal(t, "Exposed", r.SchemaFromTypeExposer(std_reflect.TypeFor[SchemaExposerType]()).Description)
}
