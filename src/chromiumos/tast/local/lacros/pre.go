// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ImageArtifact holds the name of the artifact which will be used to
// launch linux-chrome. When using the StartedByArtifact precondition, you
// must list this as one of the data dependencies of your test.
const ImageArtifact string = "lacros_binary.tar"

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(lacros.PreData)
//		...
//	}
type PreData struct {
	Chrome      *chrome.Chrome   // The CrOS-chrome instance.
	TestAPIConn *chrome.Conn     // The CrOS-chrome connection.
}

// StartedByArtifact uses a pre-built image as a data-dependency. To use this
// precondition you must have crostini.ImageArtifact as a data dependency.
func StartedByArtifact() testing.Precondition { return startedByArtifactPre }

type setupMode int

const (
	artifact setupMode = iota
)

var LacrosTestPath = "/mnt/stateful_partition/lacros_test_artifacts"

var startedByArtifactPre = &preImpl{
	name:    "lacros_started_by_artifact",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    artifact,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name    string           // Name of this precondition (for logging/uniqueing purposes).
	timeout time.Duration    // Timeout for completing the precondition.
	mode         setupMode            // Where (download/build artifact) the container image comes from.
	cr      *chrome.Chrome   // Connection to CrOS-chrome.
	tconn   *chrome.Conn     // Test-connection for CrOS-chrome.
	cmd     *testexec.Cmd    // The command context used to start linux-chrome.
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Ensures that linux-chrome binary is available on disk and ready to launch.
// Does not launch the binary.
func (p *preImpl) prepareLinuxChromeBinary(ctx context.Context, s *testing.State) interface{} {
	// We reuse the custom extension from the chrome package for exposing private interfaces.
	c := &chrome.Chrome{}
	if err := c.PrepareExtensions(ctx); err != nil {
		return err
	}

	artifactPath := s.DataPath(ImageArtifact)
	mountCmd := testexec.CommandContext(ctx, "mount", strings.Fields("-o remount,exec /mnt/stateful_partition")...)
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to remount stateful partition with exec privilege")
	}

	mountCmd = testexec.CommandContext(ctx, "rm", "-rf", LacrosTestPath)
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}
	mountCmd = testexec.CommandContext(ctx, "mkdir", "-p", LacrosTestPath)
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	mountCmd = testexec.CommandContext(ctx, "tar", "-xvf", artifactPath, "-C", LacrosTestPath)
	if err := mountCmd.Run(); err != nil {
		mountCmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to untar test artifacts")
	}

	return nil
}

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

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
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	switch p.mode {
	case artifact:
		p.prepareLinuxChromeBinary(ctx, s)
	default:
		s.Fatal("Unrecognized mode: ", p.mode)
	}

	ret := p.buildPreData(ctx, s)
	chrome.Lock()
	shouldClose = false
	return ret
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructors.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	chrome.Unlock()
	p.cleanUp(ctx, s)
}

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (p *preImpl) cleanUp(ctx context.Context, s *testing.State) {
	// Nothing special needs to be done to close the test API connection.
	p.tconn = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Error("Failure closing chrome: ", err)
		}
		p.cr = nil
	}

}

// buildPreData is a helper method that resets the machine state in
// advance of building the precondition data for the actual tests.
func (p *preImpl) buildPreData(ctx context.Context, s *testing.State) PreData {
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return PreData{p.cr, p.tconn}
}
