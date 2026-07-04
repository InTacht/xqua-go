package env

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBindDefaultsAndOverrides(t *testing.T) {
	t.Setenv("BIND_HOST", "example.com")
	t.Setenv("BIND_PORT", "9090")

	type Config struct {
		Host     string        `env:"BIND_HOST" default:"0.0.0.0"`
		Port     int           `env:"BIND_PORT" default:"8080"`
		Version  string        `env:"BIND_VERSION" default:"dev"`
		Debug    bool          `env:"BIND_DEBUG" default:"true"`
		Shutdown time.Duration `env:"BIND_SHUTDOWN" default:"30s"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	if cfg.Host != "example.com" {
		t.Errorf("Host = %q, want example.com (from env)", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090 (from env)", cfg.Port)
	}
	if cfg.Version != "dev" {
		t.Errorf("Version = %q, want dev (default)", cfg.Version)
	}
	if !cfg.Debug {
		t.Errorf("Debug = %v, want true (default)", cfg.Debug)
	}
	if cfg.Shutdown != 30*time.Second {
		t.Errorf("Shutdown = %v, want 30s (default)", cfg.Shutdown)
	}
}

func TestBindRequiredMissingCollectsAll(t *testing.T) {
	type Config struct {
		DSN   string `env:"BIND_DSN,required"`
		Token string `env:"BIND_TOKEN,required"`
	}

	var cfg Config
	err := Bind(&cfg)
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BIND_DSN") || !strings.Contains(msg, "BIND_TOKEN") {
		t.Fatalf("error should mention both missing vars, got: %v", msg)
	}
}

func TestBindRequiredPresent(t *testing.T) {
	t.Setenv("BIND_DSN", "postgres://localhost")

	type Config struct {
		DSN string `env:"BIND_DSN,required"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.DSN != "postgres://localhost" {
		t.Errorf("DSN = %q", cfg.DSN)
	}
}

func TestBindInvalidValues(t *testing.T) {
	t.Setenv("BIND_PORT", "not-a-number")
	t.Setenv("BIND_DUR", "not-a-duration")

	type Config struct {
		Port int           `env:"BIND_PORT"`
		Dur  time.Duration `env:"BIND_DUR"`
	}

	var cfg Config
	err := Bind(&cfg)
	if err == nil {
		t.Fatal("expected error for invalid values")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BIND_PORT") || !strings.Contains(msg, "BIND_DUR") {
		t.Fatalf("error should mention both invalid vars, got: %v", msg)
	}
}

func TestBindSkipsUntaggedAndUnexported(t *testing.T) {
	t.Setenv("BIND_NAME", "svc")

	type Config struct {
		Name     string `env:"BIND_NAME"`
		Internal string `env:"-"`
		NoTag    string
		secret   string //nolint:unused // verifies unexported fields are skipped
	}

	cfg := Config{Internal: "keep", NoTag: "keep"}
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Name != "svc" {
		t.Errorf("Name = %q, want svc", cfg.Name)
	}
	if cfg.Internal != "keep" || cfg.NoTag != "keep" {
		t.Errorf("untagged fields were modified: %+v", cfg)
	}
	_ = cfg.secret
}

func TestBindRejectsNonPointer(t *testing.T) {
	type Config struct{}
	if err := Bind(Config{}); err == nil {
		t.Fatal("expected error for non-pointer argument")
	}
	var p *Config
	if err := Bind(p); err == nil {
		t.Fatal("expected error for nil pointer argument")
	}
}

func TestBindUintAndFloat(t *testing.T) {
	t.Setenv("BIND_WORKERS", "8")
	t.Setenv("BIND_RATE", "1.5")

	type Config struct {
		Workers uint    `env:"BIND_WORKERS"`
		Rate    float64 `env:"BIND_RATE"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Workers != 8 {
		t.Errorf("Workers = %d, want 8", cfg.Workers)
	}
	if cfg.Rate != 1.5 {
		t.Errorf("Rate = %v, want 1.5", cfg.Rate)
	}
}

func TestBindOverflowReported(t *testing.T) {
	t.Setenv("BIND_SMALL", "99999")
	t.Setenv("BIND_NEG", "-1")

	type Config struct {
		Small int8  `env:"BIND_SMALL"`
		Neg   uint8 `env:"BIND_NEG"`
	}

	var cfg Config
	err := Bind(&cfg)
	if err == nil {
		t.Fatal("expected error for out-of-range values")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BIND_SMALL") || !strings.Contains(msg, "BIND_NEG") {
		t.Fatalf("error should mention both out-of-range vars, got: %v", msg)
	}
}

func TestBindTextUnmarshaler(t *testing.T) {
	t.Setenv("BIND_STARTED", "2026-07-04T05:00:00Z")
	t.Setenv("BIND_BIND_IP", "10.0.0.1")

	type Config struct {
		StartedAt time.Time `env:"BIND_STARTED"`
		BindIP    net.IP    `env:"BIND_BIND_IP"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if !cfg.StartedAt.Equal(time.Date(2026, 7, 4, 5, 0, 0, 0, time.UTC)) {
		t.Errorf("StartedAt = %v", cfg.StartedAt)
	}
	if !cfg.BindIP.Equal(net.ParseIP("10.0.0.1")) {
		t.Errorf("BindIP = %v, want 10.0.0.1", cfg.BindIP)
	}
}

func TestBindTextUnmarshalerInvalid(t *testing.T) {
	t.Setenv("BIND_STARTED", "not-a-timestamp")

	type Config struct {
		StartedAt time.Time `env:"BIND_STARTED"`
	}

	var cfg Config
	if err := Bind(&cfg); err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestBindSlices(t *testing.T) {
	t.Setenv("BIND_HOSTS", "a.example.com, b.example.com ,c.example.com")
	t.Setenv("BIND_PORTS", "8080,8081")

	type Config struct {
		Hosts   []string `env:"BIND_HOSTS"`
		Ports   []int    `env:"BIND_PORTS"`
		Fallbck []string `env:"BIND_MISSING" default:"x,y"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	wantHosts := []string{"a.example.com", "b.example.com", "c.example.com"}
	if len(cfg.Hosts) != len(wantHosts) {
		t.Fatalf("Hosts = %v, want %v", cfg.Hosts, wantHosts)
	}
	for i := range wantHosts {
		if cfg.Hosts[i] != wantHosts[i] {
			t.Fatalf("Hosts = %v, want %v", cfg.Hosts, wantHosts)
		}
	}
	if len(cfg.Ports) != 2 || cfg.Ports[0] != 8080 || cfg.Ports[1] != 8081 {
		t.Errorf("Ports = %v, want [8080 8081]", cfg.Ports)
	}
	if len(cfg.Fallbck) != 2 || cfg.Fallbck[0] != "x" || cfg.Fallbck[1] != "y" {
		t.Errorf("Fallbck = %v, want [x y]", cfg.Fallbck)
	}
}

func TestBindSliceElementError(t *testing.T) {
	t.Setenv("BIND_PORTS", "8080,nope")

	type Config struct {
		Ports []int `env:"BIND_PORTS"`
	}

	var cfg Config
	err := Bind(&cfg)
	if err == nil {
		t.Fatal("expected error for invalid slice element")
	}
	if !strings.Contains(err.Error(), "BIND_PORTS") {
		t.Fatalf("error should mention BIND_PORTS, got: %v", err)
	}
}

func TestBindNamedStringType(t *testing.T) {
	t.Setenv("BIND_LEVEL", "debug")

	type Level string
	type Config struct {
		Level Level `env:"BIND_LEVEL" default:"info"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Level != "debug" {
		t.Errorf("Level = %q, want debug", cfg.Level)
	}
}

func TestBindRejectsPointerToNonStruct(t *testing.T) {
	n := 1
	if err := Bind(&n); err == nil {
		t.Fatal("expected error for pointer to non-struct")
	}
}

func TestBindEmptyEnvUsesDefault(t *testing.T) {
	t.Setenv("BIND_EMPTY_HOST", "")

	type Config struct {
		Host string `env:"BIND_EMPTY_HOST" default:"fallback"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Host != "fallback" {
		t.Errorf("Host = %q, want fallback", cfg.Host)
	}
}

func TestBindEmptyEnvRequiredFails(t *testing.T) {
	t.Setenv("BIND_EMPTY_REQ", "")

	type Config struct {
		Token string `env:"BIND_EMPTY_REQ,required"`
	}

	var cfg Config
	if err := Bind(&cfg); err == nil {
		t.Fatal("expected error for empty required var")
	}
}

func TestBindUnsetKeepsZeroValue(t *testing.T) {
	type Config struct {
		Port  int    `env:"BIND_ZERO_PORT"`
		Debug bool   `env:"BIND_ZERO_DEBUG"`
		Name  string `env:"BIND_ZERO_NAME"`
	}

	cfg := Config{Port: 1, Debug: true, Name: "preset"}
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	// Unset fields without defaults must not be overwritten.
	if cfg.Port != 1 || !cfg.Debug || cfg.Name != "preset" {
		t.Errorf("zero-path mutated fields: %+v", cfg)
	}
}

func TestBindInvalidBoolAndFloat(t *testing.T) {
	t.Setenv("BIND_BAD_BOOL", "maybe")
	t.Setenv("BIND_BAD_FLOAT", "nope")

	type Config struct {
		Debug bool    `env:"BIND_BAD_BOOL"`
		Rate  float64 `env:"BIND_BAD_FLOAT"`
	}

	var cfg Config
	err := Bind(&cfg)
	if err == nil {
		t.Fatal("expected error for invalid bool and float")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BIND_BAD_BOOL") || !strings.Contains(msg, "BIND_BAD_FLOAT") {
		t.Fatalf("error should mention both vars, got: %v", msg)
	}
}

func TestBindDurationFromEnv(t *testing.T) {
	t.Setenv("BIND_TIMEOUT", "5m")

	type Config struct {
		Timeout time.Duration `env:"BIND_TIMEOUT"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", cfg.Timeout)
	}
}

func TestBindAllIntegerWidths(t *testing.T) {
	t.Setenv("BIND_I8", "8")
	t.Setenv("BIND_I16", "16")
	t.Setenv("BIND_I32", "32")
	t.Setenv("BIND_I64", "64")
	t.Setenv("BIND_U16", "16")
	t.Setenv("BIND_U32", "32")
	t.Setenv("BIND_U64", "64")
	t.Setenv("BIND_F32", "1.25")

	type Config struct {
		I8  int8    `env:"BIND_I8"`
		I16 int16   `env:"BIND_I16"`
		I32 int32   `env:"BIND_I32"`
		I64 int64   `env:"BIND_I64"`
		U16 uint16  `env:"BIND_U16"`
		U32 uint32  `env:"BIND_U32"`
		U64 uint64  `env:"BIND_U64"`
		F32 float32 `env:"BIND_F32"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.I8 != 8 || cfg.I16 != 16 || cfg.I32 != 32 || cfg.I64 != 64 {
		t.Errorf("signed ints: %+v", cfg)
	}
	if cfg.U16 != 16 || cfg.U32 != 32 || cfg.U64 != 64 {
		t.Errorf("unsigned ints: %+v", cfg)
	}
	if cfg.F32 != 1.25 {
		t.Errorf("F32 = %v, want 1.25", cfg.F32)
	}
}

func TestBindUnsupportedType(t *testing.T) {
	t.Setenv("BIND_MAP", "a=b")

	type Config struct {
		Labels map[string]string `env:"BIND_MAP"`
	}

	var cfg Config
	err := Bind(&cfg)
	if err == nil {
		t.Fatal("expected error for unsupported map type")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error should mention unsupported, got: %v", err)
	}
}

func TestBindTagOptions(t *testing.T) {
	t.Setenv("BIND_OPT", "ok")

	// Unknown options are ignored; whitespace around "required" is accepted.
	type Config struct {
		Value string `env:"BIND_OPT, foo ,required"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Value != "ok" {
		t.Errorf("Value = %q, want ok", cfg.Value)
	}
}

func TestBindEmptySlice(t *testing.T) {
	// Explicit empty default yields an empty (non-nil) slice.
	type Config struct {
		Hosts []string `env:"BIND_EMPTY_SLICE" default:""`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Hosts == nil {
		t.Fatal("Hosts is nil, want empty slice")
	}
	if len(cfg.Hosts) != 0 {
		t.Errorf("Hosts = %v, want empty", cfg.Hosts)
	}
}

func TestBindWhitespaceOnlySlice(t *testing.T) {
	t.Setenv("BIND_WS_SLICE", "   ")

	type Config struct {
		Hosts []string `env:"BIND_WS_SLICE"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Hosts == nil || len(cfg.Hosts) != 0 {
		t.Errorf("Hosts = %v, want empty non-nil slice", cfg.Hosts)
	}
}

func TestBindSliceOfDurations(t *testing.T) {
	t.Setenv("BIND_BACKOFFS", "1s, 500ms, 2s")

	type Config struct {
		Backoffs []time.Duration `env:"BIND_BACKOFFS"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	want := []time.Duration{time.Second, 500 * time.Millisecond, 2 * time.Second}
	if len(cfg.Backoffs) != len(want) {
		t.Fatalf("Backoffs = %v, want %v", cfg.Backoffs, want)
	}
	for i := range want {
		if cfg.Backoffs[i] != want[i] {
			t.Fatalf("Backoffs = %v, want %v", cfg.Backoffs, want)
		}
	}
}

func TestBindCustomTextUnmarshaler(t *testing.T) {
	t.Setenv("BIND_COLOR", "blue")

	type Config struct {
		Color color `env:"BIND_COLOR"`
	}

	var cfg Config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if cfg.Color != "blue" {
		t.Errorf("Color = %q, want blue", cfg.Color)
	}
}

func TestBindCustomTextUnmarshalerInvalid(t *testing.T) {
	t.Setenv("BIND_COLOR", "purple")

	type Config struct {
		Color color `env:"BIND_COLOR"`
	}

	var cfg Config
	if err := Bind(&cfg); err == nil {
		t.Fatal("expected error for invalid color")
	}
}

// color is a user-defined TextUnmarshaler used to exercise the custom-type path.
type color string

func (c *color) UnmarshalText(text []byte) error {
	switch s := string(text); s {
	case "red", "green", "blue":
		*c = color(s)
		return nil
	default:
		return errors.New("unknown color")
	}
}
