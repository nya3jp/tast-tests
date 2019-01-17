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

// LoggedIn returns a precondition that provides access to an already-logged-in Chrome process.
// Tests must list this in their Pre field during registration before using it during testing.
func LoggedIn() *Pre { return loggedInPre }

// loggedInPre is returned by LoggedIn.
var loggedInPre = &Pre{
	&preImpl{
		name:    "chrome_logged_in",
		timeout: time.Minute,
	},
}

// Pre implements testing.Precondition and provides fast access to an already-started Chrome process.
// While using the precondition, tests cannot call New; they should use the process returned by Chrome instead.
type Pre struct {
	impl *preImpl
}

// Register is called by the test framework.
func (p *Pre) Register(t *testing.Test) { t.RegisterPre(p.impl) }

// Chrome returns the Chrome object that should be used by tests using this precondition.
// The returned object must not be closed by tests.
func (p *Pre) Chrome() *Chrome {
	if p.impl.cr == nil {
		panic("Pre field must be set to chrome.LoggedIn when registering test")
	}
	return p.impl.cr
}

// preImpl implements testing.PreconditionImpl.
type preImpl struct {
	name    string        // testing.PreconditionImpl.String
	timeout time.Duration // testing.PreconditionImpl.Timeout
	cr      *Chrome       // underlying Chrome instance
	opts    []option      // options that should be passed to New
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) {
	defer func() { locked = true }()
	locked = false

	if p.cr != nil {
		if err := p.checkChrome(ctx); err != nil {
			s.Log("Existing Chrome connection is unusable: ", err)
		} else if err = p.resetChromeState(ctx); err != nil {
			s.Log("Failed resetting existing Chrome session: ", err)
		} else {
			s.Log("Reusing existing Chrome session")
			return
		}
		p.closeInternal(ctx, s)
	}

	var err error
	if p.cr, err = New(ctx, p.opts...); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
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
