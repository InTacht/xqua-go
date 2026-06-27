package errors

import "testing"

func TestCloneNil(t *testing.T) {
	if clone(nil) != nil {
		t.Fatal("clone(nil) should return nil")
	}
}
