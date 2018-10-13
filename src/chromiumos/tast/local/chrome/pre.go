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
		p.cr.release()
		p.cr.Close(ctx)
		p.cr = nil
	}

	var err error
	if p.cr, err = New(ctx, shared()); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
}

// Close is defined to implement testing.Precondition.
// It is called by the test framework after the last test that uses this precondition.
func (p *LoggedInPre) Close(ctx context.Context, s *testing.State) {
	if s.RunningTest() {
		s.Fatal("Tests cannot call Close")
	}

	if p.cr == nil {
		return
	}

	p.cr.release()
	if err := p.cr.Close(ctx); err != nil {
		// TODO(derat): Should we report an error? Tests typically call Close in a defer statement.
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

func (p *LoggedInPre) resetChromeState(ctx context.Context) error {
	testing.ContextLog(ctx, "Resetting Chrome's state")
	conn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	// TODO: Simplify this JavaScript if possible.
	if err = conn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
  chrome.windows.getAll({}, (windows) => {
    var promises = [];
    windows.forEach((window) => {
      promises.push(new Promise((resolve, reject) => {
        chrome.windows.remove(window.id, () => { resolve(); });
      }));
    });
    Promise.all(promises).then(() => { resolve(); });
  });
})`, nil); err != nil {
		return errors.Wrap(err, "closing windows failed")
	}
	return nil
}
