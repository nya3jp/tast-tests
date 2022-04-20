// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ambient

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 30 * time.Second

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
}

// NewPrecondition creates a new arc precondition for tests that need different args.
func NewPrecondition(name string, gaia *GaiaVars) testing.Precondition {
	timeout := resetTimeout + chrome.LoginTimeout
	if gaia != nil {
		timeout = resetTimeout + chrome.GAIALoginTimeout
	}
	pre := &preImpl{
		name:    name,
		timeout: timeout,
		gaia:    gaia,
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

	gaia  *GaiaVars // a struct containing GAIA secret variables
	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil {
		err := func() error {
			if err := p.cr.Responded(ctx); err != nil {
				return errors.Wrap(err, "existing Chrome connection is unusable")
			}

			if err := p.cr.ResetState(ctx); err != nil {
				return errors.Wrap(err, "failed to reset existing Chrome session")
			}

			return nil
		}()
		if err == nil {
			s.Log("reuse existing Chrome session")
			return PreData{p.cr, p.tconn}
		}
		chrome.Unlock()
		p.closeInternal(ctx, s)
	}

	ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
	defer cancel()
	var err error

	if p.gaia != nil {
		// Login into the device, using GAIA login.
		username := s.RequiredVar(p.gaia.UserVar)
		password := s.RequiredVar(p.gaia.PassVar)
		p.cr, err = chrome.New(ctx, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.EnableFeatures("PersonalizationHub"))
	} else {
		p.cr, err = chrome.New(ctx, chrome.EnableFeatures("PersonalizationHub"))
	}
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	p.tconn, err = p.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, p.tconn)

	chrome.Lock()
	return PreData{p.cr, p.tconn}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()
	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// closeInternal closes and resets p.cr if non-nil.
func (p *preImpl) closeInternal(ctx context.Context, s *testing.PreState) {
	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
		p.tconn = nil
	}
}
