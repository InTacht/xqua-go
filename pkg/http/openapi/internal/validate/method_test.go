package validate_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func TestAllowsOperationMethod(t *testing.T) {
	tests := []struct {
		version string
		method  string
		want    bool
	}{
		{spec.Version304, http.MethodGet, true},
		{spec.Version304, http.MethodPost, true},
		{spec.Version312, http.MethodGet, true},
		{spec.Version320, http.MethodGet, true},
		{spec.Version304, http.MethodConnect, false},
		{spec.Version312, http.MethodConnect, false},
		{spec.Version320, http.MethodConnect, true},
		{spec.Version304, "PURGE", true},
		{spec.Version320, "PURGE", true},
	}

	for _, tt := range tests {
		t.Run(tt.version+"_"+tt.method, func(t *testing.T) {
			assert.Equal(t, tt.want, validate.AllowsOperationMethod(tt.version, tt.method))
		})
	}
}
