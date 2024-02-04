package context

import (
	"errors"
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
	select {
	case <-child.Done():
		if child.Err() == nil {
			t.Error("Err should not return nil")
		}
	default:
		t.Error("child.Done should be closed")
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

func TestParentCancelContext(t *testing.T) {
	parent, cancel := WithCancel(emptyCtx{})
	child, _ := WithCancel(parent)
	cancel()
	select {
	case <-parent.Done():
	case <-time.After(1 * time.Second):
		t.Error("parent.Done should be closed")
	}
	if !errors.Is(parent.Err(), ErrCanceled) {
		t.Error("Err should return ErrCanceled")
	}

	select {
	case <-child.Done():
	case <-time.After(1 * time.Second):
		t.Error("child.Done should be closed")
	}
	if child.Err() == nil {
		t.Error("Err should not return nil")
	}
}

func TestParentDeadlineContext(t *testing.T) {
	parent, _ := WithDeadline(emptyCtx{}, time.Now().Add(5*time.Second))
	child, _ := WithDeadline(parent, time.Now().Add(1*time.Second))
	time.Sleep(3 * time.Second)
	if child.Err() == nil || !errors.Is(child.Err(), ErrDeadlineExceed) {
		t.Errorf("Err should return ErrDeadlineExceeded, got %v", child.Err())
	}
	if parent.Err() != nil {
		t.Error("parent.Err should return nil")
	}
	select {
	case <-child.Done():
		t.Error("child.Done should be closed")
	case <-parent.Done():
	default:
		t.Error("child.Done should be closed")
	}
}
