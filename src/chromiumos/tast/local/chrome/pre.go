// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Pre provides access to an already-started Chrome instance as specified by a precondition.
//
// While adding a test, the testing.Test.Pre field may be set to a testing.Precondition:
//
//	func init() {
//		testing.AddTest(&testing.Test{
//			Func: VerifySomething,
//			Pre:  chrome.LoggedIn(),
//			...
//		})
//	}
//
// Later, in the test function, s.Pre() may be converted to *chrome.Pre to access the Chrome instance
// described by the precondition:
//
//	func VerifySomething(ctx context.Context, s *testing.State) {
//		cr := s.Pre().(*chrome.Pre).Chrome()
//		conn, err := cr.NewConn(ctx, "http://www.example.org/")
//		...
//	}
//
// While using a precondition, tests cannot call New; they should use the process returned by Chrome instead.
type Pre struct{ cr *Chrome }

// Chrome returns the Chrome object that should be used by tests using this precondition.
// The returned object cannot be closed by tests.
func (p *Pre) Chrome() *Chrome { return p.cr }

// LoggedIn returns a precondition that Chrome is already logged in when a test is run.
// See Pre for example usage.
func LoggedIn() testing.Precondition { return loggedInPre }

// loggedInPre is returned by LoggedIn.
var loggedInPre = &preImpl{
	name:    "chrome_logged_in",
	timeout: time.Minute,
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	name    string        // testing.PreconditionImpl.String
	timeout time.Duration // testing.PreconditionImpl.Timeout
	cr      *Chrome       // underlying Chrome instance
	opts    []option      // options that should be passed to New
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	defer func() { locked = true }()
	locked = false

	if p.cr != nil {
		if err := p.checkChrome(ctx); err != nil {
			s.Log("Existing Chrome connection is unusable: ", err)
		} else if err = p.resetChromeState(ctx); err != nil {
			s.Log("Failed resetting existing Chrome session: ", err)
		} else {
			s.Log("Reusing existing Chrome session")
			return &Pre{p.cr}
		}
		p.closeInternal(ctx, s)
	}

	var err error
	if p.cr, err = New(ctx, p.opts...); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	return &Pre{p.cr}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	locked = false
	p.closeInternal(ctx, s)
}

// closeInternal closes and resets p.cr if non-nil.
func (p *preImpl) closeInternal(ctx context.Context, s *testing.State) {
	if p.cr == nil {
		return
	}
	if err := p.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	p.cr = nil
}

// checkChrome performs basic checks to verify that cr is responsive.
func (p *preImpl) checkChrome(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	result := false
	if err = conn.Eval(ctx, "true", &result); err != nil {
		return err
	}
	if !result {
		return errors.New("eval 'true' returned false")
	}
	return nil
}

// resetChromeState attempts to reset state between tests.
func (p *preImpl) resetChromeState(ctx context.Context) error {
	testing.ContextLog(ctx, "Resetting Chrome's state")
	conn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	// TODO(derat): Find some way to close incognito windows: https://crbug.com/350379
	if err = conn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
  chrome.windows.getAll({}, (wins) => {
    const promises = [];
    for (const win of wins) {
      promises.push(new Promise((resolve, reject) => {
        chrome.windows.remove(win.id, resolve);
      }));
    }
    Promise.all(promises).then(resolve);
  });
})`, nil); err != nil {
		return errors.Wrap(err, "closing windows failed")
	}
	return nil
}
