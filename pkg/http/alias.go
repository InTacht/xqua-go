package http

import "github.com/gofiber/fiber/v3"

// Ctx is the request context passed to handlers. It is an alias of fiber.Ctx
// so application code can depend on this package alone for route handlers.
type Ctx = fiber.Ctx

// Handler serves an HTTP request.
type Handler = func(Ctx) error
