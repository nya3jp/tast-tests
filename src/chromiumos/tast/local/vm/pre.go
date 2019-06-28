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

// ContainerImageArtifact holds the name of the artifact which will be used to
// boot crostini. When using the CrostiniStartedByArtifact precondition, you
// must list this as one of the data dependencies of your test.
const ContainerImageArtifact string = "crostini_start_basic_guest_images.tar"

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

// CrostiniStartedByDownload is a precondition that ensures a tast test will
// begin after crostini has been started by downloading an image.
func CrostiniStartedByDownload() testing.Precondition { return crostiniStartedByDownloadPre }

// CrostiniStartedByArtifact is similar to CrostiniStartedByDownload, but will
// use a pre-built image as a data-dependency rather than downloading one. To
// use this precondition you must have ContainerImageArtifact as a data
// dependency.
func CrostiniStartedByArtifact() testing.Precondition { return crostiniStartedByArtifactPre }

// The precondition used for crostini when an image download is needed. The 10
// minute timeout is based on what the tests themselves define, and is generous
// to allow for downloading the container image.
var crostiniStartedByDownloadPre = &preImpl{
	name:    "crostini_started_by_download",
	timeout: chrome.LoginTimeout + 10*time.Minute,
	dlImage: true,
}

// The precondition used for crostini when an image will be provided as a data
// dependency.
var crostiniStartedByArtifactPre = &preImpl{
	name:    "crostini_started_by_artifact",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	dlImage: false,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name    string
	timeout time.Duration
	dlImage bool
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
		return p.buildCrostiniPre(ctx, s)
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.cleanUp(ctx, s)
		}
	}()

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

	if p.dlImage {
		s.Log("Setting up component ", StagingComponent)
		if err = SetUpComponent(ctx, StagingComponent); err != nil {
			s.Fatal("Failed to set up component: ", err)
		}
		s.Log("Creating default container (from download)")
		if p.cont, err = CreateDefaultVMContainer(ctx, s.OutDir(), p.cr.User(), StagingImageServer, ""); err != nil {
			s.Fatal("Failed to set up default container (from download): ", err)
		}
	} else {
		s.Log("Setting up component (from artifact)")
		artifactPath := s.DataPath(ContainerImageArtifact)
		if err = MountArtifactComponent(ctx, artifactPath); err != nil {
			s.Fatal("Failed to set up component: ", err)
		}

		s.Log("Creating default container (from artifact)")
		if p.cont, err = CreateDefaultVMContainer(ctx, s.OutDir(), p.cr.User(), Tarball, artifactPath); err != nil {
			s.Fatal("Failed to set up default container (from artifact): ", err)
		}
	}

	locked = true
	chrome.Lock()

	shouldClose = false
	return p.buildCrostiniPre(ctx, s)
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructors.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()
	p.cleanUp(ctx, s)
}

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (p *preImpl) cleanUp(ctx context.Context, s *testing.State) {
	if p.cont != nil {
		if err := p.cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
		if err := StopConcierge(ctx); err != nil {
			s.Error("Failure stopping concierge: ", err)
		}
		p.cont = nil
	}
	// It is always safe to unmount the component, which just posts some
	// logs if it was never mounted.
	UnmountComponent(ctx)

	// Nothing special needs to be done to close the test API connection.
	p.tconn = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Error("Failure closing chrome: ", err)
		}
		p.cr = nil
	}
}

// Helper method that builds the CrostiniPre and resets the machine state in
// advance of running the actual tests.
func (p *preImpl) buildCrostiniPre(ctx context.Context, s *testing.State) CrostiniPre {
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return CrostiniPre{p.cr, p.tconn, p.cont}
}
