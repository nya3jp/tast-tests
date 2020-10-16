// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// StateManager store all data needed to create, reset, and close ARC.
type StateManager struct {
	arc *ARC

	origInitPID       int32               // initial PID (outside container) of ARC init process
	origInstalledPkgs map[string]struct{} // initially-installed packages
	origRunningPkgs   map[string]struct{} // initially-running packages
}

// Active returns true iff this ARC instance has been Activated.
func (i *StateManager) Active() bool {
	return i.arc != nil
}

// TODO: change to Activate/Deactivate instead of initialize close

// Activate a new ARC instance, remembering its state so we can reset it.
func (i *StateManager) Activate(ctx context.Context, outDir string) error {
	var err error
	if i.arc, err = New(ctx, outDir); err != nil {
		return errors.Wrap(err, "failed to start ARC")
	}
	if i.origInitPID, err = InitPID(); err != nil {
		return errors.Wrap(err, "failed to get initial init PID")
	}
	if i.origInstalledPkgs, err = i.arc.InstalledPackages(ctx); err != nil {
		return errors.Wrap(err, "failed to list initial packages")
	}
	if i.origRunningPkgs, err = i.runningPackages(ctx); err != nil {
		return errors.Wrap(err, "failed to list running packages")
	}
	return nil
}

// CheckAndReset the ARC instance between test runs.
func (i *StateManager) CheckAndReset(ctx context.Context, outDir string) error {
	installed, err := i.arc.InstalledPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get installed packages")
	}
	running, err := i.runningPackages(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get running packages")
	}
	if err := i.checkUsable(ctx, installed, running); err != nil {
		return errors.Wrap(err, "existing Chrome or ARC connection is unusable")
	}
	if err := i.resetState(ctx, installed, running); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome or ARC session")
	}
	if err := i.arc.setLogcatFile(filepath.Join(outDir, logcatName)); err != nil {
		return errors.Wrap(err, "failed to update logcat output file")
	}
	return nil
}

// Deactivate and reset ARC instance if non nil.
func (i *StateManager) Deactivate(ctx context.Context) error {
	i.origInstalledPkgs = nil
	i.origRunningPkgs = nil
	if i.arc == nil {
		return nil
	}
	err := i.arc.Close()
	i.arc = nil
	if err != nil {
		return errors.Wrap(err, "failed to close ARC connection")
	}
	return nil
}

// ARC gets the *ARC whose state we are managing.
func (i *StateManager) ARC() *ARC {
	return i.arc
}

// runningPackages returns a set of currently-running packages, e.g. "com.android.settings".
// It queries all running activities, but it returns the activity's package name.
func (i *StateManager) runningPackages(ctx context.Context) (map[string]struct{}, error) {
	tasks, err := i.arc.DumpsysActivityActivities(ctx)
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
func (i *StateManager) checkUsable(ctx context.Context, installed, running map[string]struct{}) error {
	ctx, st := timing.Start(ctx, "check_arc")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check that the init process is the same as before. Otherwise, ARC was probably restarted.
	if pid, err := InitPID(); err != nil {
		return err
	} else if pid != i.origInitPID {
		return errors.Errorf("init process changed from %v to %v; probably crashed", i.origInitPID, pid)
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

	return nil
}

// resetState resets ARC's state between tests.
// installed should come from InstalledPackages.
// running should come from runningPackages.
func (i *StateManager) resetState(ctx context.Context, installed, running map[string]struct{}) error {
	// Stop any packages that weren't present when ARC booted. Stop before uninstall.
	for pkg := range running {
		if _, ok := i.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := i.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Uninstall any packages that weren't present when ARC booted.
	for pkg := range installed {
		if _, ok := i.origInstalledPkgs[pkg]; ok {
			continue
		}
		testing.ContextLog(ctx, "Uninstalling ", pkg)
		if err := i.arc.Command(ctx, "pm", "uninstall", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to uninstall %v", pkg)
		}
	}

	return nil
}
