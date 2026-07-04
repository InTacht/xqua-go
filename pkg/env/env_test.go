package env

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	t.Setenv("ENV_STR", "hello")

	if got := String("ENV_STR", "def"); got != "hello" {
		t.Errorf("set: got %q, want hello", got)
	}
	if got := String("ENV_STR_MISSING", "def"); got != "def" {
		t.Errorf("unset: got %q, want def", got)
	}

	t.Setenv("ENV_STR_EMPTY", "")
	if got := String("ENV_STR_EMPTY", "def"); got != "def" {
		t.Errorf("empty: got %q, want def", got)
	}
}

func TestInt(t *testing.T) {
	t.Setenv("ENV_INT", "42")
	if got := Int("ENV_INT", 7); got != 42 {
		t.Errorf("set: got %d, want 42", got)
	}
	if got := Int("ENV_INT_MISSING", 7); got != 7 {
		t.Errorf("unset: got %d, want 7", got)
	}

	t.Setenv("ENV_INT_EMPTY", "")
	if got := Int("ENV_INT_EMPTY", 7); got != 7 {
		t.Errorf("empty: got %d, want 7", got)
	}

	t.Setenv("ENV_INT_BAD", "nope")
	if got := Int("ENV_INT_BAD", 7); got != 7 {
		t.Errorf("invalid: got %d, want 7", got)
	}
}

func TestBool(t *testing.T) {
	cases := []struct {
		key, val  string
		def, want bool
	}{
		{"ENV_BOOL_T", "true", false, true},
		{"ENV_BOOL_F", "false", true, false},
		{"ENV_BOOL_1", "1", false, true},
		{"ENV_BOOL_0", "0", true, false},
	}
	for _, tc := range cases {
		t.Setenv(tc.key, tc.val)
		if got := Bool(tc.key, tc.def); got != tc.want {
			t.Errorf("%s=%q: got %v, want %v", tc.key, tc.val, got, tc.want)
		}
	}

	if got := Bool("ENV_BOOL_MISSING", true); !got {
		t.Errorf("unset: got %v, want true", got)
	}

	t.Setenv("ENV_BOOL_EMPTY", "")
	if got := Bool("ENV_BOOL_EMPTY", true); !got {
		t.Errorf("empty: got %v, want true", got)
	}

	t.Setenv("ENV_BOOL_BAD", "maybe")
	if got := Bool("ENV_BOOL_BAD", true); !got {
		t.Errorf("invalid: got %v, want true", got)
	}
}

func TestMustString(t *testing.T) {
	t.Setenv("ENV_MUST", "present")
	if got := MustString("ENV_MUST"); got != "present" {
		t.Errorf("got %q, want present", got)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unset variable")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "ENV_MUST_MISSING") {
			t.Fatalf("panic = %v, want mention of ENV_MUST_MISSING", r)
		}
	}()
	MustString("ENV_MUST_MISSING")
}

func TestMustStringEmptyPanics(t *testing.T) {
	t.Setenv("ENV_MUST_EMPTY", "")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty variable")
		}
	}()
	MustString("ENV_MUST_EMPTY")
}
