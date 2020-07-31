// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddFixt(&testing.Fixt{
		Name: "arc_booted",
		Impl: BootedFixt(),
		// TODO: should chrome.LoginTimeout be included here?
		Timeout:  resetTimeout + BootTimeout,
		Desc:     "Fixture to set up ARC.",
		Contacts: []string{"oka@chromium.org"},
		Fixt:     "chrome_fixt",
	})
}

// FixtData holds information made available to tests that specify fixtures.
type FixtData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome
	// ARC enables interaction with an already-started ARC environment.
	// It cannot be closed by tests.
	ARC *ARC
}

// BootedFixt returns a fixture that ARC Container has already booted when a test is run.
//
// When using this fixture, tests cannot call New or chrome.New.
// The Chrome and ARC instances are also shared and cannot be closed by tests.
func BootedFixt() testing.Fixture { return bootedFixt }

// bootedFixt is returned by Booted.
var bootedFixt = &fixtImpl{
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
}

// fixtImpl implements testing.Fixture.
type fixtImpl struct {
	name    string
	timeout time.Duration // testing.Fixture.Timeout

	extraArgs []string  // passed to Chrome on initialization
	gaia      *GaiaVars // a struct containing GAIA secret variables

	cr  *chrome.Chrome
	arc *ARC

	origInitPID       int32               // initial PID (outside container) of ARC init process
	origInstalledPkgs map[string]struct{} // initially-installed packages
	origRunningPkgs   map[string]struct{} // initially-running packages
}

// Prepare is called by the test framework at the beginning of every test using this fixture.
// It returns a FixtData containing objects that can be used by the test.
func (p *fixtImpl) Prepare(ctx context.Context, s *testing.FixtState) interface{} {
	p.name = s.Name()
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	p.cr = s.PreValue().(*chrome.Chrome)

	// Create new ARC instance.
	func() {
		ctx, cancel := context.WithTimeout(ctx, BootTimeout)
		defer cancel()
		var err error
		if p.arc, err = New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		if p.origInitPID, err = InitPID(); err != nil {
			s.Fatal("Failed to get initial init PID: ", err)
		}
		if p.origInstalledPkgs, err = p.arc.InstalledPackages(ctx); err != nil {
			s.Fatal("Failed to list initial packages: ", err)
		}
		if p.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
			s.Fatal("Failed to list running packages: ", err)
		}
	}()

	return FixtData{p.cr, p.arc}
}

func (p *fixtImpl) Adjust(ctx context.Context, s *testing.FixtTestState) error {
	ctx, cancel := context.WithTimeout(ctx, resetTimeout)
	defer cancel()
	ctx, st := timing.Start(ctx, "reset_"+p.name)
	defer st.End()
	installed, err := p.arc.InstalledPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get installed packages")
	}
	running, err := p.runningPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get running packages")
	}
	if err := p.checkUsable(ctx, installed, running); err != nil {
		return errors.Wrap(err, "existing Chrome or ARC connection is unusable")
	}
	if err := p.resetState(ctx, installed, running); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome or ARC session")
	}
	if err := p.arc.setLogcatFile(filepath.Join(s.OutDir(), logcatName)); err != nil {
		return errors.Wrap(err, "failed to update logcat output file")
	}
	return nil
}

func (p *fixtImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Do nothing
}

// Close is called by the test framework after the last test that uses this fixture.
func (p *fixtImpl) Close(ctx context.Context, s *testing.FixtState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// runningPackages returns a set of currently-running packages, e.g. "com.android.settings".
// It queries all running activities, but it returns the activity's package name.
func (p *fixtImpl) runningPackages(ctx context.Context) (map[string]struct{}, error) {
	tasks, err := p.arc.DumpsysActivityActivities(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing activities failed")
	}

	acts := make(map[string]struct{})
	for _, t := range tasks {
		for _, a := range t.ActivityInfos {
			acts[a.PackageName] = struct{}{}
		}
	}
	return acts, nil
}

// checkUsable verifies that p.cr and p.arc are still usable. Both must be non-nil.
// installed should come from InstalledPackages.
// running should come from runningPackages.
func (p *fixtImpl) checkUsable(ctx context.Context, installed, running map[string]struct{}) error {
	ctx, st := timing.Start(ctx, "check_arc")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check that the init process is the same as before. Otherwise, ARC was probably restarted.
	if pid, err := InitPID(); err != nil {
		return err
	} else if pid != p.origInitPID {
		return errors.Errorf("init process changed from %v to %v; probably crashed", p.origInitPID, pid)
	}

	// Check that the package manager service is running.
	const pkgi = "android"
	if _, ok := installed[pkgi]; !ok {
		return errors.Errorf("pm didn't list %q among %d package(s)", pkgi, len(installed))
	}

	// Check that home package is running.
	const pkgr = "org.chromium.arc.home"
	if _, ok := running[pkgr]; !ok {
		return errors.Errorf("package %q is not running", pkgr)
	}

	// TODO(nya): Should we also check that p.cr is still usable?
	return nil
}

// resetState resets ARC's and Chrome's state between tests.
// installed should come from InstalledPackages.
// running should come from runningPackages.
func (p *fixtImpl) resetState(ctx context.Context, installed, running map[string]struct{}) error {
	// Stop any packages that weren't present when ARC booted. Stop before uninstall.
	for pkg := range running {
		if _, ok := p.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := p.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Uninstall any packages that weren't present when ARC booted.
	for pkg := range installed {
		if _, ok := p.origInstalledPkgs[pkg]; ok {
			continue
		}
		testing.ContextLog(ctx, "Uninstalling ", pkg)
		if err := p.arc.Command(ctx, "pm", "uninstall", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to uninstall %v", pkg)
		}
	}

	if err := p.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting Chrome's state")
	}
	return nil
}

// closeInternal closes and resets p.arc and p.cr if non-nil.
func (p *fixtImpl) closeInternal(ctx context.Context, s *testing.FixtState) {
	if p.arc != nil {
		if err := p.arc.Close(); err != nil {
			s.Log("Failed to close ARC connection: ", err)
		}
		p.arc = nil
	}
	p.origInstalledPkgs = nil
	p.origRunningPkgs = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
}
