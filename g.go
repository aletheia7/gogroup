// Copyright 2016 aletheia7. All rights reserved. Use of this source code is
// governed by a BSD-2-Clause license that can be found in the LICENSE file.

// Package gogroup provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.

package gogroup

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type option func(o *Group)

// Use With_cancel_nowait() as the context to New(). If ctx is a *gogroup.Group, the
// parent will not Wait().
//
// parent goroutine will not wait on the child.
// parent.Cancel() will call child.Cancel().
// child.Cancel() will not call parent.Cancel().
//
func With_cancel_nowait(ctx context.Context) option {
	return func(o *Group) {
		switch {
		case o.Context != nil:
			panic("context already set")
		case ctx == nil:
			o.Context, o.CancelFunc = context.WithCancel(context.Background())
		default:
			o.Context, o.CancelFunc = context.WithCancel(ctx)
		}
	}
}

// Use With_cancel() as the context to New(). Will panic if context is already
// set. Will panic if parent is nil. Register/Unregister use the parent's
// WaitGroup.
//
// parent goroutine will not wait on the child.
// parent.Cancel() will call child.Cancel().
// child.Cancel() will not call parent.Cancel().
//
func With_cancel(parent *Group) option {
	return func(o *Group) {
		switch {
		case o.Context != nil:
			panic("context already set")
		case parent == nil:
			panic("parent is nil")
		default:
			o.Context, o.CancelFunc = context.WithCancel(parent)
			o.parent = parent
		}
	}
}

// Use With_timeout_nowait() as the context to New().
//
// parent goroutine will not wait on the child.
// parent.Cancel() will call child.Cancel().
// child.Cancel() will not call parent.Cancel().
// child timeout will not cancel parent.
//
func With_timeout_nowait(ctx context.Context, timeout time.Duration) option {
	return func(o *Group) {
		switch {
		case o.Context != nil:
			panic("context already set")
		case ctx == nil:
			o.Context, o.CancelFunc = context.WithTimeout(context.Background(), timeout)
		default:
			o.Context, o.CancelFunc = context.WithTimeout(ctx, timeout)
		}
	}
}

// Will panic if gg is nil or context is already set.
//
// parent.Cancel() will call child.Cancel().
// child.Cancel() will not call parent.Cancel().
// child timeout will not cancel parent.
// child timeout will not cancel parent.
//
func With_timeout(parent *Group, timeout time.Duration) option {
	return func(o *Group) {
		switch {
		case o.Context != nil:
			panic("context already set")
		case parent == nil:
			panic("parent is nil")
		default:
			o.Context, o.CancelFunc = context.WithTimeout(parent, timeout)
			o.parent = parent
		}
	}
}

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
type Group struct {
	context.Context
	context.CancelFunc
	Interrupted   bool
	parent        *Group
	local_wg      *sync.WaitGroup // Not used when parent is present
	err_once      sync.Once
	err           error
	wait_lock     sync.Mutex
	wait_index    int
	wait_register map[int]bool
}

// New returns a Group using with zero or more options. If a context is not
// provided in an option, With_cancel() will be used. The Group.Context is
// canceled when either a Go() func returns or a func using Register()/Unregister().
// New must be called to make a Group.
//
func New(opt ...option) (r *Group) {
	r = &Group{wait_register: map[int]bool{}}
	for _, o := range opt {
		o(r)
	}
	if r.CancelFunc == nil {
		With_cancel_nowait(context.Background())(r)
	}
	if r.parent == nil {
		r.local_wg = &sync.WaitGroup{}
	}
	r.wg().Add(1)
	go func() {
		defer r.wg().Done()
		ch := make(chan os.Signal, 1)
		defer close(ch)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)
		select {
		case <-r.Done():
		case <-ch:
			r.Interrupted = true
			fmt.Fprintf(os.Stderr, "%v", Line_end)
		}
		r.Cancel()
	}()
	return
}

func (o *Group) Cancel() {
	if o.CancelFunc != nil {
		o.CancelFunc()
	}
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
//
func (o *Group) Wait() error {
	<-o.Done()
	o.wg().Wait()
	o.Cancel()
	return o.err
}

func (o *Group) wg() *sync.WaitGroup {
	if o.parent != nil {
		return o.parent.wg()
	}
	return o.local_wg
}

// Register increments the internal sync.WaitGroup. Unregister() must be
// called with the returned int to end Group.Wait(). goroutines using
// Register/Unregister must end upon receipt from the Group.Ctx.Done()
// channel.
//
func (o *Group) Register() int {
	o.wait_lock.Lock()
	defer o.wait_lock.Unlock()
	o.wg().Add(1)
	o.wait_index++
	o.wait_register[o.wait_index] = true
	return o.wait_index
}

// Unregister decrements the internal sync.WaitGroup and calls
// Group.Cancel(). It is safe to call Unregister multiple times.
//
func (o *Group) Unregister(index int) {
	o.wait_lock.Lock()
	defer o.wait_lock.Unlock()
	if o.wait_register[index] {
		delete(o.wait_register, index)
		o.wg().Done()
		o.Cancel()
	}
}

// Set_err will return the first called
//
func (o *Group) Set_err(err error) {
	o.err_once.Do(func() {
		o.err = err
	})
}

func (o *Group) Get_err() error {
	return o.err
}
