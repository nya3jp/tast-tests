// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"os"
	"path/filepath"
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

	TestAPIConn *chrome.TestConn
}

// NewPrecondition creates a new drivefs precondition for tests that need different logins.
func NewPrecondition(name string, gaia *GaiaVars, extraArgs ...string) testing.Precondition {
	pre := &preImpl{
		name:      name,
		timeout:   chrome.LoginTimeout,
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

	mountPath string
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

	// If mountPath exists, check if Drive has stabilized return early if it has.
	if p.mountPath != "" {
		_, err := WaitForDriveFs(ctx, p.cr.User())
		if err != nil {
			s.Log("Failed as precondition is unsatisfied: ", err)
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

	// Start Chrome with our supplied GAIA credentials.
	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		extraArgs := p.extraArgs
		var err error
		if p.gaia != nil {
			username := s.RequiredVar(p.gaia.UserVar)
			password := s.RequiredVar(p.gaia.PassVar)
			p.cr, err = chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ExtraArgs(extraArgs...))
		} else {
			s.Fatal("Failed to start Chrome as no GAIA credentials supplied")
		}
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	// Wait for Drivefs to start.
	mountPath, err := WaitForDriveFs(ctx, p.cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	p.mountPath = mountPath
	s.Log("drivefs fully started")

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

func (p *preImpl) cleanUp(ctx context.Context, s *testing.PreState) {
	p.tconn = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed closing chrome: ", err)
		}
		p.cr = nil
	}

	if p.mountPath != "" {
		if err := p.removeDriveContents(ctx); err != nil {
			s.Logf("Failed removing all drive mount path %q contents: %v", p.mountPath, err)
		}
		p.mountPath = ""
	}
}

// removeDriveContents clears all the files used during test execution from file system.
func (p *preImpl) removeDriveContents(ctx context.Context) error {
	rootMountPath := filepath.Join(p.mountPath, "root")
	directory, err := os.Open(rootMountPath)
	if err != nil {
		return err
	}
	defer directory.Close()

	names, err := directory.Readdirnames(-1)
	if err != nil {
		return err
	}

	if len(names) > 0 {
		testing.ContextLogf(ctx, "Attempting to remove %d files from Drive", len(names))
	}

	for _, name := range names {
		pathToRemove := filepath.Join(rootMountPath, name)
		if err := os.RemoveAll(pathToRemove); err != nil {
			testing.ContextLogf(ctx, " [FAILED] %s: %v", pathToRemove, err)
		} else {
			testing.ContextLog(ctx, " [SUCCESS] ", pathToRemove)
		}
	}

	return nil
}
