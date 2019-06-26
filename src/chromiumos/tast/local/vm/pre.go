// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// The CrostiniPre object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(vm.CrostiniPre)
//		...
//	}
type CrostiniPre struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.Conn
	Container   *Container
}

// CrostiniStarted returns a precondition which is used in tests to ensure that
// the crostini instance has been started, and makes several useful object
// available for running your tests.
func CrostiniStarted() testing.Precondition { return crostiniStartedPre }

// The precondition used for crostini. The 10 minute timeout is based
// on what the tests themselves define, and is generous to allow for
// downloading the container image.
var crostiniStartedPre = &preImpl{
	name:    "crostini_started",
	timeout: chrome.LoginTimeout + 10*time.Minute,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name    string
	timeout time.Duration
	cr      *chrome.Chrome
	tconn   *chrome.Conn
	cont    *Container
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cont != nil {
		// TODO(hollingum): sanity checks on the incoming state, see local/arc/pre.go.
		return p.buildCrostiniPre(ctx)
	}

	var err error
	if p.cr, err = chrome.New(ctx); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	s.Log("Enabling Crostini preference setting")
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = EnableCrostini(ctx, p.tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}
	s.Log("Setting up component ", StagingComponent)
	if err = SetUpComponent(ctx, StagingComponent); err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	// TODO(crbug/979074): Use a pre-build image rather than downloading one, to prevent
	// network-based flakiness.
	if p.cont, err = CreateDefaultContainer(ctx, s.OutDir(), p.cr.User(), StagingImageServer, ""); err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}

	locked = true
	chrome.Lock()

	// TODO(hollingum): cleanup code for if the precondition fails. See local/arc/pre.go.

	return p.buildCrostiniPre(ctx)

}

// Called after all tests involving this precondition have been run. Cleans up
// the precondition data.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()

	if err := p.cont.DumpLog(ctx, s.OutDir()); err != nil {
		s.Error("Failure dumping container log: ", err)
	}
	StopConcierge(ctx)
	UnmountComponent(ctx)
	p.cr.Close(ctx)
}

// Helper method that builds the CrostiniPre and resets the machine state in
// advance of running the actual tests.
func (p *preImpl) buildCrostiniPre(ctx context.Context) CrostiniPre {
	p.cr.ResetState(ctx)
	return CrostiniPre{p.cr, p.tconn, p.cont}
}
