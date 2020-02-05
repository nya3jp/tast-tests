// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/input"
	"github.com/mafredri/cdp/protocol/profiler"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/jslog"
)

// Conn represents a connection to a web content view, e.g. a tab.
type Conn struct {
	co *cdputil.Conn

	lw *jslog.Worker

	locked bool // if true, don't allow Close or CloseTarget to be called

	chromeErr func(error) error // wraps Chrome.chromeErr
}

// newConn starts a new session using sm for communicating with the supplied target.
// pageURL is only used when logging JavaScript console messages via lm.
func newConn(ctx context.Context, s *cdputil.Session, id target.ID,
	lm *jslog.Master, pageURL string, chromeErr func(error) error) (c *Conn, retErr error) {
	co, err := s.NewConn(ctx, id)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			co.Close()
		}
	}()

	ev, err := co.ConsoleAPICalled(ctx)
	if err != nil {
		return nil, err
	}

	return &Conn{
		co:        co,
		lw:        lm.NewWorker(string(id), pageURL, ev),
		chromeErr: chromeErr,
	}, nil
}

// Close closes the connection to the target and frees related resources.
// Tests should typically defer calls to this method and ignore the returned error.
// This method does not close the web content itself; see CloseTarget for that.
func (c *Conn) Close() error {
	if c.locked {
		return errors.New("can't close locked connection")
	}
	c.lw.Close()
	return c.co.Close()
}

// CloseTarget closes the web content (e.g. tab) associated with c.
// Close must still be called to free associated resources.
// Tests should not feel obligated to call this to clean up.
func (c *Conn) CloseTarget(ctx context.Context) error {
	if c.locked {
		return errors.New("can't close target for locked connection")
	}
	return c.co.CloseTarget(ctx)
}

// Exec executes the JavaScript expression expr and discards its result.
// If out is a *chrome.JSObject, a reference to the result is returned.
// The *chrome.JSObject should get released or the memory it references will not be freed.
// An error is returned if an exception is generated.
func (c *Conn) Exec(ctx context.Context, expr string) error {
	return c.doEval(ctx, expr, false, nil)
}

// Eval evaluates the JavaScript expression expr and stores its result in out.
// If out is a *chrome.JSObject, a reference to the result is returned.
// The *chrome.JSObject should get released or the memory it references will not be freed.
// An error is returned if the result can't be unmarshaled into out.
//
//	sum := 0
//	err := conn.Eval(ctx, "3 + 4", &sum)
func (c *Conn) Eval(ctx context.Context, expr string, out interface{}) error {
	return c.doEval(ctx, expr, false, out)
}

// EvalPromise evaluates the JavaScript expression expr (which must return a Promise),
// awaits its result, and stores the result in out (if non-nil). If out is a *chrome.JSObject,
// a reference to the result is returned. The *chrome.JSObject should get released or
// the memory it references will not be freed. An error is returned if evaluation fails,
// an exception is raised, ctx's deadline is reached, or out is non-nil and the result
// can't be unmarshaled into it.
//
//	data := make(map[string]interface{})
//	err := conn.EvalPromise(ctx,
//		`new Promise(function(resolve, reject) {
//			runAsync(function(data) {
//				if (data != null) {
//					resolve(data);
//				} else {
//					reject("it failed");
//				}
//			});
//		})`, &data)
func (c *Conn) EvalPromise(ctx context.Context, expr string, out interface{}) error {
	return c.doEval(ctx, expr, true, out)
}

// doEval is a helper function that evaluates JavaScript code for Exec, Eval, and EvalPromise.
func (c *Conn) doEval(ctx context.Context, expr string, awaitPromise bool, out interface{}) error {
	// If returning JSObject, pass its RemoteObject to Eval.
	newOb, returnJSObject := out.(*JSObject)
	if returnJSObject {
		out = &newOb.ro
	}

	exc, err := c.co.Eval(ctx, expr, awaitPromise, out)
	if err != nil {
		if exc != nil {
			c.lw.Report(time.Now(), "eval-error", err.Error(), exc.StackTrace)
		}
		return err
	}
	if returnJSObject {
		newOb.conn = c
	}
	return nil
}

// WaitForExpr repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// Errors returned by Eval are treated the same as expr == false.
func (c *Conn) WaitForExpr(ctx context.Context, expr string) error {
	return c.waitForExprImpl(ctx, expr, cdputil.ContinueOnError)
}

// WaitForExprFailOnErr repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// It returns immediately if Eval returns an error.
func (c *Conn) WaitForExprFailOnErr(ctx context.Context, expr string) error {
	return c.waitForExprImpl(ctx, expr, cdputil.ExitOnError)
}

// waitForExprImpl repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// The behavior on evaluation errors depends on the value of exitOnError.
func (c *Conn) waitForExprImpl(ctx context.Context, expr string, ea cdputil.ErrorAction) error {
	if err := c.co.WaitForExpr(ctx, expr, ea); err != nil {
		return c.chromeErr(err)
	}
	return nil
}

// PageContent returns the current top-level page content.
func (c *Conn) PageContent(ctx context.Context) (string, error) {
	return c.co.PageContent(ctx)
}

// Navigate navigates to url.
func (c *Conn) Navigate(ctx context.Context, url string) error {
	return c.co.Navigate(ctx, url)
}

// DispatchKeyEvent executes a key event (i.e. arrowDown, Enter)
func (c *Conn) DispatchKeyEvent(ctx context.Context, args *input.DispatchKeyEventArgs) error {
	return c.co.DispatchKeyEvent(ctx, args)
}

// DispatchMouseEvent executes a mouse event (i.e. mouseMoves, mousePressed, mouseReleased)
func (c *Conn) DispatchMouseEvent(ctx context.Context, args *input.DispatchMouseEventArgs) error {
	return c.co.DispatchMouseEvent(ctx, args)
}

// StartProfiling enables the profiling of current connection.
func (c *Conn) StartProfiling(ctx context.Context) error {
	return c.co.StartProfiling(ctx)
}

// StopProfiling disables the profiling of current connection and returns the profiling result.
func (c *Conn) StopProfiling(ctx context.Context) (*profiler.TakePreciseCoverageReply, error) {
	return c.co.StopProfiling(ctx)
}

// Conns simply wraps a list of Conn and provides a method to Close all of them.
type Conns []*Conn

// Close closes all of the connections.
func (cs Conns) Close() error {
	var firstErr error
	numErrs := 0
	for _, c := range cs {
		if err := c.Close(); err != nil {
			numErrs++
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if numErrs == 0 {
		return nil
	}
	if numErrs == 1 {
		return firstErr
	}
	return errors.Wrapf(firstErr, "failed closing multiple connections: encountered %d errors; first one follows", numErrs)
}
