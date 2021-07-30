// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 30 * time.Second

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome
	// ARC enables interaction with an already-started ARC environment.
	// It cannot be closed by tests.
	ARC *ARC
	// UIDevice is a UI Automator device object.
	// It cannot be closed by tests.
	UIDevice *ui.Device
	// LacrosFixt is lacros fixture data when tests use lacros related fixtures.
	LacrosFixt launcher.FixtData
}

// Booted returns a precondition that ARC Container has already booted when a test is run.
//
// When adding a test, the testing.Test.Pre field may be set to the value returned by this function.
// Later, in the main test function, the value returned by testing.State.PreValue may be converted
// to a PreData containing already-initialized chrome.Chrome and ARC objects:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(arc.PreData)
//		conn, err := d.Chrome.NewConn(ctx, "http://www.example.org/")
//		...
//		cmd := d.ARC.Command(ctx, "dumpsys", "window", "displays")
//		...
//	}
//
// When using this precondition, tests cannot call New or chrome.New.
// The Chrome and ARC instances are also shared and cannot be closed by tests.
func Booted() testing.Precondition { return bootedPre }

// bootedPre is returned by Booted.
var bootedPre = &preImpl{
	name:    "arc_booted",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
}

// NewPrecondition creates a new arc precondition for tests that need different args.
func NewPrecondition(name string, gaia *GaiaVars, extraArgs ...string) testing.Precondition {
	timeout := resetTimeout + chrome.LoginTimeout + BootTimeout
	if gaia != nil {
		timeout = resetTimeout + chrome.GAIALoginTimeout + BootTimeout + optin.OptinTimeout
	}
	pre := &preImpl{
		name:      name,
		timeout:   timeout,
		gaia:      gaia,
		extraArgs: extraArgs,
	}
	return pre
}

// GaiaVars holds the secret variables for username and password for a GAIA login.
type GaiaVars struct {
	UserVar string // the secret variable for the GAIA username
	PassVar string // the secret variable for the GAIA password
}

// preImpl implements testing.Precondition.
type preImpl struct {
	name    string        // testing.Precondition.String
	timeout time.Duration // testing.Precondition.Timeout

	extraArgs []string  // passed to Chrome on initialization
	gaia      *GaiaVars // a struct containing GAIA secret variables

	cr  *chrome.Chrome
	arc *ARC

	init *Snapshot
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.arc != nil {
		pre, err := func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			if err := p.init.Restore(ctx, p.arc); err != nil {
				return nil, errors.Wrap(err, "failed to reset ARC")
			}
			if err := p.cr.ResetState(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to reset Chrome")
			}
			if err := p.arc.saveLogFiles(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to save ARC-related log files")
			}
			if err := p.arc.resetOutDir(ctx, s.OutDir()); err != nil {
				return nil, errors.Wrap(err, "failed to reset outDir field of ARC object")
			}
			return PreData{Chrome: p.cr, ARC: p.arc}, nil
		}()
		if err == nil {
			s.Log("Reusing existing ARC session")
			return pre
		}
		s.Log("Failed to reuse existing ARC session: ", err)
		Unlock()
		chrome.Unlock()
		p.closeInternal(ctx, s)
	}

	// Revert partial initialization.
	shouldClose := true
	defer func() {
		if shouldClose {
			p.closeInternal(ctx, s)
		}
	}()

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		extraArgs := p.extraArgs
		var err error
		if p.gaia != nil {
			username := s.RequiredVar(p.gaia.UserVar)
			password := s.RequiredVar(p.gaia.PassVar)
			p.cr, err = chrome.New(ctx, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.ARCSupported(), chrome.ExtraArgs(extraArgs...))
		} else {
			p.cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(extraArgs...))
		}
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	// Opt-in if performing a GAIA login.
	if p.gaia != nil {
		func() {
			ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout)
			defer cancel()
			tconn, err := p.cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create test API connection: ", err)
			}
			if err := optin.PerformAndClose(ctx, p.cr, tconn); err != nil {
				s.Fatal("Failed to optin to Play Store and Close: ", err)
			}
		}()
	}

	func() {
		ctx, cancel := context.WithTimeout(ctx, BootTimeout)
		defer cancel()
		var err error
		if p.arc, err = New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		if p.init, err = NewSnapshot(ctx, p.arc); err != nil {
			s.Fatal("Failed to get initial ARC state snapshot: ", err)
		}
	}()

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	Lock()
	chrome.Lock()

	shouldClose = false
	return PreData{Chrome: p.cr, ARC: p.arc}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	Unlock()
	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// closeInternal closes and resets p.arc and p.cr if non-nil.
func (p *preImpl) closeInternal(ctx context.Context, s *testing.PreState) {
	if p.arc != nil {
		if err := p.arc.Close(ctx); err != nil {
			s.Log("Failed to close ARC connection: ", err)
		}
		p.arc = nil
	}
	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
	p.init = nil
}
