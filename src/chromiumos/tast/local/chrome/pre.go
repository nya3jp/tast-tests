// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

var loggedIn *LoggedInPre = &LoggedInPre{}

// LoggedIn returns LoggedInPre.
// TODO(derat): Make the testing package expose whether test registration is complete
// to prevent this function from being called later. Or maybe just prevent it from being called
// while we're running tests? It feels like it may be reasonable for one precondition to use
// another internally, maybe...
func LoggedIn() *LoggedInPre { return loggedIn }

// LoggedInPre is a testing.Precondition implementation that provides tests with a connection
// to a logged-in Chrome process. It should be accessed using the LoggedIn function.
type LoggedInPre struct {
	cr *Chrome
}

func (p *LoggedInPre) String() string         { return "chrome_logged_in" }
func (p *LoggedInPre) Timeout() time.Duration { return time.Minute }

// Chrome returns the Chrome object that should be used by tests using LoggedInPre.
func (p *LoggedInPre) Chrome() *Chrome { return p.cr }

// Prepare is defined to implement testing.Precondition.
// It is called by the test framework at the beginning of every test using this precondition.
func (p *LoggedInPre) Prepare(ctx context.Context, s *testing.State) {
	if p != loggedIn {
		s.Fatal("Must use chrome.LoggedIn")
	}
	if s.RunningTest() {
		s.Fatal("Tests cannot call Prepare")
	}

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
	if p.cr, err = New(ctx); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	p.cr.lock()
}

// Close is defined to implement testing.Precondition.
// It is called by the test framework after the last test that uses this precondition.
func (p *LoggedInPre) Close(ctx context.Context, s *testing.State) {
	if s.RunningTest() {
		s.Fatal("Tests cannot call Close")
	}
	p.closeInternal(ctx, s)
}

// closeInternal closes and resets p.cr if non-nil.
func (p *LoggedInPre) closeInternal(ctx context.Context, s *testing.State) {
	if p.cr == nil {
		return
	}

	p.cr.unlock()
	if err := p.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	p.cr = nil
}

// checkChrome performs basic checks to verify that cr is responsive.
func (p *LoggedInPre) checkChrome(ctx context.Context) error {
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
func (p *LoggedInPre) resetChromeState(ctx context.Context) error {
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
