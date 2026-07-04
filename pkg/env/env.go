// Package env reads process environment variables with typed defaults.
//
// It is a thin convenience for program main packages that assemble config from
// the environment. The library core stays config-first and never reads the
// environment itself; env is opt-in glue for the program that wires a runtime.
//
// Read individual variables with the typed helpers:
//
//	cfg := Config{
//	    Host:  env.String("APP_HOST", "0.0.0.0"),
//	    Port:  env.Int("APP_PORT", 8080),
//	    Debug: env.Bool("DEBUG", false),
//	    DSN:   env.MustString("DATABASE_URL"),
//	}
//
// Or declare the whole config as a tagged struct and populate it in one call
// with Bind, which reports all missing or invalid variables together:
//
//	type Config struct {
//	    Host string `env:"APP_HOST" default:"0.0.0.0"`
//	    Port int    `env:"APP_PORT" default:"8080"`
//	    DSN  string `env:"DATABASE_URL,required"`
//	}
//
//	var cfg Config
//	if err := env.Bind(&cfg); err != nil {
//	    log.Fatal(err)
//	}
package env

import (
	"fmt"
	"os"
	"strconv"
)

// String returns the value of key, or def when key is unset or empty.
func String(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Int returns the value of key parsed as an int, or def when key is unset,
// empty, or not a valid integer.
func Int(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Bool returns the value of key parsed as a bool (via strconv.ParseBool), or
// def when key is unset, empty, or not a valid boolean.
func Bool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// MustString returns the value of key and panics when it is unset or empty. Use
// it for required configuration that has no sensible default.
func MustString(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("env: required variable %q is not set", key))
	}
	return v
}
