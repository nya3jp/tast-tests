// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 30 * time.Second

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome
	// ARC enables interaction with an already-started ARC environment.
	// It cannot be closed by tests.
	ARC *ARC
}

// Booted returns a precondition that ARC Container has already booted when a test is run.
//
// When adding a test, the testing.Test.Pre field may be set to the value returned by this function.
// Later, in the main test function, the value returned by testing.State.PreValue may be converted
// to a PreData containing already-initialized chrome.Chrome and ARC objects:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(arc.PreData)
//		conn, err := d.Chrome.NewConn(ctx, "http://www.example.org/")
//		...
//		cmd := d.ARC.Command(ctx, "dumpsys", "window", "displays")
//		...
//	}
//
// When using this precondition, tests cannot call New or chrome.New.
// The Chrome and ARC instances are also shared and cannot be closed by tests.
func Booted() testing.Precondition { return bootedPre }

// bootedPre is returned by Booted.
var bootedPre = &preImpl{
	name:    "arc_booted",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
}

// VMBooted returns a precondition similar to Booted(). The only difference from Booted() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
func VMBooted() testing.Precondition { return vmBootedPre }

// vmBootedPre is returned by VMBooted.
var vmBootedPre = &preImpl{
	name:      "arcvm_booted",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--enable-arcvm"},
}

// BootedInTabletMode returns a precondition similar to Booted(). The only difference from Booted() is
// that Chrome is launched in tablet mode in this precondition.
func BootedInTabletMode() testing.Precondition { return bootedInTabletModePre }

// bootedInTabletModePre is returned by BootedInTabletMode.
var bootedInTabletModePre = &preImpl{
	name:      "arc_booted_in_tablet_mode",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--force-tablet-mode=touch_view", "--enable-virtual-keyboard"},
}

// VMBootedInTabletMode returns a precondition similar to BootedInTabletMode().
// The only difference from BootedInTabletMode() is that Chrome is launched in tablet mode in this precondition.
func VMBootedInTabletMode() testing.Precondition { return vmBootedInTabletModePre }

// vmBootedInTabletModePre is returned by VMBootedInTabletMode.
var vmBootedInTabletModePre = &preImpl{
	name:      "arcvm_booted_in_tablet_mode",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--enable-arcvm", "--force-tablet-mode=touch_view", "--enable-virtual-keyboard"},
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	name    string        // testing.PreconditionImpl.String
	timeout time.Duration // testing.PreconditionImpl.Timeout

	extraArgs []string // passed to Chrome on initialization
	cr        *chrome.Chrome
	arc       *ARC

	origInitPID       int32               // initial PID (outside container) of ARC init process
	origInstalledPkgs map[string]struct{} // initially-installed packages
	origRunningPkgs   map[string]struct{} // initially-running packages
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.arc != nil {
		pre, err := func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			installed, err := p.installedPackages(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get installed packages")
			}
			running, err := p.runningPackages(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get running packages")
			}
			if err := p.checkUsable(ctx, installed, running); err != nil {
				return nil, errors.Wrap(err, "existing Chrome or ARC connection is unusable")
			}
			if err := p.resetState(ctx, installed, running); err != nil {
				return nil, errors.Wrap(err, "failed resetting existing Chrome or ARC session")
			}
			if err := p.arc.setLogcatFile(filepath.Join(s.OutDir(), logcatName)); err != nil {
				return nil, errors.Wrap(err, "failed to update logcat output file")
			}
			return PreData{p.cr, p.arc}, nil
		}()
		if err == nil {
			s.Log("Reusing existing ARC session")
			return pre
		}
		s.Log("Failed to reuse existing ARC session: ", err)
		locked = false
		chrome.Unlock()
		p.closeInternal(ctx, s)
	}

	// Revert partial initialization.
	shouldClose := true
	defer func() {
		if shouldClose {
			p.closeInternal(ctx, s)
		}
	}()

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		var err error
		if p.cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(p.extraArgs...)); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

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
		if p.origInstalledPkgs, err = p.installedPackages(ctx); err != nil {
			s.Fatal("Failed to list initial packages: ", err)
		}
		if p.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
			s.Fatal("Failed to list running packages: ", err)
		}
	}()

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	locked = true
	chrome.Lock()

	shouldClose = false
	return PreData{p.cr, p.arc}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// installedPackages returns a set of currently-installed packages, e.g. "android".
// This operation is slow (700+ ms), so unnecessary calls should be avoided.
func (p *preImpl) installedPackages(ctx context.Context) (map[string]struct{}, error) {
	ctx, st := timing.Start(ctx, "installed_packages")
	defer st.End()

	out, err := p.arc.Command(ctx, "pm", "list", "packages").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "listing packages failed")
	}

	pkgs := make(map[string]struct{})
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// |pm list packages| prepends "package:" to installed packages. Not needed.
		n := strings.TrimPrefix(pkg, "package:")
		pkgs[n] = struct{}{}
	}
	return pkgs, nil
}

// runningPackages returns a set of currently-running packages, e.g. "com.android.settings".
// It queries all running activities, but it returns the activity's package name.
func (p *preImpl) runningPackages(ctx context.Context) (map[string]struct{}, error) {
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
// installed should come from installedPackages.
// running should come from runningPackages.
func (p *preImpl) checkUsable(ctx context.Context, installed, running map[string]struct{}) error {
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
// installed should come from installedPackages.
// running should come from runningPackages.
func (p *preImpl) resetState(ctx context.Context, installed, running map[string]struct{}) error {
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
func (p *preImpl) closeInternal(ctx context.Context, s *testing.State) {
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
