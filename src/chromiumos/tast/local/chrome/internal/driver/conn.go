// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package driver

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/input"
	"github.com/mafredri/cdp/protocol/media"
	"github.com/mafredri/cdp/protocol/profiler"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/testing"
)

// Conn represents a connection to a web content view, e.g. a tab.
type Conn struct {
	co        *cdputil.Conn
	lw        *jslog.Worker
	chromeErr func(error) error // wraps Chrome.chromeErr

	locked bool // if true, don't allow Close or CloseTarget to be called
}

// NewConn starts a new session using sm for communicating with the supplied target.
// pageURL is only used when logging JavaScript console messages via lm.
func NewConn(ctx context.Context, s *cdputil.Session, id target.ID,
	la *jslog.Aggregator, pageURL string, chromeErr func(error) error) (c *Conn, retErr error) {
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
		lw:        la.NewWorker(string(id), pageURL, ev),
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
//
// DEPRECATED: please use Eval(ctx, expr, nil) instead.
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
//
// If the expression is evaluated into a Promise instance, it will be awaited until
// it is settled, and the resolved value is stored in out if given.
//
//	data := make(map[string]interface{})
//	err := conn.EvalPromiseDeprecated(ctx,
//		`new Promise(function(resolve, reject) {
//			runAsync(function(data) {
//				if (data != null) {
//					resolve(data);
//				} else {
//					reject("it failed");
//				}
//			});
//		})`, &data)
func (c *Conn) Eval(ctx context.Context, expr string, out interface{}) error {
	return c.doEval(ctx, expr, true, out)
}

// EvalPromiseDeprecated evaluates the JavaScript expression expr (which must return a Promise),
// awaits its result, and stores the result in out (if non-nil). If out is a *chrome.JSObject,
// a reference to the result is returned. The *chrome.JSObject should get released or
// the memory it references will not be freed. An error is returned if evaluation fails,
// an exception is raised, ctx's deadline is reached, or out is non-nil and the result
// can't be unmarshaled into it.
//
//	data := make(map[string]interface{})
//	err := conn.EvalPromiseDeprecated(ctx,
//		`new Promise(function(resolve, reject) {
//			runAsync(function(data) {
//				if (data != null) {
//					resolve(data);
//				} else {
//					reject("it failed");
//				}
//			});
//		})`, &data)
//
// DEPRECATED: please use Eval, instead.
func (c *Conn) EvalPromiseDeprecated(ctx context.Context, expr string, out interface{}) error {
	return c.doEval(ctx, expr, true, out)
}

// doEval is a helper function that evaluates JavaScript code for Exec, Eval, and EvalPromiseDeprecated.
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

// Call applies fn to given args on this connection, then stores its result to out if given.
// fn must be a JavaScript expression which is evaluated to a (possibly async) function
// under the current execution context. If the function is async, or the function
// returns a Promise instance, it will be awaited until settled, and the resolved value
// will be stored into out, or error is reported if rejected.
// args must be either JSON serializable value, or *chrome.JSObject which is tied to
// the current conn.
// out must be either a pointer to the JSON deserialize typed data, *chrome.JSObject, or nil
// (if output should be ignored). If *chrome.JSObject is passed, the caller has the
// responsibility to call its Release() after its use.
//
// Examples:
//
//   // 1)  Calling a function. ret will be set to 30.
//   var ret int
//   if err := c.Call(ctx, &ret, "function(a, b) { return a + b; }", 10, 20); err != nil {
//      ...
//
//   // 2) Calling async function. ret will be set whether the given app is shown.
//   tconn, err := cr.TestAPIConn()
//   ...
//   var ret bool
//   if err := tconn.Call(ctx, &ret, "tast.promisify(chrome.autotestPrivate.isAppShown)", appID); err != nil {
//     ...
//
//   // 3) Serialize structure. Move the mouse to (100, 200) immediately.
//   loc := struct {
//     X double `json:"x"`
//     Y double `json:"y"`
//   } {
//     X: 100,
//     Y: 200,
//   }
//   if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.mouseMove)", &loc, 0 /* ms */); err != nil {
//     ...
//
//   // 4) Deserialize structure. Output can be JSON deserialized value.
//   var ret struct {
//     Provisioned bool `json:"provisioned"`
//     TOSNeeded bool `json:"tosNeeded"`
//   }
//   if err := tconn.Call(ctx, &ret, "tast.promisify(chrome.autotestPrivate.getArcState)"); err != nil {
//     ...
func (c *Conn) Call(ctx context.Context, out interface{}, fn string, args ...interface{}) error {
	// Either objectId or executionContextId should be specified to invoke Runtime.callFunctionOn.
	// Thus, take the "this" first, then call the method on the object.
	// cf) https://chromedevtools.github.io/devtools-protocol/tot/Runtime#method-callFunctionOn
	var this JSObject
	if err := c.Eval(ctx, "this", &this); err != nil {
		return err
	}
	defer func() {
		if err := this.Release(ctx); err != nil {
			// If an evaluated expression causes navigation or browser restart,
			// ReleaseObject may fail. Thus always report ReleaseObject errors
			// as informational. See also crbug.com/1193417.
			testing.ContextLog(ctx, "Ignored: failed to release 'this' object: ", err)
		}
	}()
	return this.Call(ctx, out, fn, args...)
}

// WaitForExpr repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// Errors returned by Eval are treated the same as expr == false.
func (c *Conn) WaitForExpr(ctx context.Context, expr string) error {
	return c.waitForExprImpl(ctx, expr, cdputil.ContinueOnError, 0)
}

// WaitForExprFailOnErr repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// It returns immediately if Eval returns an error.
func (c *Conn) WaitForExprFailOnErr(ctx context.Context, expr string) error {
	return c.waitForExprImpl(ctx, expr, cdputil.ExitOnError, 0)
}

// WaitForExprFailOnErrWithTimeout is the same as WaitForExprFailOnErr but will fail if timeout is exceeded.
func (c *Conn) WaitForExprFailOnErrWithTimeout(ctx context.Context, expr string, timeout time.Duration) error {
	return c.waitForExprImpl(ctx, expr, cdputil.ExitOnError, timeout)
}

// waitForExprImpl repeatedly evaluates the JavaScript expression expr until it evaluates to true.
// The behavior on evaluation errors depends on the value of exitOnError.
func (c *Conn) waitForExprImpl(ctx context.Context, expr string, ea cdputil.ErrorAction, timeout time.Duration) error {
	if err := c.co.WaitForExpr(ctx, expr, ea, timeout); err != nil {
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

// GetMediaPropertiesChangedObserver enables media logging for the current
// connection and retrieves a properties change observer.
func (c *Conn) GetMediaPropertiesChangedObserver(ctx context.Context) (observer media.PlayerPropertiesChangedClient, err error) {
	return c.co.GetMediaPropertiesChangedObserver(ctx)
}

// TestConn is a connection to the Tast test extension's background page.
// cf) crbug.com/1043590
type TestConn struct {
	*Conn
}

// ResetAutomation resets the automation API feature. The automation API feature
// is widely used to control the UI, but keeping it activated sometimes causes
// performance drawback on low-end devices. This method deactivates the
// automation API and resets internal states. See: https://crbug.com/1096719.
func (tconn *TestConn) ResetAutomation(ctx context.Context) error {
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.disableAutomation)()", nil); err != nil {
		return errors.Wrap(err, "failed to disable the automation feature")
	}

	// Reloading the test extension contents to clear all of Javascript objects.
	// This also resets the internal state of automation tree, so without
	// reloading, disableAutomation above would cause failures.
	if err := tconn.Eval(ctx, "location.reload()", nil); err != nil {
		return errors.Wrap(err, "failed to reload the testconn")
	}
	if err := tconn.WaitForExpr(ctx, "document.readyState == 'complete'"); err != nil {
		return errors.Wrap(err, "test API extension is unavailable")
	}

	if err := tconn.WaitForExpr(ctx, `typeof tast != 'undefined'`); err != nil {
		return errors.Wrap(err, "tast API is unavailable")
	}

	if err := tconn.Eval(ctx, "chrome.autotestPrivate.initializeEvents()", nil); err != nil {
		return errors.Wrap(err, "failed to initialize test API events")
	}
	return nil
}

// PrivateReleaseAllObjects releases all remote JavaScript objects not released yet.
// This function must kept private to the chrome package.
func PrivateReleaseAllObjects(ctx context.Context, c *Conn) error {
	return c.co.ReleaseAllObjects(ctx)
}
