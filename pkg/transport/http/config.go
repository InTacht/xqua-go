package http

import (
	"github.com/InTacht/xqua-go/pkg/logger"

	"github.com/gofiber/fiber/v3"
)

type Config struct {
	Host string
	Port int

	Logger *logger.Logger

	FiberConfig fiber.Config
}
