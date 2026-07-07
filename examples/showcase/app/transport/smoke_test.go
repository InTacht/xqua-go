package transport_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/InTacht/xqua-go/examples/showcase/app/transport"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/memory"
	authsvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/auth"
	itemsvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/item"
	usersvc "github.com/InTacht/xqua-go/examples/showcase/pkg/services/user"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/logger"
)

type stubUsers struct{}

func (stubUsers) GetByID(_ context.Context, id int64) (*domain.User, error) {
	return &domain.User{ID: id, Name: "Test", Email: "test@example.com"}, nil
}

func (stubUsers) List(_ context.Context, _ int) ([]domain.User, error) {
	return []domain.User{{ID: 1, Name: "Test", Email: "test@example.com"}}, nil
}

func (stubUsers) ListPaged(_ context.Context, _, _ int) ([]domain.User, int, error) {
	return []domain.User{{ID: 1, Name: "Test", Email: "test@example.com"}}, 1, nil
}

func (stubUsers) Update(_ context.Context, id int64, name, email string) (*domain.User, error) {
	return &domain.User{ID: id, Name: name, Email: email}, nil
}

type stubAudit struct{}

func (stubAudit) ListByUser(_ context.Context, _ int64, _ int) ([]domain.AuditEntry, error) {
	return nil, nil
}

func newSmokeTransport(t *testing.T) *http.Transport {
	t.Helper()
	log := logger.New(&logger.Config{Name: "showcase-smoke", ID: "smoke-1"})
	repo := &repository.Repo{
		Users:  stubUsers{},
		Items:  memory.NewItems(),
		Tokens: memory.NewKeys(),
		Audit:  stubAudit{},
	}
	repo.SetPing(func(context.Context) error { return nil })
	cfg := transport.Config{
		Host:    "127.0.0.1",
		Port:    8080,
		Version: "test",
		Name:    "showcase-smoke",
		Users:   usersvc.NewService(repo),
		Items:   itemsvc.NewService(repo),
		Auth:    authsvc.NewService(repo),
		Ping:    repo.Ping,
	}
	u := transport.HTTP(cfg, log)
	tr, ok := u.(*http.Transport)
	if !ok {
		t.Fatal("expected http transport")
	}
	return tr
}

type envelope struct {
	Status string `json:"status"`
	Errors []struct {
		Code string `json:"code"`
	} `json:"errors"`
}

func readEnvelope(t *testing.T, body io.Reader) envelope {
	t.Helper()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	var out envelope
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, raw)
	}
	return out
}

func TestShowcaseSmokeHealth(t *testing.T) {
	tr := newSmokeTransport(t)
	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/health", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestShowcaseSmokeDemoItemValidationCollection(t *testing.T) {
	tr := newSmokeTransport(t)
	req := httptest.NewRequest("POST", "/demo/items", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	out := readEnvelope(t, resp.Body)
	if len(out.Errors) != 2 || out.Errors[0].Code != "11002" || out.Errors[1].Code != "11003" {
		t.Fatalf("unexpected errors: %+v", out.Errors)
	}
}

func TestShowcaseSmokeAuthLoginValidation(t *testing.T) {
	tr := newSmokeTransport(t)
	req := httptest.NewRequest("POST", "/demo/auth/login", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
