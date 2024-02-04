// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context defines the Context type, which carries deadlines,
// cancellation signals, and other request-scoped values across API boundaries
// and between processes.
//
// Incoming requests to a server should create a [Context], and outgoing
// calls to servers should accept a Context. The chain of function
// calls between them must propagate the Context, optionally replacing
// it with a derived Context created using [WithCancel], [WithDeadline],
// [WithTimeout], or [WithValue]. When a Context is canceled, all
// Contexts derived from it are also canceled.
//
// The [WithCancel], [WithDeadline], and [WithTimeout] functions take a
// Context (the parent) and return a derived Context (the child) and a
// [CancelFunc]. Calling the CancelFunc cancels the child and its
// children, removes the parent's reference to the child, and stops
// any associated timers. Failing to call the CancelFunc leaks the
// child and its children until the parent is canceled or the timer
// fires. The go vet tool checks that CancelFuncs are used on all
// control-flow paths.
//
// The [WithCancelCause] function returns a [CancelCauseFunc], which
// takes an error and records it as the cancellation cause. Calling
// [Cause] on the canceled context or any of its children retrieves
// the cause. If no cause is specified, Cause(ctx) returns the same
// value as ctx.Err().
//
// Programs that use Contexts should follow these rules to keep interfaces
// consistent across packages and enable static analysis tools to check context
// propagation:
//
// Do not store Contexts inside a struct type; instead, pass a Context
// explicitly to each function that needs it. The Context should be the first
// parameter, typically named ctx:
//
//	func DoSomething(ctx context.Context, arg Arg) error {
//		// ... use ctx ...
//	}
//
// Do not pass a nil [Context], even if a function permits it. Pass [context.TODO]
// if you are unsure about which Context to use.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The same Context may be passed to functions running in different goroutines;
// Contexts are safe for simultaneous use by multiple goroutines.
//
// See https://blog.golang.org/context for example code for a server that uses
// Contexts.
package context

import (
	"errors"
	"time"
)

var (
	ErrCanceled       = errors.New("context canceled")
	ErrDeadlineExceed = errors.New("context deadline exceeded")
)

// A Context carries a deadline, a cancellation signal, and other values across
// API boundaries.
//
// Context's methods may be called by multiple goroutines simultaneously.
type Context interface {
	// Deadline returns the time when work done on behalf of this context
	// should be canceled. Deadline returns ok==false when no deadline is
	// set. Successive calls to Deadline return the same results.
	Deadline() (deadline time.Time, ok bool)

	// Done returns a channel that's closed when work done on behalf of this
	// context should be canceled. Done may return nil if this context can
	// never be canceled. Successive calls to Done return the same value.
	// The close of the Done channel may happen asynchronously,
	// after the cancel function returns.
	//
	// WithCancel arranges for Done to be closed when cancel is called;
	// WithDeadline arranges for Done to be closed when the deadline
	// expires; WithTimeout arranges for Done to be closed when the timeout
	// elapses.
	//
	// Done is provided for use in select statements:
	//
	//  // Stream generates values with DoSomething and sends them to out
	//  // until DoSomething returns an error or ctx.Done is closed.
	//  func Stream(ctx context.Context, out chan<- Value) error {
	//  	for {
	//  		v, err := DoSomething(ctx)
	//  		if err != nil {
	//  			return err
	//  		}
	//  		select {
	//  		case <-ctx.Done():
	//  			return ctx.Err()
	//  		case out <- v:
	//  		}
	//  	}
	//  }
	//
	// See https://blog.golang.org/pipelines for more examples of how to use
	// a Done channel for cancellation.
	Done() <-chan struct{}

	// If Done is not yet closed, Err returns nil.
	// If Done is closed, Err returns a non-nil error explaining why:
	// Canceled if the context was canceled
	// or DeadlineExceeded if the context's deadline passed.
	// After Err returns a non-nil error, successive calls to Err return the same error.
	Err() error
}

type emptyCtx struct{}

var _ Context = (*emptyCtx)(nil)

func (emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (emptyCtx) Done() <-chan struct{} {
	return nil
}

func (emptyCtx) Err() error {
	return nil
}

type CancelFunc func()

type CancelCtx struct {
	parent Context
	done   chan struct{}
	err    error
}

var _ Context = (*CancelCtx)(nil)

func (c *CancelCtx) Deadline() (deadline time.Time, ok bool) {
	return c.parent.Deadline()
}

func (c *CancelCtx) Done() <-chan struct{} {
	if c.parent.Done() == nil {
		return c.done
	}
	select {
	case <-c.parent.Done():
		c.cancelWithCause(c.parent.Err())()
	default:
	}
	return c.done
}

func (c *CancelCtx) Err() error {
	return c.err
}

// WithCancel returns a copy of parent with a new Done channel. The returned
// context's Done channel is closed when the returned cancel function is called
// or when the parent context's Done channel is closed, whichever happens first.
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	c := &CancelCtx{
		parent: parent,
		done:   make(chan struct{}, 1),
	}
	c.done <- struct{}{}
	return c, c.cancelWithCause(ErrCanceled)
}

func (c *CancelCtx) cancelWithCause(err error) func() {
	return func() {
		select {
		case <-c.done:
			close(c.done)
			c.err = err
		default:
		}
	}
}

type DeadlineCtx struct {
	parent   Context
	deadline time.Time
	done     chan struct{}
	err      error
}

var _ Context = (*DeadlineCtx)(nil)

func (c DeadlineCtx) Deadline() (deadline time.Time, ok bool) {
	return c.deadline, true
}

func (c DeadlineCtx) Done() <-chan struct{} {
	select {
	case <-c.parent.Done():
		return c.parent.Done()
	default:
	}
	return c.done
}

func (c DeadlineCtx) Err() error {
	return c.err
}

// WithDeadline returns a copy of the parent context with the deadline adjusted
// to be no later than d. If the parent's deadline is already earlier than d,
// WithDeadline(parent, d) is semantically equivalent to parent. The returned
// [Context.Done] channel is closed when the deadline expires, when the returned
// cancel function is called, or when the parent context's Done channel is
// closed, whichever happens first.
func WithDeadline(parent Context, deadline time.Time) (Context, CancelFunc) {
	c := &DeadlineCtx{
		parent:   parent,
		deadline: deadline,
		done:     make(chan struct{}, 1),
	}
	t, ok := parent.Deadline()
	if ok && t.Before(deadline) {
		c.deadline = t
	}
	c.done <- struct{}{}
	cancel := time.AfterFunc(time.Until(c.deadline), c.cancelWithCause(ErrDeadlineExceed))
	f := func() {
		if !cancel.Stop() {
			return
		}
		c.cancelWithCause(ErrCanceled)()
	}
	return c, f
}

func (c *DeadlineCtx) cancelWithCause(err error) func() {
	return func() {
		select {
		case <-c.done:
			close(c.done)
			c.err = err
		default:
		}
	}
}

// WithTimeout returns WithDeadline(parent, time.Now().Add(timeout)).
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this [Context] complete:
//
//	func slowOperationWithTimeout(ctx context.Context) (Result, error) {
//		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
//		defer cancel()  // releases resources if slowOperation completes before timeout elapses
//		return slowOperation(ctx)
//	}
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}
