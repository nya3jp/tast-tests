// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// ImageArtifact holds the name of the artifact which will be used to
// boot crostini. When using the StartedByArtifact precondition, you
// must list this as one of the data dependencies of your test.
const ImageArtifact string = "crostini_guest_images.tar"

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
}

// StartedByDownload is a precondition that ensures a tast test will
// begin after crostini has been started by downloading an image.
func StartedByDownload() testing.Precondition { return startedByDownloadPre }

// StartedByArtifact is similar to StartedByDownload, but will
// use a pre-built image as a data-dependency rather than downloading one. To
// use this precondition you must have crostini.ImageArtifact as a data dependency.
func StartedByArtifact() testing.Precondition { return startedByArtifactPre }

// StartedGPUEnabled is similar to StartedByArtifact, but will
// use pass enable-gpu to vm instance to allow gpu being used.
func StartedGPUEnabled() testing.Precondition { return startedGPUEnabledPre }

// StartedByInstaller works like StartedByArtifact (including the need to add
// its data dependency) but additionally runs the installer in order to update
// CrostiniManager within chrome.
//
// TODO(crbug.com/994040): This is a temporary precondition. Once we have
// verified that it is stable, remove it and add its logic to all the others.
func StartedByInstaller() testing.Precondition { return startedByInstallerPre }

type setupMode int

const (
	artifact setupMode = iota
	download
	gpu
	installer
)

var startedByArtifactPre = &preImpl{
	name:    "crostini_started_by_artifact",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    artifact,
}

var startedByDownloadPre = &preImpl{
	name:    "crostini_started_by_download",
	timeout: chrome.LoginTimeout + 10*time.Minute,
	mode:    download,
}

var startedGPUEnabledPre = &preImpl{
	name:    "crostini_started_gpu_enabled",
	timeout: chrome.LoginTimeout + 10*time.Minute,
	mode:    gpu,
}

var startedByInstallerPre = &preImpl{
	name:    "crostini_started_by_installer",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    installer,
}

// Implementation of crostini's precondition.
type preImpl struct {
	name    string
	timeout time.Duration
	cr      *chrome.Chrome
	tconn   *chrome.Conn
	cont    *vm.Container
	mode    setupMode
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
		return p.buildPreData(ctx, s)
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
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if p.mode == installer {
		s.Logf("Notifying chrome of a pre-existing component %q at %q", vm.TerminaComponentName, vm.TerminaMountDir)
		if err := p.tconn.Eval(ctx, fmt.Sprintf(
			`chrome.autotestPrivate.registerComponent("%s", "%s")`,
			vm.TerminaComponentName, vm.TerminaMountDir), nil); err != nil {
			s.Fatal("Failed to run autotestPrivate.registerComponent: ", err)
		}

	} else {
		s.Log("Enabling Crostini preference setting")
		if err = vm.EnableCrostini(ctx, p.tconn); err != nil {
			s.Fatal("Failed to enable Crostini preference setting: ", err)
		}
	}

	switch p.mode {
	case download:
		s.Log("Setting up component ", vm.StagingComponent)
		if err = vm.SetUpComponent(ctx, vm.StagingComponent); err != nil {
			s.Fatal("Failed to set up component: ", err)
		}
		s.Log("Creating default container (from download)")
		if p.cont, err = vm.CreateDefaultVMContainer(ctx, s.OutDir(), p.cr.User(), vm.StagingImageServer, "", false); err != nil {
			s.Fatal("Failed to set up default container (from download): ", err)
		}
	case artifact, gpu, installer:
		s.Log("Setting up component (from artifact)")
		artifactPath := s.DataPath(ImageArtifact)
		if err = vm.MountArtifactComponent(ctx, artifactPath); err != nil {
			s.Fatal("Failed to set up component: ", err)
		}
		s.Log("Creating default container (from artifact)")
		if p.cont, err = vm.CreateDefaultVMContainer(ctx, s.OutDir(), p.cr.User(), vm.Tarball, artifactPath, p.mode == gpu); err != nil {
			s.Fatal("Failed to set up default container (from artifact): ", err)
		}
		if p.mode == installer {
			s.Log("Installing crostini")
			if err := p.tconn.EvalPromise(ctx,
				`new Promise((resolve, reject) => {
					chrome.autotestPrivate.runCrostiniInstaller(() => {
						if (chrome.runtime.lastError === undefined) {
							resolve();
						} else {
							reject(new Error(chrome.runtime.lastError.message));
						}
					});
				})`, nil); err != nil {
				s.Fatal("Running autotestPrivate.runCrostiniInstaller failed: ", err)
			}

		}
	default:
		s.Fatal("Unrecognized mode: ", p.mode)
	}

	// Stop the apt-daily systemd timers since they may end up running while we
	// are executing the tests and cause failures due to resource contention.
	for _, t := range []string{"apt-daily", "apt-daily-upgrade"} {
		s.Log("Disabling service: ", t)
		cmd := p.cont.Command(ctx, "sudo", "systemctl", "stop", t+".timer")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to stop %s timer: %v", t, err)
		}
	}

	chrome.Lock()
	vm.Lock()

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

	vm.Unlock()
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
		if err := vm.StopConcierge(ctx); err != nil {
			s.Error("Failure stopping concierge: ", err)
		}
		p.cont = nil
	}
	// It is always safe to unmount the component, which just posts some
	// logs if it was never mounted.
	vm.UnmountComponent(ctx)

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
	return PreData{p.cr, p.tconn, p.cont}
}
