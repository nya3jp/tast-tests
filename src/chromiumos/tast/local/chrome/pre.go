// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"github.com/mafredri/cdp/protocol/target"

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

// NewPrecondition creates a new precondition that can be shared by tests
// that require an already-started Chrome object that was created with opts.
// suffix is appended to precondition's name.
func NewPrecondition(suffix string, opts ...option) testing.Precondition {
	return &preImpl{
		name:    "chrome_" + suffix,
		timeout: time.Minute,
		opts:    opts,
	}
}

var loggedInPre = NewPrecondition("logged_in")

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

	// Try to close all "normal" pages.
	targets, err := p.cr.getDevtoolTargets(ctx, func(t *target.Info) bool {
		return t.Type == "page" || t.Type == "app"
	})
	if err != nil {
		return errors.Wrap(err, "failed to get targets")
	}
	if len(targets) > 0 {
		testing.ContextLogf(ctx, "Closing %d target(s)", len(targets))
		for _, t := range targets {
			args := &target.CloseTargetArgs{TargetID: t.TargetID}
			if reply, err := p.cr.client.Target.CloseTarget(ctx, args); err != nil {
				testing.ContextLogf(ctx, "Failed to close %v: %v", t.URL, err)
			} else if !reply.Success {
				testing.ContextLogf(ctx, "Failed to close %v: unknown failure", t.URL)
			}
		}
	}
	return nil
}
