// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/dlc"
	"chromiumos/tast/testing"
)

const (
	// TerminaDlcID is the name of the Chrome component for the VM kernel and rootfs.
	TerminaDlcID = "termina-dlc"

	// TerminaDlcDir is a path to the location where the DLC is extracted.
	TerminaDlcDir = "/run/imageloader/termina-dlc/package/root"
)

// Dlc returns a precondition that Crostini's artifact such as the
// guest kernel is available before the test runs.
//
//	testing.AddTest(&testing.Test{
//		...
//		Pre:   vm.Dlc(),
//	})
//
// Later, in the main test function, the VM artifacts are available via
// PreData.
func Dlc() testing.Precondition { return dlcPre }

var dlcPre = &dlcPreImpl{
	name:    "vm_dlc",
	timeout: 15 * time.Second,
}

// Implementation of vm_dlc precondition.
type dlcPreImpl struct {
	name    string        // Name of this precondition.
	timeout time.Duration // Timeout for completing the precondition.
}

// Interface methods for a testing.Precondition.
func (p *dlcPreImpl) String() string         { return p.name }
func (p *dlcPreImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by tast before each test is run. We use this method
// to initialize the precondition data, or return early if the precondition
// is already active.
func (p *dlcPreImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if err := dlc.Install(ctx, TerminaDlcID, "" /*omahaURL*/); err != nil {
		s.Fatal("Failed to install DLC: ", err)
	}

	return PreData{
		Kernel: filepath.Join(TerminaDlcDir, "vm_kernel"),
		Rootfs: filepath.Join(TerminaDlcDir, "vm_rootfs.img"),
	}
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructo
func (p *dlcPreImpl) Close(ctx context.Context, s *testing.PreState) {
	if err := dlc.Purge(ctx, TerminaDlcID); err != nil {
		s.Fatal("Purge failed: ", err)
	}
}
