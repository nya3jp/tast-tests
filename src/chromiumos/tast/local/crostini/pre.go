// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
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
	TestAPIConn *chrome.TestConn
	Container   *vm.Container
	Keyboard    *input.KeyboardEventWriter
}

// StartedByArtifact is similar to StartedByDownload, but will
// use a pre-built image as a data-dependency rather than downloading one. To
// use this precondition you must have crostini.ImageArtifact as a data dependency.
func StartedByArtifact() testing.Precondition { return startedByArtifactPre }

// StartedByDownload is a precondition that ensures a tast test will
// begin after crostini has been started by downloading an image.
func StartedByDownload() testing.Precondition { return startedByDownloadPre }

// StartedByDownloadBuster is a precondition that ensures a tast test
// will begin after crostini has been started by downloading an image
// running debian buster.
func StartedByDownloadBuster() testing.Precondition { return startedByDownloadBusterPre }

// StartedGPUEnabled is similar to StartedByArtifact, but will
// use pass enable-gpu to vm instance to allow gpu being used.
func StartedGPUEnabled() testing.Precondition { return startedGPUEnabledPre }

// StartedGPUEnabledBuster is similar to StartedGPUEnabled, but will
// started by downloading an image running debian buster.
func StartedGPUEnabledBuster() testing.Precondition { return startedGPUEnabledBusterPre }

// StartedTraceVM will try to setup a debian buster VM with GPU enabled and a large disk.
func StartedTraceVM() testing.Precondition { return startedTraceVMPre }

// StartedARCEnabled is similar to StartedByArtifact, but will start Chrome
// with ARCEnabled() option.
func StartedARCEnabled() testing.Precondition { return startedARCEnabledPre }

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
)

var startedByArtifactPre = &preImpl{
	name:     "crostini_started_by_artifact",
	timeout:  chrome.LoginTimeout + 7*time.Minute,
	mode:     artifact,
	diskSize: vm.DefaultDiskSize,
}

var startedByDownloadPre = &preImpl{
	name:     "crostini_started_by_download_stretch",
	timeout:  chrome.LoginTimeout + 10*time.Minute,
	mode:     download,
	diskSize: vm.DefaultDiskSize,
}

var startedByDownloadBusterPre = &preImpl{
	name:     "crostini_started_by_download_buster",
	timeout:  chrome.LoginTimeout + 10*time.Minute,
	mode:     download,
	arch:     vm.DebianBuster,
	diskSize: vm.DefaultDiskSize,
}

var startedGPUEnabledPre = &preImpl{
	name:       "crostini_started_gpu_enabled",
	timeout:    chrome.LoginTimeout + 10*time.Minute,
	mode:       artifact,
	gpuEnabled: true,
	diskSize:   vm.DefaultDiskSize,
}

var startedGPUEnabledBusterPre = &preImpl{
	name:       "crostini_started_gpu_enabled_buster",
	timeout:    chrome.LoginTimeout + 10*time.Minute,
	arch:       vm.DebianBuster,
	mode:       download,
	gpuEnabled: true,
	diskSize:   vm.DefaultDiskSize,
}

var startedTraceVMPre = &preImpl{
	name:       "crostini_started_trace_vm",
	timeout:    chrome.LoginTimeout + 10*time.Minute,
	arch:       vm.DebianBuster,
	mode:       download,
	gpuEnabled: true,
	diskSize:   8 * 1024 * 1024 * 1024,
}

var startedARCEnabledPre = &preImpl{
	name:       "crostini_started_arc_enabled",
	timeout:    chrome.LoginTimeout + 10*time.Minute,
	mode:       artifact,
	arcEnabled: true,
	diskSize:   vm.DefaultDiskSize,
}

var startedByInstallerPre = &preImpl{
	name:         "crostini_started_by_installer",
	timeout:      chrome.LoginTimeout + 7*time.Minute,
	mode:         artifact,
	useInstaller: true,
	diskSize:     vm.DefaultDiskSize,
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
	diskSize     uint64               // The size of the VM image.
	cr           *chrome.Chrome
	tconn        *chrome.TestConn
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
		if err := SimpleCommandWorks(ctx, p.cont); err != nil {
			s.Log("Precondition unsatisifed: ", err)
			p.cont = nil
			p.Close(ctx, s)
		} else if err := p.cr.Responded(ctx); err != nil {
			s.Log("Precondition unsatisfied: Chrome is unresponsive: ", err)
			p.Close(ctx, s)
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

	opt := chrome.ARCDisabled()
	if p.arcEnabled {
		opt = chrome.ARCEnabled()
	}

	// To help identify sources of flake, we report disk usage before the test.
	if size, err := checkStatefulDisk(ctx, used); err != nil {
		s.Log("Failed to check disk usage: ", err)
	} else {
		s.Logf("Disk has %s space used", size)
	}
	if size, err := checkStatefulDisk(ctx, available); err != nil {
		s.Log("Failed to check disk availability: ", err)
	} else {
		s.Logf("Disk has %s space available", size)
	}

	var err error
	if p.cr, err = chrome.New(ctx, opt); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
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
		if p.cont, err = vm.CreateDefaultVMContainer(ctx, s.OutDir(), p.cr.User(), vm.ContainerType{Image: vm.StagingImageServer, Arch: p.arch}, "", p.gpuEnabled, p.diskSize); err != nil {
			s.Fatal("Failed to set up default container (from download): ", err)
		}
	case artifact:
		s.Log("Setting up component (from artifact)")
		artifactPath := s.DataPath(ImageArtifact)
		if err = vm.MountArtifactComponent(ctx, artifactPath); err != nil {
			s.Fatal("Failed to set up component: ", err)
		}
		s.Log("Creating default container (from artifact)")
		if p.cont, err = vm.CreateDefaultVMContainer(ctx, s.OutDir(), p.cr.User(), vm.ContainerType{Image: vm.Tarball, Arch: p.arch}, artifactPath, p.gpuEnabled, p.diskSize); err != nil {
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

	ret := p.buildPreData(ctx, s)
	s.Logf("VM Disk size: %.1fGB", float64(p.cont.VM.Concierge.DiskSize())/1024/1024/1024)

	chrome.Lock()
	vm.Lock()
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

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failure closing chrome: ", err)
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

type fstatMetric int

const (
	available fstatMetric = iota
	used
)

// checkStatefulDisk reports the chosen fstat |metric| in the /mnt/stateful
// partition. Returns the size as a human-readable string like "12G".
func checkStatefulDisk(ctx context.Context, metric fstatMetric) (string, error) {
	var result unix.Statfs_t
	if err := unix.Statfs("/mnt/stateful_partition", &result); err != nil {
		return "", err
	}
	var bytes uint64
	switch metric {
	case available:
		bytes = result.Bavail
	case used:
		bytes = result.Blocks - result.Bfree
	}
	bytes = bytes * uint64(result.Bsize)
	return fmt.Sprintf("%.1fG", float64(bytes)/1024/1024/1024), nil
}
