// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"chromiumos/tast/errors"
	//"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// LoginReuse returns the loginReuse pre.
func LoginReuse() testing.Precondition { return loginReusePre }

var loginReusePreName = "login_reuse"
var loginReusePre = newPrecondition(loginReusePreName, &preLoginReuse{})

// preLoginReuse implements the preInternal interface.
type preLoginReuse struct{}

func (pl *preLoginReuse) prepare(ctx context.Context, s *testing.State, p *preImpl) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil {
		err := PreReset(ctx, p.cr)
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
	Unlock()
	if p.cr, err = newChrome(ctx, s, true); err != nil {
		//s.Log("Failed to start Chrome. Restarting UI before failing...")
		//upstart.RestartJob(ctx, "ui")
		s.Fatal("Failed to start Chrome: ", err, " MTBF- Reboot DUT Required.")
	}
	Lock()
	return p.cr
}

// newChrome gets the Chrome instance.
// try to reuse the exsting running chrome if "reuse" is true (i.e. no ui restart).
func newChrome(ctx context.Context, s *testing.State, reuseUI bool) (*Chrome, error) {
	var opts []Option // Options that should be passed to New.
	opts = append(opts, KeepState(), ARCSupported(), GAIALogin())
	if reuseUI {
		opts = append(opts, ReuseLogin())
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
		opts = append(opts, Auth(userID, userPasswd, ""))

	}
	return New(ctx, opts...)
}

func (pl *preLoginReuse) close(ctx context.Context, s *testing.State, p *preImpl) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	Unlock()
	p.closeInternal(ctx, s)
}

// PreReset checks the existing chrome and resets state.
func PreReset(ctx context.Context, cr *Chrome) error {
	p := &preImpl{
		cr:      cr,
		name:    namePrefix + loginReusePreName,
		timeout: resetTimeout + LoginTimeout,
	}
	ctx, cancel := context.WithTimeout(ctx, resetTimeout)
	defer cancel()
	ctx, st := timing.Start(ctx, "reset_"+p.name)
	defer st.End()
	if err := p.checkChrome(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := p.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

// ForceRelogin returns the forceRelogin pre.
func ForceRelogin() testing.Precondition { return forceReloginPre }

var forceReloginPreName = "force_relogin"
var forceReloginPre = newPrecondition(forceReloginPreName, &preForceRelogin{})

// preLoginReuse implements the preInternal interface.
type preForceRelogin struct{}

func (pl *preForceRelogin) prepare(ctx context.Context, s *testing.State, p *preImpl) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	Unlock()
	if p.cr != nil {
		p.closeInternal(ctx, s)
	}

	ctx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()

	var err error
	if p.cr, err = newChrome(ctx, s, false); err != nil {
		s.Fatal("Failed to start Chrome: ", err, " MTBF- Reboot DUT Required.")
	}
	Lock()
	return p.cr
}

func (pl *preForceRelogin) close(ctx context.Context, s *testing.State, p *preImpl) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	Unlock()
	p.closeInternal(ctx, s)
}
