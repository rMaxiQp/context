package context

import (
	"testing"
	"time"
)

func TestEmptyContext(t *testing.T) {
	ctx := emptyCtx{}
	if _, ok := ctx.Deadline(); ok {
		t.Error("Deadline should return false")
	}
	if ctx.Done() != nil {
		t.Error("Done should return nil")
	}
	if ctx.Err() != nil {
		t.Error("Err should return nil")
	}
}

func TestCancelContext(t *testing.T) {
	parent := emptyCtx{}
	child, cancel := WithCancel(parent)
	if _, ok := child.Deadline(); ok {
		t.Error("Deadline should return false")
	}
	if child.Done() == nil {
		t.Error("Done should not return nil")
	}
	if child.Err() != nil {
		t.Error("Err should return nil")
	}
	cancel()
	if child.Err() == nil {
		t.Error("Err should not return nil")
	}
}

func TestDeadlineContext(t *testing.T) {
	parent := emptyCtx{}
	deadline := time.Now().Add(10 * time.Second)
	child, cancel := WithDeadline(parent, deadline)
	d, ok := child.Deadline()
	if !ok {
		t.Error("Deadline should return true")
	}
	if d != deadline {
		t.Errorf("Deadline should return %v", deadline)
	}
	if child.Done() == nil {
		t.Error("Done should not return nil")
	}
	if child.Err() != nil {
		t.Error("Err should return nil")
	}
	cancel()
	if child.Err() == nil {
		t.Error("Err should not return nil")
	}
}
