// Copyright 2016 aletheia7. All rights reserved. Use of this source code is
// governed by a BSD-2-Clause license that can be found in the LICENSE file.

// Package gogroup provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.

package gogroup

import (
	"context"
	"fmt"
	// "log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Line_end string

const (
	None      Line_end = ``
	Unix               = "\n"
	Osx                = "\r"
	Windows            = "\r\n"
	Backspace          = "\b\b"
)

type option func(o *Group)

// Use WithCancel() as the context to New(). If ctx is a *gogroup.Group, the
// parent will not Wait(). Use With_cancel_wait() to have a parent gogroup.Group
// wait on a child gogroup.Group.
//
func With_cancel_nowait(ctx context.Context) option {
	return func(o *Group) {
		if o.Context != nil {
			panic("context already set")
		}
		if ctx == nil {
			o.Context, o.CancelFunc = context.WithCancel(context.Background())
			return
		}
		o.Context, o.CancelFunc = context.WithCancel(ctx)
	}
}

// Use WithCancel() as the context to New(). Will panic if context is already
// set. Will panic if parent is nil. Register/Unregister use the parent's
// WaitGroup.
//
func With_cancel(parent *Group) option {
	return func(o *Group) {
		if o.Context != nil {
			panic("context already set")
		}
		if parent == nil {
			panic("parent is nil")
		}
		o.Context, o.CancelFunc = context.WithCancel(parent)
		o.parent = parent
	}
}

// Use WithTimeout() as the context to New().
//
func With_timeout_nowait(ctx context.Context, timeout time.Duration) option {
	return func(o *Group) {
		if o.Context != nil {
			panic("context already set")
		}
		if ctx == nil {
			o.Context, o.CancelFunc = context.WithTimeout(context.Background(), timeout)
			return
		}
		o.Context, o.CancelFunc = context.WithTimeout(ctx, timeout)
	}
}

// Will panic if gg is nil or context is already set.
//
func With_timeout(parent *Group, timeout time.Duration) option {
	return func(o *Group) {
		if o.Context != nil {
			panic("context already set")
		}
		if parent == nil {
			panic("parent is nil")
		}
		o.Context, o.CancelFunc = context.WithTimeout(parent, timeout)
		o.parent = parent
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
	local_wg      *sync.WaitGroup // No used when parent is present
	err_once      sync.Once
	err           error
	wait_lock     sync.Mutex
	wait_index    int
	wait_register map[int]bool
	sig           []os.Signal
	line_end      Line_end
}

// New returns a Group using with zero or more options. If a context is not
// provided in an option, With_cancel() will be used. The Group.Context is
// canceled when either a Go() func returns or a func using Register()/Unregister().
// New must be called to make a Group.
//
func New(opt ...option) *Group {
	r := &Group{wait_register: map[int]bool{}}
	for _, o := range opt {
		o(r)
	}
	if r.CancelFunc == nil {
		With_cancel_nowait(nil)(r)
	}
	if r.parent == nil {
		r.local_wg = &sync.WaitGroup{}
	}
	if r.sig != nil {
		r.wg().Add(1)
		go func() {
			defer r.wg().Done()
			ch := make(chan os.Signal, 1)
			defer close(ch)
			signal.Notify(ch, r.sig...)
			defer signal.Stop(ch)
			select {
			case <-r.Done():
			case <-ch:
				r.Interrupted = true
				fmt.Fprintf(os.Stderr, "%v", r.line_end)
			}
			r.Cancel()
		}()
	}
	return r
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

type Grouper interface {
	Run(g *Group)
}

// Register increments the internal sync.WaitGroup. Unregister() must be
// called with the returned int to end Group.Wait(). goroutines using
// Register/Unregister should end upon receipt from the Group.Ctx.Done()
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

// Unregister decrements the internal sync.WaitGroup and called
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

// Go calls the given function in a new goroutine.
// The first call to return cancels the group. A Grouper should receive on
// Group.Ctx.Done() to gracefully end.
//
func (o *Group) Go(f Grouper) {
	o.wg().Add(1)
	go func() {
		defer o.wg().Done()
		f.Run(o)
		o.Cancel()
	}()
}

// Set_err will set the returned error for the first time called.
//
func (o *Group) Set_err(err error) {
	o.err_once.Do(func() {
		o.err = err
	})
}

// Add_signals will call Group.Cancel() when a signal is received. Interrupted
// will be set to true.  An "end" character will be output to os.Stderr upon
// receiving a signal. if sig is absent, os.Interrupt and syscall.SIGTERM will
// be used.
//
func Add_signals(end Line_end, sig ...os.Signal) option {
	return func(o *Group) {
		o.line_end = end
		if len(sig) == 0 {
			o.sig = []os.Signal{os.Interrupt, syscall.SIGTERM}
			return
		}
		o.sig = sig
	}
}
