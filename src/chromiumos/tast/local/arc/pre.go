// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome
	// ARC enables interaction with an already-started ARC environment.
	// It cannot be closed by tests.
	ARC *ARC
}

// Booted returns a precondition that ARC has already booted when a test is run.
//
// When adding a test, the testing.Test.Pre field may be set to the value returned by this function.
// Later, in the main test function, the value returned by testing.State.PreValue may be converted
// to a PreData containing already-initialized chrome.Chrome and ARC objects:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(arc.PreData)
//		conn, err := d.Chrome.NewConn(ctx, "http://www.example.org/")
//		...
//      cmd := d.ARC.Command(ctx, "dumpsys", "window", "displays")
//		...
//	}
//
// When using this precondition, tests cannot call New or chrome.New.
// The Chrome and ARC instances are also shared and cannot be closed by tests.
func Booted() testing.Precondition { return bootedPre }

// bootedPre is returned by Booted.
var bootedPre = &preImpl{
	name:    "arc_booted",
	timeout: chrome.LoginTimeout + BootTimeout,
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	name    string        // testing.PreconditionImpl.String
	timeout time.Duration // testing.PreconditionImpl.Timeout

	cr  *chrome.Chrome
	arc *ARC

	origInitPID  int32               // initial PID (outside container) of ARC init process
	origPackages map[string]struct{} // initially-installed packages
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	defer timing.Start(ctx, "prepare_"+p.name).End()

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	defer func() {
		locked = true
		chrome.Lock()
	}()
	locked = false
	chrome.Unlock()

	if p.arc != nil {
		if pkgs, err := p.installedPackages(ctx); err != nil {
			s.Log("Failed to get installed packages: ", err)
		} else if err := p.checkUsable(ctx, pkgs); err != nil {
			s.Log("Existing Chrome or ARC connection is unusable: ", err)
		} else if err := p.resetState(ctx, pkgs); err != nil {
			s.Log("Failed resetting existing Chrome or ARC session: ", err)
		} else if outDir, ok := testing.ContextOutDir(ctx); !ok {
			s.Log("Failed to get output dir from context")
		} else if err := p.arc.setLogcatFile(filepath.Join(outDir, logcatName)); err != nil {
			s.Log("Failed to update logcat output file: ", err)
		} else {
			s.Log("Reusing existing ARC session")
			return PreData{p.cr, p.arc}
		}
		p.closeInternal(ctx, s)
	}

	// Revert partial initialization.
	shouldClose := true
	defer func() {
		if shouldClose {
			p.closeInternal(ctx, s)
		}
	}()

	var err error
	if p.cr, err = chrome.New(ctx, chrome.ARCEnabled()); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		s.Fatal("Failed to get output dir")
	}
	if p.arc, err = New(ctx, outDir); err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	if p.origInitPID, err = p.initPID(); err != nil {
		s.Fatal("Failed to get initial init PID: ", err)
	}
	if p.origPackages, err = p.installedPackages(ctx); err != nil {
		s.Fatal("Failed to list initial packages: ", err)
	}

	shouldClose = false
	return PreData{p.cr, p.arc}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	defer timing.Start(ctx, "close_"+p.name).End()
	locked = false
	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// initPID returns the PID (outside the container) of the ARC init process.
func (p *preImpl) initPID() (int32, error) {
	uid, err := sysutil.GetUID("android-root")
	if err != nil {
		return -1, err
	}

	procs, err := process.Processes()
	if err != nil {
		return -1, errors.Wrap(err, "failed to list processes")
	}

	const initPath = "/init"
	for _, p := range procs {
		if uids, err := p.Uids(); err == nil && uint32(uids[0]) == uid {
			if exe, err := p.Exe(); err == nil && exe == initPath {
				return p.Pid, nil
			}
		}
	}
	return -1, errors.New("didn't find init process")
}

// installedPackages returns a set of currently-installed packages, e.g. "package:android".
// This operation is slow (700+ ms), so unnecessary calls should be avoided.
func (p *preImpl) installedPackages(ctx context.Context) (map[string]struct{}, error) {
	defer timing.Start(ctx, "installed_packages").End()
	out, err := p.arc.Command(ctx, "pm", "list", "packages").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "listing packages failed")
	}

	pkgs := make(map[string]struct{})
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkgs[pkg] = struct{}{}
	}
	return pkgs, nil
}

// checkUsable verifies that p.cr and p.arc are still usable. Both must be non-nil.
// pkgs should come from installedPackages.
func (p *preImpl) checkUsable(ctx context.Context, pkgs map[string]struct{}) error {
	defer timing.Start(ctx, "check_arc").End()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check that the init process is the same as before. Otherwise, ARC was probably restarted.
	if pid, err := p.initPID(); err != nil {
		return err
	} else if pid != p.origInitPID {
		return errors.Errorf("init process changed from %v to %v; probably crashed", p.origInitPID, pid)
	}

	// Check that the package manager service is running.
	const pkg = "package:android"
	if _, ok := pkgs[pkg]; !ok {
		return errors.Errorf("pm didn't list %q among %d package(s)", pkg, len(pkgs))
	}

	// TODO(derat): Should we also check that p.cr is still usable?
	return nil
}

// resetState resets ARC's and Chrome's state between tests.
// pkgs should come from installedPackages.
func (p *preImpl) resetState(ctx context.Context, pkgs map[string]struct{}) error {
	// Uninstall any packages that weren't present when ARC booted.
	for pkg := range pkgs {
		if _, ok := p.origPackages[pkg]; ok {
			continue
		}
		testing.ContextLog(ctx, "Uninstalling ", pkg)
		if err := adbCommand(ctx, "uninstall", pkg).Run(testexec.DumpLogOnError); err != nil {
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
	p.origPackages = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
}
