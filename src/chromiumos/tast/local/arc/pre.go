// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
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

// BootedWithDisableSyncFlags returns a precondition similar to Booted(). The only difference from
// Booted() is that ARC content sync is disabled to avoid noise during power/performance
// measurements.
func BootedWithDisableSyncFlags() testing.Precondition { return bootedWithDisableSyncFlagsPre }

// bootedWithDisableSyncFlagsPre is returned by BootedWithDisableSyncFlags.
var bootedWithDisableSyncFlagsPre = &preImpl{
	name:      "arc_booted_disable_sync",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: DisableSyncFlags(),
}

// BootedInTabletMode returns a precondition similar to Booted(). The only difference from Booted() is
// that Chrome is launched in tablet mode in this precondition.
func BootedInTabletMode() testing.Precondition { return bootedInTabletModePre }

// bootedInTabletModePre is returned by BootedInTabletMode.
var bootedInTabletModePre = &preImpl{
	name:      "arc_booted_in_tablet_mode",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--force-tablet-mode=touch_view", "--enable-virtual-keyboard"},
}

// BootedWithVideoLogging returns a precondition similar to Booted(), but with additional Chrome video logging enabled.
func BootedWithVideoLogging() testing.Precondition { return bootedWithVideoLoggingPre }

// bootedWithVideoLoggingPre is returned by BootedWithVideoLogging.
var bootedWithVideoLoggingPre = &preImpl{
	name:    "arc_booted_with_video_logging",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{
		"--vmodule=" + strings.Join([]string{
			"*/media/gpu/chromeos/*=2",
			"*/media/gpu/vaapi/*=2",
			"*/media/gpu/v4l2/*=2",
			"*/components/arc/video_accelerator/*=2"}, ",")},
}

// NewPrecondition creates a new arc precondition for tests that need different args.
func NewPrecondition(name string, gaia *GaiaVars, extraArgs ...string) testing.Precondition {
	pre := &preImpl{
		name:      name,
		timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
		gaia:      gaia,
		extraArgs: extraArgs,
	}
	if pre.gaia != nil {
		pre.timeout += optin.OptinTimeout
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

	cr    *chrome.Chrome
	state StateManager
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.state.Active() {
		pre, err := func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			// Make sure ARC still works, and reset it.
			if err := p.state.CheckAndReset(ctx, s.OutDir()); err != nil {
				return nil, err
			}
			// Make sure Chrome still works and reset it.
			if err := p.cr.Responded(ctx); err != nil {
				return nil, err
			}
			if err := p.cr.ResetState(ctx); err != nil {
				return nil, errors.Wrap(err, "failed resetting Chrome's state")
			}
			return PreData{p.cr, p.state.arc}, nil
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
			p.cr, err = chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(), chrome.ExtraArgs(extraArgs...))
		} else {
			p.cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(extraArgs...))
		}
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	// Opt-in if performing a GAIA login.
	if p.gaia != nil {
		optin.PostLogin(ctx, p.cr)
	}

	func() {
		ctx, cancel := context.WithTimeout(ctx, BootTimeout)
		defer cancel()
		if err := p.state.Activate(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to create ARC: ", err)
		}
	}()

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	Lock()
	chrome.Lock()

	shouldClose = false
	return PreData{p.cr, p.state.arc}
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
	if p.state.Active() {
		if err := p.state.Deactivate(ctx); err != nil {
			s.Log("Failed to close arc state: ", err)
		}
	}

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
}
