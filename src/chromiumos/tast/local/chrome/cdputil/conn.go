// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cdputil

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/protocol/dom"
	"github.com/mafredri/cdp/protocol/input"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/protocol/profiler"
	"github.com/mafredri/cdp/protocol/runtime"
	"github.com/mafredri/cdp/protocol/target"
	"github.com/mafredri/cdp/rpcc"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Conn is the connection to a web content view, e.g. a tab.
type Conn struct {
	co       *rpcc.Conn
	cl       *cdp.Client
	targetID target.ID
}

// NewConn creates a new connection to the given id.
func (s *Session) NewConn(ctx context.Context, id target.ID) (conn *Conn, retErr error) {
	testing.ContextLog(ctx, "Connecting to Chrome target ", string(id))
	co, err := s.manager.Dial(ctx, id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			co.Close()
		}
	}()

	cl := cdp.NewClient(co)
	if _, err := cl.Target.AttachToTarget(ctx, &target.AttachToTargetArgs{TargetID: id}); err != nil {
		return nil, err
	}

	if err := cl.Runtime.Enable(ctx); err != nil {
		return nil, err
	}

	if err := cl.Page.Enable(ctx); err != nil {
		return nil, err
	}

	return &Conn{
		co:       co,
		cl:       cl,
		targetID: id,
	}, nil
}

// Close releases the resources associated with the connection.
func (c *Conn) Close() error {
	// TODO(crbug.com/1020484): Return the error from rpcc.Conn.Close.
	// rpcc.Conn invokes Target.DetachFromTarget before closing the connection,
	// which fails if the target is already closed. This error is not a real
	// problem, but it can confuse cautious callers who check errors of Close.
	// See also an upstream bug: https://github.com/mafredri/cdp/issues/110
	c.co.Close()
	return nil
}

// ConsoleAPICalled creates a client for ConsoleAPICalled events.
func (c *Conn) ConsoleAPICalled(ctx context.Context) (runtime.ConsoleAPICalledClient, error) {
	return c.cl.Runtime.ConsoleAPICalled(ctx)
}

// CloseTarget closes the web content (e.g. tab) associated with c.
// Close must still be called to free associated resources.
// Tests should not feel obligated to call this to clean up.
func (c *Conn) CloseTarget(ctx context.Context) error {
	args := &target.CloseTargetArgs{TargetID: c.targetID}
	if reply, err := c.cl.Target.CloseTarget(ctx, args); err != nil {
		return err
	} else if !reply.Success {
		return errors.New("failed to close target")
	}
	return nil
}

// Eval evaluates the given JavaScript expression. If awaitPromise is set to true, this method
// waits until it is fulfilled. If out is given, the returned value is set.
// In case of JavaScript exceptions, errorText and exc are returned.
func (c *Conn) Eval(ctx context.Context, expr string, awaitPromise bool, out interface{}) (*runtime.EvaluateReply, error) {
	args := runtime.NewEvaluateArgs(expr)
	if awaitPromise {
		args = args.SetAwaitPromise(true)
	}
	if out != nil {
		args = args.SetReturnByValue(true)
	}

	repl, err := c.cl.Runtime.Evaluate(ctx, args)
	if err != nil {
		return nil, err
	}
	if exc := repl.ExceptionDetails; exc != nil {
		text := extractExceptionText(exc)
		return repl, errors.New(text)
	}
	if out == nil {
		return repl, nil
	}
	return repl, json.Unmarshal(repl.Result.Value, out)
}

// ReleaseObject releases the specified object.
func (c *Conn) ReleaseObject(ctx context.Context, object runtime.RemoteObject) error {
	if object.ObjectID != nil {
		args := runtime.NewReleaseObjectArgs(*object.ObjectID)
		return c.cl.Runtime.ReleaseObject(ctx, args)
	}
	return nil
}

// extractExceptionText extracts an error string from the exception described by d.
//
// The Chrome DevTools Protocol reports exceptions (and failed promises) in different ways depending
// on how they occur. This function tries to return a single-line string that contains the original error.
//
// Exec, Eval: throw new Error("foo"):
//	.Text:                  "Uncaught"
//	.Error:                 "runtime.ExceptionDetails: Uncaught exception at 0:0: Error: foo\n  <stack>"
//	.Exception.Description: "Error: foo\n  <stack>"
//	.Exception.Value:       null
//
// EvalPromise: reject("foo"):
//	.Text:                  "Uncaught (in promise)"
//	.Error:                 "runtime.ExceptionDetails: Uncaught (in promise) exception at 0:0"
//	.Exception.Description: nil
//	.Exception.Value:       "foo"
//
// EvalPromise: reject(new Error("foo")), throw new Error("foo"):
//	.Text:                  "Uncaught (in promise) Error: foo"
//	.Error:                 "runtime.ExceptionDetails: Uncaught (in promise) Error: foo exception at 0:0"
//	.Exception.Description: nil
//	.Exception.Value:       {}
func extractExceptionText(d *runtime.ExceptionDetails) string {
	if d.Exception != nil && d.Exception.Description != nil {
		return strings.Split(*d.Exception.Description, "\n")[0]
	}
	var s string
	if err := json.Unmarshal(d.Exception.Value, &s); err == nil {
		return d.Text + ": " + s
	}
	return d.Text
}

// ErrorAction defines the behavior of WaitForExpr if the given expression reports
// an exception.
type ErrorAction int

const (
	// ContinueOnError means to continue to poll the expression until satisfied (or timed out).
	ContinueOnError ErrorAction = iota

	// ExitOnError means to immediately return from the polling if an error is found.
	ExitOnError
)

// WaitForExpr repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// The behavior on evaluation errors depends on the value of ea.
func (c *Conn) WaitForExpr(ctx context.Context, expr string, ea ErrorAction) error {
	boolExpr := "!!(" + expr + ")"
	falseErr := errors.Errorf("%q is false", boolExpr)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		v := false
		if _, err := c.Eval(ctx, boolExpr, false, &v); err != nil {
			if ea == ExitOnError {
				return testing.PollBreak(err)
			}
			return err
		}
		if !v {
			return falseErr
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond}); err != nil {
		return err
	}
	return nil
}

// PageContent returns the current top-level page content.
func (c *Conn) PageContent(ctx context.Context) (string, error) {
	doc, err := c.cl.DOM.GetDocument(ctx, nil)
	if err != nil {
		return "", err
	}
	result, err := c.cl.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
		NodeID: &doc.Root.NodeID,
	})
	if err != nil {
		return "", err
	}
	return result.OuterHTML, nil
}

// Navigate navigates to url.
func (c *Conn) Navigate(ctx context.Context, url string) error {
	testing.ContextLog(ctx, "Navigating to ", url)
	fired, err := c.cl.Page.DOMContentEventFired(ctx)
	if err != nil {
		return err
	}
	defer fired.Close()

	if _, err := c.cl.Page.Navigate(ctx, page.NewNavigateArgs(url)); err != nil {
		return err
	}
	if _, err = fired.Recv(); err != nil {
		return err
	}
	return nil
}

// DispatchKeyEvent dispatches a key event to the page.
func (c *Conn) DispatchKeyEvent(ctx context.Context, args *input.DispatchKeyEventArgs) error {
	return c.cl.Input.DispatchKeyEvent(ctx, args)
}

// DispatchMouseEvent dispatches a mouse event to the page.
func (c *Conn) DispatchMouseEvent(ctx context.Context, args *input.DispatchMouseEventArgs) error {
	return c.cl.Input.DispatchMouseEvent(ctx, args)
}

// StartProfiling starts the profiling for current connection.
func (c *Conn) StartProfiling(ctx context.Context) error {
	if err := c.cl.Profiler.Enable(ctx); err != nil {
		return err
	}

	callCount := false
	detailed := true
	args := profiler.StartPreciseCoverageArgs{
		CallCount: &callCount,
		Detailed:  &detailed,
	}
	if err := c.cl.Profiler.StartPreciseCoverage(ctx, &args); err != nil {
		return err
	}

	return nil
}

// StopProfiling stops the profiling for current connection.
func (c *Conn) StopProfiling(ctx context.Context) (*profiler.TakePreciseCoverageReply, error) {
	reply, err := c.cl.Profiler.TakePreciseCoverage(ctx)
	if err != nil {
		return nil, err
	}

	if err := c.cl.Profiler.StopPreciseCoverage(ctx); err != nil {
		return nil, err
	}

	if err := c.cl.Profiler.Disable(ctx); err != nil {
		return nil, err
	}

	return reply, nil
}
