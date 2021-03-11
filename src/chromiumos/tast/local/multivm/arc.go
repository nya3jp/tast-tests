// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ARCName is a stable name for the ARC VM.
const ARCName = "ARC"

// ARCOptions describes how to start ARC.
type ARCOptions struct {
}

// arcActivation represents an instance of the ARC VM used in a multi-VM test.
type arcActivation struct {
	vm       *arc.ARC
	snapshot *arc.Snapshot
}

// Name returns a stable name for the ARC VM.
func (o ARCOptions) Name() string {
	return ARCName
}

// ChromeOpts returns the Chrome option(s) that should be passed to
// chrome.New().
func (o ARCOptions) ChromeOpts() []chrome.Option {
	return []chrome.Option{chrome.ARCEnabled()}
}

// ActivateTimeout returns the timeout needed to setup the ARC VM.
func (o ARCOptions) ActivateTimeout() time.Duration {
	return arc.BootTimeout
}

// Activate spins up the ARC VM.
func (o ARCOptions) Activate(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, st StateManagerTestingState) (VMActivation, error) {
	testing.ContextLog(ctx, "Creating ARC")
	vm, err := arc.New(ctx, st.OutDir())
	if err != nil {
		return nil, errors.Wrap(err, "starting ARC")
	}
	success := false
	defer func() {
		if !success {
			if vm != nil {
				cleanupARC(ctx, vm)
			}
		}
	}()

	var snapshot *arc.Snapshot
	snapshot, err = arc.NewSnapshot(ctx, vm)
	if err != nil {
		return nil, errors.Wrap(err, "taking ARC state snapshot")
	}
	success = true
	arc.Lock()
	return &arcActivation{vm: vm, snapshot: snapshot}, nil
}

// CheckAndReset restores the state of ARC VM between tests.
func (a *arcActivation) CheckAndReset(ctx context.Context, st StateManagerTestingState) error {
	if err := a.snapshot.Restore(ctx, a.vm); err != nil {
		return errors.Wrap(err, "restoring ARC")
	}
	return nil
}

// Deactivate stops the ARC VM.
func (a *arcActivation) Deactivate(ctx context.Context) error {
	arc.Unlock()
	return cleanupARC(ctx, a.vm)
}

func cleanupARC(ctx context.Context, vm *arc.ARC) error {
	if err := vm.Close(ctx); err != nil {
		return errors.Wrap(err, "closing ARC")
	}
	return nil
}

// VM returns the underlying ARC VM instance.
func (a *arcActivation) VM() interface{} {
	return a.vm
}

// ARCFromPre returns the ARC instance setup by the multi-vm precondition,
// if available, and nil otherwise.
func ARCFromPre(pre *PreData) *arc.ARC {
	if vm, ok := pre.VMs[ARCName]; ok {
		return vm.(*arc.ARC)
	}
	return nil
}
