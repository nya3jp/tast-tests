// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
	"chromiumos/tast/local/testexec"
)

// ImageArtifact holds the name of the artifact which will be used to
// boot crostini. When using the StartedByArtifact precondition, you
// must list this as one of the data dependencies of your test.
const ImageArtifact string = "lacros_binary.tar"

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(crostini.PreData)
//		...
//	}
type PreData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.Conn
	Container   *vm.Container
	Keyboard    *input.KeyboardEventWriter
}

// StartedByArtifact is similar to StartedByDownload, but will
// use a pre-built image as a data-dependency rather than downloading one. To
// use this precondition you must have crostini.ImageArtifact as a data dependency.
func StartedByArtifact() testing.Precondition { return startedByArtifactPre }

type setupMode int

const (
	artifact setupMode = iota
	download
)

var startedByArtifactPre = &preImpl{
	name:    "lacros_started_by_artifact",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    artifact,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name         string               // Name of this precondition (for logging/uniqueing purposes).
	timeout      time.Duration        // Timeout for completing the precondition.
	mode         setupMode            // Where (download/build artifact) the container image comes from.
	arch         vm.ContainerArchType // Architecture/distribution of the container image.
	arcEnabled   bool                 // Flag for whether Arc++ should be available (as well as crostini).
	gpuEnabled   bool                 // Flag for whether the crostini VM should be booted with GPU support.
	useInstaller bool                 // Flag for whether to run the Crostini installer in chrome (useful for setting up e.g. CrostiniManager).
	cr           *chrome.Chrome
	tconn        *chrome.Conn
	cont         *vm.Container
	keyboard     *input.KeyboardEventWriter
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
		// if err := SimpleCommandWorks(ctx, p.cont); err != nil {
		// 	s.Log("Precondition unsatisifed: ", err)
		// 	p.cont = nil
		// 	p.Close(ctx, s)
		// } else {
		// 	return p.buildPreData(ctx, s)
		// }
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.cleanUp(ctx, s)
		}
	}()

	opt := chrome.ARCDisabled()

	var err error
	if p.cr, err = chrome.New(ctx, opt); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	switch p.mode {
	case artifact:
		s.Log("asdf1-------------------------")
		artifactPath := s.DataPath(ImageArtifact)
		mountCmd := testexec.CommandContext(ctx, "mount", strings.Fields("-o remount,exec /mnt/stateful_partition")...)
		if err := mountCmd.Run(); err != nil {
			s.Log("bbbbbbb", err)
			mountCmd.DumpLog(ctx)
			return errors.Wrap(err, "failed to mount component")
		}
		mountCmd = testexec.CommandContext(ctx, "rm", strings.Fields("-rf /mnt/stateful_partition/test1234")...)
		if err := mountCmd.Run(); err != nil {
			mountCmd.DumpLog(ctx)
			return errors.Wrap(err, "failed to mount component")
		}
		mountCmd = testexec.CommandContext(ctx, "mkdir", strings.Fields("-p /mnt/stateful_partition/test1234")...)
		if err := mountCmd.Run(); err != nil {
			mountCmd.DumpLog(ctx)
			return errors.Wrap(err, "failed to mount component")
		}

		mountCmd = testexec.CommandContext(ctx, "tar", strings.Fields("-xvf " + artifactPath + " -C /mnt/stateful_partition/test1234")...)
		if err := mountCmd.Run(); err != nil {
			mountCmd.DumpLog(ctx)
			return errors.Wrap(err, "failed to mount component")
		}

		binary_loc := "/mnt/stateful_partition/test1234/lacros_binary"
		mountCmd = testexec.CommandContext(ctx, binary_loc + "/chrome", strings.Fields("--ozone-platform=wayland --no-sandbox --user-data-dir=" + binary_loc + "/user_data --lang=en-US --breakpad-dump-location=" + binary_loc)...)
		    mountCmd.Cmd.Env = os.Environ()
		    mountCmd.Cmd.Env = append(mountCmd.Cmd.Env, "XDG_RUNTIME_DIR=/run/chrome")
		    mountCmd.Cmd.Env = append(mountCmd.Cmd.Env, "LD_LIBRARY_PATH=" + binary_loc)
		if err := mountCmd.Run(); err != nil {
			mountCmd.DumpLog(ctx)
			return errors.Wrap(err, "failed to mount component")
		}
	default:
		s.Fatal("Unrecognized mode: ", p.mode)
	}

	// chrome.Lock()
	// vm.Lock()

	ret := p.buildPreData(ctx, s)
	shouldClose = false
	return ret
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructors.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	// chrome.Unlock()
	p.cleanUp(ctx, s)
}

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (p *preImpl) cleanUp(ctx context.Context, s *testing.State) {
	if p.keyboard != nil {
		if err := p.keyboard.Close(); err != nil {
			s.Error("Failure closing keyboard: ", err)
		}
		p.keyboard = nil
	}

	if p.cont != nil {
		if err := p.cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
		if err := vm.StopConcierge(ctx); err != nil {
			s.Error("Failure stopping concierge: ", err)
		}
		p.cont = nil
	}
	// It is always safe to unmount the component, which just posts some
	// logs if it was never mounted.
	// vm.UnmountComponent(ctx)

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
	return PreData{p.cr, p.tconn, p.cont, p.keyboard}
}
