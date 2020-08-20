// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"

	//"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// loginReusePreName is the precondition name for LoginReuse
const loginReusePreName = "arc_login_reuse"

// NewForLoginReuse restarts the ui job, tells Chrome to enable testing, and (by default) logs in.
func NewForLoginReuse(ctx context.Context, s *testing.State) (*chrome.Chrome, *ARC, error) {
	pre := s.Pre().(*preImpl)
	if pre.String() != loginReusePreName {
		panic(fmt.Sprintf("Do not call NewForLoginReuse while precondition %s is being used. Expecting: %s",
			pre.String(), loginReusePreName))
	}

	locked = false
	chrome.Unlock()

	var err error
	p := &preImpl{
		name:    loginReusePreName,
		timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
		pi:      &preLoginReuse{},
	}

	defer func() {
		// Reset the cr and arc stored in precondition and state
		pre.arc = p.arc
		pre.cr = p.cr
		pre.origInitPID = p.origInitPID
		pre.origInstalledPkgs = p.origInstalledPkgs
		pre.origRunningPkgs = p.origRunningPkgs
		s.SetPreValue(PreData{p.cr, p.arc})
	}()

	if p.cr, err = newChrome(ctx, s, false); err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome: ", err, " MTBF Error - Reboot DUT Required.")
		return p.cr, p.arc, err
	}

	if p.arc, err = New(ctx, s.OutDir()); err != nil {
		testing.ContextLog(ctx, "Failed to start ARC: ", err)
		return p.cr, p.arc, err
	}
	if p.origInitPID, err = InitPID(); err != nil {
		testing.ContextLog(ctx, "Failed to get initial init PID: ", err)
		return p.cr, p.arc, err
	}
	if p.origInstalledPkgs, err = p.installedPackages(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to list initial packages: ", err)
		return p.cr, p.arc, err
	}
	if p.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to list running packages: ", err)
		return p.cr, p.arc, err
	}
	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	locked = true
	chrome.Lock()
	return p.cr, p.arc, err
}

// LoginReuse reuses or login to the system with Chrome and ARC ready.
// The Chrome and ARC instances are also shared and cannot be closed by tests.
func LoginReuse() testing.Precondition { return loginReusePre }

// loginReusePre is returned by LoginReuse.
var loginReusePre = &preImpl{
	name:    loginReusePreName,
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
	pi:      &preLoginReuse{},
}

// preLoginReuse implements the preInternal interface for LoginReuse precondition
type preLoginReuse struct {
}

// prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (pl *preLoginReuse) prepare(ctx context.Context, s *testing.State, p *preImpl) interface{} {
	crOk := false
	if p.cr != nil {
		err := chrome.PreReset(ctx, p.cr)
		if err == nil {
			s.Log("Reusing existing Chrome session")
			crOk = true
		} else {
			s.Log("Failed to reuse existing Chrome session: ", err)
			locked = false
			chrome.Unlock()
			p.closeInternal(ctx, s)
		}
	}

	if crOk && p.arc != nil {
		pre, err := func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			installed, err := p.installedPackages(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get installed packages")
			}
			running, err := p.runningPackages(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get running packages")
			}
			if err := p.checkUsable(ctx, installed, running); err != nil {
				return nil, errors.Wrap(err, "existing Chrome or ARC connection is unusable")
			}
			if err := p.resetState(ctx, installed, running); err != nil {
				return nil, errors.Wrap(err, "failed resetting existing Chrome or ARC session")
			}
			if err := p.arc.setLogcatFile(filepath.Join(s.OutDir(), logcatName)); err != nil {
				return nil, errors.Wrap(err, "failed to update logcat output file")
			}
			return PreData{p.cr, p.arc}, nil
		}()
		if err == nil {
			s.Log("Reusing existing ARC session")
			return pre
		}
		s.Log("Failed to reuse existing ARC session: ", err)
		locked = false
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

	var err error
	if p.cr, err = newChrome(ctx, s, true); err != nil {
		//s.Log("Failed to start Chrome. Restarting UI before failing...")
		//upstart.RestartJob(ctx, "ui")
		s.Fatal("Failed to start Chrome: ", err, " MTBF Error - Reboot DUT Required.")
	}

	func() {
		ctx, cancel := context.WithTimeout(ctx, BootTimeout)
		defer cancel()
		var err error
		if p.arc, err = New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		if p.origInitPID, err = InitPID(); err != nil {
			s.Fatal("Failed to get initial init PID: ", err)
		}
		if p.origInstalledPkgs, err = p.installedPackages(ctx); err != nil {
			s.Fatal("Failed to list initial packages: ", err)
		}
		if p.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
			s.Fatal("Failed to list running packages: ", err)
		}
	}()

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	locked = true
	chrome.Lock()

	shouldClose = false
	return PreData{p.cr, p.arc}
}

func newChrome(ctx context.Context, s *testing.State, skipRestart bool) (*chrome.Chrome, error) {
	ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
	defer cancel()

	var opts []chrome.Option // Options that should be passed to New
	opts = append(opts, chrome.KeepState(), chrome.GAIALogin(), chrome.ARCSupported())
	if skipRestart {
		opts = append(opts, chrome.ReuseLogin())
	}

	var userID, userPasswd string
	var ok bool
	if userID, ok = s.CheckVar("userID"); !ok {
		s.Fatal("create new Chrome - userID not provided. Please specify it in your test vars configuration")
	}
	if userPasswd, ok = s.CheckVar("userPasswd"); !ok {
		s.Fatal("create new Chrome - userPasswd not provided. Please specify it in your test vars configuration")
	}
	if ok {
		opts = append(opts, chrome.Auth(userID, userPasswd, ""))

	}
	return chrome.New(ctx, opts...)
}

func (pl *preLoginReuse) close(ctx context.Context, s *testing.State, p *preImpl) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()
	p.closeInternal(ctx, s)
}
