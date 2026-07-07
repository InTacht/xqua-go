package reflect_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type User struct {
	ID   string `json:"id" required:"true"`
	Name string `json:"name"`
}

type ReflectionVersionPayload struct {
	Name  string `json:"name" const:"fixed" examples:"fixed,other" contentEncoding:"base64" contentMediaType:"text/plain" nullable:"true" exclusiveMaximum:"true"`
	Owner *User  `json:"owner"`
}

type ReflectionVersionPayload312 struct {
	Name  string `json:"name" const:"fixed" examples:"fixed,other" contentEncoding:"base64" contentMediaType:"text/plain" nullable:"true"`
	Score int    `json:"score" exclusiveMinimum:"0"`
}

func generatedComponentProperty(t *testing.T, raw []byte, componentName, propertyName string) map[string]any {
	t.Helper()
	var doc map[string]any
	err := json.Unmarshal(raw, &doc)
	require.NoError(t, err)

	components, ok := doc["components"].(map[string]any)
	require.True(t, ok, "missing components")

	schemas, ok := components["schemas"].(map[string]any)
	require.True(t, ok, "missing components.schemas")

	component, ok := schemas[componentName].(map[string]any)
	require.True(t, ok, "missing component %s", componentName)

	properties, ok := component["properties"].(map[string]any)
	require.True(t, ok, "missing component properties for %s", componentName)

	property, ok := properties[propertyName].(map[string]any)
	require.True(t, ok, "missing component property %s.%s", componentName, propertyName)

	return property
}
