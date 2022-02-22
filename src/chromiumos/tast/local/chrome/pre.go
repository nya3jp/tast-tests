// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ResetTimeout is the timeout durection to trying reset of the current precondition.
const ResetTimeout = 15 * time.Second

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
func NewPrecondition(suffix string, opts ...Option) testing.Precondition {
	return &preImpl{
		name:    "chrome_" + suffix,
		timeout: ResetTimeout + LoginTimeout,
		opts:    opts,
	}
}

var loggedInPre = NewPrecondition("logged_in")

// preImpl implements testing.Precondition.
type preImpl struct {
	name    string        // testing.Precondition.String
	timeout time.Duration // testing.Precondition.Timeout
	cr      *Chrome       // underlying Chrome instance
	opts    []Option      // Options that should be passed to New
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a *chrome.Chrome that can be used by tests.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil {
		err := func() error {
			ctx, cancel := context.WithTimeout(ctx, ResetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			if err := p.cr.Responded(ctx); err != nil {
				return errors.Wrap(err, "existing Chrome connection is unusable")
			}
			if err := p.cr.ResetState(ctx); err != nil {
				return errors.Wrap(err, "failed resetting existing Chrome session")
			}
			return nil
		}()
		if err == nil {
			s.Log("Reusing existing Chrome session")
			return p.cr
		}
		s.Log("Failed to reuse existing Chrome session: ", err)
		Unlock()
		p.closeInternal(ctx, s)
	}

	ctx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()

	var err error
	if p.cr, err = New(ctx, p.opts...); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	Lock()

	return p.cr
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	Unlock()
	p.closeInternal(ctx, s)
}

// closeInternal closes and resets p.cr if non-nil.
func (p *preImpl) closeInternal(ctx context.Context, s *testing.PreState) {
	if p.cr == nil {
		return
	}
	if err := p.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	p.cr = nil
}
