// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chromiumos/tast/testing"
)

var loggedIn *LoggedInPre

func init() {
	loggedIn = &LoggedInPre{}
}

// LoggedIn returns LoggedInPre.
func LoggedIn() *LoggedInPre { return loggedIn }

// LoggedInPre is a testing.Precondition implementation that provides tests with a connection
// to a logged-in Chrome process. It should be referenced using the LoggedIn function.
type LoggedInPre struct {
	cr *Chrome
}

// Chrome returns the Chrome object that should be used by tests using LoggedInPre.
func (p *LoggedInPre) Chrome() *Chrome { return p.cr }

// Prepare is defined to implement testing.Precondition.
// It is called by the test framework at the beginning of every test using this precondition.
func (p *LoggedInPre) Prepare(ctx context.Context) error {
	if p.cr != nil {
		err := p.checkChrome(ctx)
		if err == nil {
			return p.resetChromeState(ctx)
		}
		testing.ContextLog(ctx, "Existing Chrome connection is unusable: ", err)
		p.cr.Close(ctx)
	}

	var err error
	if p.cr, err = New(ctx); err != nil {
		return fmt.Errorf("failed to start Chrome: %v", err)
	}
	return nil
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
		return fmt.Errorf("closing windows failed: %v", err)
	}
	return nil
}

func (p *LoggedInPre) String() string { return "chrome_logged_in" }
