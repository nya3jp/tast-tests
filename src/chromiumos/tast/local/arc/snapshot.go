// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Snapshot represents a Snapshot of ARC state. Fixtures and preconditions can
// use a Snapshot to revert ARC state to the original one after running a test.
type Snapshot struct {
	initPID       int32               // PID (outside container) of ARC init process
	installedPkgs map[string]struct{} // installed packages
	runningPkgs   map[string]struct{} // running packages
}

// NewSnapshot captures an ARC state snapshot.
func NewSnapshot(ctx context.Context, a *ARC) (*Snapshot, error) {
	initPID, err := InitPID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get init PID")
	}

	installedPkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list packages")
	}

	runningPkgs, err := runningPackages(ctx, a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list running packages")
	}

	return &Snapshot{
		initPID:       initPID,
		installedPkgs: installedPkgs,
		runningPkgs:   runningPkgs,
	}, nil
}

// Restore restores the ARC state to the snapshot state.
func (s *Snapshot) Restore(ctx context.Context, a *ARC) error {
	cur, err := NewSnapshot(ctx, a)
	if err != nil {
		return err
	}
	if err := s.checkUsable(ctx, a, cur); err != nil {
		return err
	}
	if err := s.restorePackages(ctx, a, cur); err != nil {
		return err
	}
	return nil
}

func (s *Snapshot) checkUsable(ctx context.Context, a *ARC, cur *Snapshot) error {
	ctx, st := timing.Start(ctx, "check_arc")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check that the init process is the same as before. Otherwise, ARC was probably restarted.
	if cur.initPID != s.initPID {
		return errors.Errorf("init process changed from %v to %v; probably crashed", s.initPID, cur.initPID)
	}

	// Check that the package manager service is running.
	const pkgi = "android"
	if _, ok := cur.installedPkgs[pkgi]; !ok {
		return errors.Errorf("pm didn't list %q among %d package(s)", pkgi, len(cur.installedPkgs))
	}

	// Check that home package is running.
	const pkgr = "org.chromium.arc.home"
	if _, ok := cur.runningPkgs[pkgr]; !ok {
		return errors.Errorf("package %q is not running", pkgr)
	}
	return nil
}

func (s *Snapshot) restorePackages(ctx context.Context, a *ARC, cur *Snapshot) error {
	ctx, st := timing.Start(ctx, "restore_packages")
	defer st.End()

	// Stop any packages that weren't present when ARC booted. Stop before uninstall.
	for pkg := range cur.runningPkgs {
		if _, ok := s.runningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := a.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Uninstall any packages that weren't present when ARC booted.
	for pkg := range cur.installedPkgs {
		if _, ok := s.installedPkgs[pkg]; ok {
			continue
		}
		testing.ContextLog(ctx, "Uninstalling ", pkg)
		if err := a.Command(ctx, "pm", "uninstall", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to uninstall %v", pkg)
		}
	}
	return nil
}

// runningPackages returns a set of currently-running packages, e.g. "com.android.settings".
// It queries all running activities, but it returns the activity's package name.
func runningPackages(ctx context.Context, a *ARC) (map[string]struct{}, error) {
	tasks, err := a.DumpsysActivityActivities(ctx)
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
