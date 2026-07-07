package openapi

import (
	"sync"

	xhttp "github.com/InTacht/xqua-go/pkg/http"
)

// handler serves one OpenAPI document as JSON. The document is generated on the
// first request — after all routes are registered — and cached for the process
// lifetime. install attaches it directly on Fiber, so spec endpoints are never
// documented and never appear in any generated document.
func (g *Generator) handler(s Spec) xhttp.Handler {
	var (
		once sync.Once
		body []byte
		err  error
	)
	return func(c xhttp.Ctx) error {
		once.Do(func() {
			var doc *Document
			doc, err = g.Document(s)
			if err != nil {
				return
			}
			body, err = MarshalJSON(doc)
		})
		if err != nil {
			return err
		}
		c.Type("json", "utf-8")
		return c.Send(body)
	}
}
