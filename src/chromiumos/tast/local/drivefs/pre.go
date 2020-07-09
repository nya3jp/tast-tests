// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome

	// The path that DriveFS has mounted at.
	MountPath string

	// The API connection to the Test extension, reused by tests
	TestAPIConn *chrome.TestConn
}

// NewPrecondition creates a new drivefs precondition for tests that need different logins.
func NewPrecondition(name string, gaia *GaiaVars) testing.Precondition {
	pre := &preImpl{
		name:    name,
		timeout: chrome.LoginTimeout,
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

	gaia *GaiaVars // a struct containing GAIA secret variables

	mountPath string // The path where Drivefs is mounted
	cr        *chrome.Chrome
	tconn     *chrome.TestConn
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	// If mountPath exists, check if Drive has stabilized and return early if it has.
	if p.mountPath != "" {
		_, err := WaitForDriveFs(ctx, p.cr.User())
		if err != nil {
			s.Log("Failed as precondition is unsatisfied: ", err)
			p.cleanUp(ctx, s)
		} else {
			return p.buildPreData(ctx, s)
		}
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.cleanUp(ctx, s)
		}
	}()

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		var err error
		if p.gaia != nil {
			username := s.RequiredVar(p.gaia.UserVar)
			password := s.RequiredVar(p.gaia.PassVar)
			p.cr, err = chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCDisabled())
		} else {
			s.Fatal("Failed to start Chrome with a GAIA login")
		}
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	mountPath, err := WaitForDriveFs(ctx, p.cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("drivefs fully started")
	p.mountPath = mountPath

	tconn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}
	p.tconn = tconn

	// Lock Chrome and make sure deferred function does not run cleanup.
	chrome.Lock()
	shouldClose = false

	return p.buildPreData(ctx, s)
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	chrome.Unlock()
	p.cleanUp(ctx, s)
}

// buildPreData is a helper function that resets Chrome state and returns required data.
func (p *preImpl) buildPreData(ctx context.Context, s *testing.PreState) PreData {
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return PreData{p.cr, p.mountPath, p.tconn}
}

// cleanUp closes Chrome, resets the mountPath to empty string and sets tconn to nil
func (p *preImpl) cleanUp(ctx context.Context, s *testing.PreState) {
	p.tconn = nil
	p.mountPath = ""

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed closing chrome: ", err)
		}
		p.cr = nil
	}
}
