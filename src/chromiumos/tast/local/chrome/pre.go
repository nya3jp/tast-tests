// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/devtool"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// LoggedIn returns a precondition that Chrome is already logged in when a test is run.
//
// When adding a test, the testing.Test.Pre field may be set to the value returned by this function.
// Later, in the main test function, the value returned by testing.State.PreValue may be converted
// to an already-logged-in *chrome.Chrome:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		cr := s.PreValue().(*chrome.Chrome)
//		conn, err := cr.NewConn(ctx, "http://www.example.org/")
//		...
//	}
//
// When using this precondition, tests cannot call New.
// The Chrome instance is also shared and cannot be closed by tests.
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
// It returns a *chrome.Chrome that can be used by tests.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	defer timing.Start(ctx, "prepare_"+p.name).End()
	defer func() { locked = true }()
	locked = false

	if p.cr != nil {
		if err := p.checkChrome(ctx); err != nil {
			s.Log("Existing Chrome connection is unusable: ", err)
		} else if err = p.resetChromeState(ctx); err != nil {
			s.Log("Failed resetting existing Chrome session: ", err)
		} else {
			s.Log("Reusing existing Chrome session")
			return p.cr
		}
		p.closeInternal(ctx, s)
	}

	var err error
	if p.cr, err = New(ctx, p.opts...); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	return p.cr
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	defer timing.Start(ctx, "close_"+p.name).End()
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
	defer timing.Start(ctx, "check_chrome").End()
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
	defer timing.Start(ctx, "reset_chrome").End()

	// Try to close all "normal" renderers.
	targets, err := p.cr.getDevtoolTargets(ctx, func(t *devtool.Target) bool {
		return t.URL == "chrome://newtab/" || strings.HasPrefix(t.URL, "http://") ||
			strings.HasPrefix(t.URL, "https://")
	})
	if err != nil {
		return errors.Wrap(err, "failed to get targets")
	}
	for _, t := range targets {
		if conn, err := newConn(ctx, t, p.cr.logMaster, p.cr.chromeErr); err != nil {
			testing.ContextLogf(ctx, "Failed connecting to %v target: %v", t.URL, err)
		} else {
			if err := conn.CloseTarget(ctx); err != nil {
				testing.ContextLogf(ctx, "Failed to close %v target: %v", t.URL, err)
			}
			conn.Close()
		}
	}
	return nil
}
