// Run: go run ./examples/api
//
// Full xqua-go HTTP service: env-based config, lifecycle hooks, structured
// errors, response envelope, and request-scoped logging.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/InTacht/xqua-go/examples/api/ctx"
	"github.com/InTacht/xqua-go/examples/api/transport"
	"github.com/InTacht/xqua-go/pkg/service"
)

func main() {
	appName := env("APP_NAME", "api-example")

	appCtx := &ctx.Ctx{
		Name: appName,
		Host: env("APP_HOST", "0.0.0.0"),
		Port: envInt("APP_PORT", 8080),
	}

	srv := service.New(service.Config{
		Name:            appName,
		ID:              env("APP_ID", appName),
		Debug:           envBool("DEBUG", false),
		ShutdownTimeout: time.Duration(envInt("SHUTDOWN_TIMEOUT", 30)) * time.Second,
	}, appCtx)

	srv.OnStartup(func(context.Context) error {
		srv.Logger().Info("warming up dependencies")
		time.Sleep(200 * time.Millisecond)
		srv.Logger().Info("startup complete")
		return nil
	}).OnShutdown(func(ctx context.Context) error {
		srv.Logger().InfoCtx(ctx, "draining in-flight work")
		time.Sleep(500 * time.Millisecond)
		srv.Logger().InfoCtx(ctx, "drain complete")
		return nil
	})

	srv.Transport(transport.HTTP)

	if err := srv.Run(); err != nil {
		log.Printf("service stopped: %v", err)
		os.Exit(1)
	}
}
