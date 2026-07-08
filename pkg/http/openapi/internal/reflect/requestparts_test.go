package reflect_test

import (
	"testing"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pathAndBodyIn struct {
	ID       string            `path:"id"`
	Metadata map[string]string `json:"metadata" required:"true"`
}

func TestReflector_RequestParts_PathAndJSONBody(t *testing.T) {
	cfg := &spec.Config{OpenAPIVersion: spec.Version320}
	r := reflect.NewReflector(cfg)

	params, body, err := r.RequestParts(&pathAndBodyIn{}, "application/json")
	require.NoError(t, err)
	require.Len(t, params, 1)
	require.NotNil(t, body, "expected JSON body schema alongside path param")
	assert.NotNil(t, body.Properties["metadata"])
}
