package validate

import (
	"net/http"
	"strings"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func AllowsOperationMethod(version, method string) bool {
	if strings.EqualFold(method, http.MethodConnect) {
		return version == spec.Version320
	}

	return true
}
