// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddFixt(&testing.Fixt{
		Name: "crostini_started_by_artifact",
		Impl: &fixtImpl{
			mode:     artifact,
			diskSize: vm.DefaultDiskSize,
		},
		Timeout: chrome.LoginTimeout + 7*time.Minute,
	})
}

// The FixtData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(crostini.PreData)
//		...
//	}
type FixtData struct {
	Chrome      *chrome.Chrome
	TestAPIConn *chrome.TestConn
	Container   *vm.Container
	Keyboard    *input.KeyboardEventWriter
}

// Implementation of crostini's precondition.
type fixtImpl struct {
	name         string               // Name of this precondition (for logging/uniqueing purposes).
	timeout      time.Duration        // Timeout for completing the precondition.
	mode         setupMode            // Where (download/build artifact) the container image comes from.
	arch         vm.ContainerArchType // Architecture/distribution of the container image.
	arcEnabled   bool                 // Flag for whether Arc++ should be available (as well as crostini).
	gpuEnabled   bool                 // Flag for whether the crostini VM should be booted with GPU support.
	useInstaller bool                 // Flag for whether to run the Crostini installer in chrome (useful for setting up e.g. CrostiniManager).
	diskSize     uint64               // The targeted size of the VM image in bytes.
	tconn        *chrome.TestConn
	cont         *vm.Container
	keyboard     *input.KeyboardEventWriter
}

func (p *fixtImpl) Adjust(ctx context.Context, s *testing.FixtTestState) error {
	if err := SimpleCommandWorks(ctx, p.cont); err != nil {
		return fmt.Errorf("Precondition unsatisifed: ", err)
	}
	return nil
}

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *fixtImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	cr := s.FixtValue().(*chrome.Chrome)

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.cleanUp(ctx, s)
		}
	}()

	// To help identify sources of flake, we report disk usage before the test.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	var err error
	if p.useInstaller {
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
		s.Logf("Creating %q container (from download)", vm.ArchitectureAlias(p.arch))
		if p.cont, err = vm.CreateDefaultVMContainer(ctx, s.OutDir(), cr.User(), vm.ContainerType{Image: vm.StagingImageServer, Arch: p.arch}, "", p.gpuEnabled, p.diskSize); err != nil {
			s.Fatal("Failed to set up default container (from download): ", err)
		}
	case artifact:
		s.Log("Setting up component (from artifact)")
		artifactPath := s.DataPath(ImageArtifact)
		if err = vm.MountArtifactComponent(ctx, artifactPath); err != nil {
			s.Fatal("Failed to set up component: ", err)
		}
		s.Log("Creating default container (from artifact)")
		if p.cont, err = vm.CreateDefaultVMContainer(ctx, s.OutDir(), cr.User(), vm.ContainerType{Image: vm.Tarball, Arch: p.arch}, artifactPath, p.gpuEnabled, p.diskSize); err != nil {
			s.Fatal("Failed to set up default container (from artifact): ", err)
		}
	default:
		s.Fatal("Unrecognized mode: ", p.mode)
	}
	if p.useInstaller {
		s.Log("Installing crostini")
		if err := p.tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.runCrostiniInstaller)()`, nil); err != nil {
			s.Fatal("Running autotestPrivate.runCrostiniInstaller failed: ", err)
		}
	}

	// The VM should now be running, check that all the host daemons are also running to catch any errors in our init scripts etc.
	if err = checkDaemonsRunning(ctx); err != nil {
		s.Fatal("VM host daemons in an unexpected state: ", err)
	}

	if p.keyboard, err = input.Keyboard(ctx); err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
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

	ret := p.buildFixtData(ctx, cr, s)

	vmDiskSize, err := p.cont.VM.DiskSize()
	if err != nil {
		s.Fatal("Failed to query the disk size of the VM: ", err)
	}
	s.Logf("VM Disk size: %.1fGB", float64(vmDiskSize)/1024/1024/1024)

	chrome.Lock()
	vm.Lock()
	shouldClose = false
	return ret
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructors.
func (p *fixtImpl) Close(ctx context.Context, s *testing.FixtState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	vm.Unlock()
	chrome.Unlock()
	p.cleanUp(ctx, s)
}

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (p *fixtImpl) cleanUp(ctx context.Context, s *testing.FixtState) {
	if p.keyboard != nil {
		if err := p.keyboard.Close(); err != nil {
			s.Log("Failure closing keyboard: ", err)
		}
		p.keyboard = nil
	}

	if p.cont != nil {
		if err := p.cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Log("Failure dumping container log: ", err)
		}
		if err := vm.StopConcierge(ctx); err != nil {
			s.Log("Failure stopping concierge: ", err)
		}
		p.cont = nil
	}
	// It is always safe to unmount the component, which just posts some
	// logs if it was never mounted.
	vm.UnmountComponent(ctx)

	// Nothing special needs to be done to close the test API connection.
	p.tconn = nil
}

// buildPreData is a helper method that resets the machine state in
// advance of building the precondition data for the actual tests.
func (p *fixtImpl) buildFixtData(ctx context.Context, cr *chrome.Chrome, s *testing.PreState) FixtData {
	return FixtData{cr, p.tconn, p.cont, p.keyboard}
}
