// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// CrostiniName is a stable name for the Crostini VM.
const CrostiniName = "Crostini"

// CrostiniOptions describe how to start Crostini.
type CrostiniOptions struct {
	Mode           string                    // Where (download/build artifact) the container image comes from.
	DebianVersion  vm.ContainerDebianVersion // OS version of the container image.
	MinDiskSize    uint64                    // The minimum size of the VM image in bytes. 0 to use default disk size.
	LargeContainer bool
}

// crostiniActivation represents an instance of the Crostini VM used in a
// multi-VM test.
type crostiniActivation struct {
	container *vm.Container
}

// Name returns a stable name for the Crostini VM.
func (o CrostiniOptions) Name() string {
	return CrostiniName
}

// ChromeOpts returns the Chrome option(s) that should be passed to
// chrome.New().
func (o CrostiniOptions) ChromeOpts() []chrome.Option {
	return []chrome.Option{chrome.ExtraArgs("--vmodule=crostini*=1")}
}

// ActivateTimeout returns the time needed to setup the Crostini VM.
func (o CrostiniOptions) ActivateTimeout() time.Duration {
	return 7 * time.Minute
}

// Activate spins up the Crostini VM.
func (o CrostiniOptions) Activate(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, st StateManagerTestingState) (VMActivation, error) {
	testing.ContextLog(ctx, "Creating Crostini")
	iOptions := crostini.GetInstallerOptions(st, true, o.DebianVersion, o.LargeContainer, cr.NormalizedUser())
	iOptions.UserName = cr.NormalizedUser()
	iOptions.MinDiskSize = o.MinDiskSize
	if _, err := cui.InstallCrostini(ctx, tconn, cr, iOptions); err != nil {
		return nil, errors.Wrap(err, "installing Crostini")
	}

	var container *vm.Container
	success := false
	defer func() {
		if !success {
			if container != nil {
				cleanupCrostini(ctx, container)
			}
		}
	}()

	// Container may be set even when an error is returned. This must be cleaned
	// up if there was an error.
	container, err := vm.DefaultContainer(ctx, cr.NormalizedUser())
	if err != nil {
		return nil, errors.Wrap(err, "connecting to running container")
	}
	success = true
	vm.Lock()
	return &crostiniActivation{container: container}, nil
}

// CheckAndReset checks the Crostini VM between tests.
func (c *crostiniActivation) CheckAndReset(ctx context.Context, st StateManagerTestingState) error {
	if err := crostini.BasicCommandWorks(ctx, c.container); err != nil {
		return errors.Wrap(err, "checking Crostini")
	}
	return nil
}

// Deactivate stops the Crostini VM.
func (c *crostiniActivation) Deactivate(ctx context.Context) error {
	vm.Unlock()
	return cleanupCrostini(ctx, c.container)
}

func cleanupCrostini(ctx context.Context, container *vm.Container) error {
	if err := container.VM.Stop(ctx); err != nil {
		return errors.Wrap(err, "stopping Crostini")
	}
	return nil
}

// VM returns the underlying crostini container.
func (c *crostiniActivation) VM() interface{} {
	return c.container
}

// CrostiniFromPre returns the Crostini instance setup by the multi-vm
// precondition, if available, and nil otherwise.
func CrostiniFromPre(pre *PreData) *vm.Container {
	if container, ok := pre.VMs[CrostiniName]; ok {
		return container.(*vm.Container)
	}
	return nil
}
