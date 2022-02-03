// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"chromiumos/tast/testing"
)

// TargetArch returns the name of the VM architecture that should be used
func TargetArch() string {
	if runtime.GOARCH == "arm64" {
		// For now, we ship the same VM image to arm64 as for arm devices
		return "arm"
	}
	return runtime.GOARCH
}

// ArtifactData returns the name of the data file that must be specified
// for tests using the Artifact() precondition.
func ArtifactData() string {
	return fmt.Sprintf("crostini_vm_%s.zip", TargetArch())
}

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(vm.PreData)
//		...
//	}
type PreData struct {
	Kernel string // Path to the guest kernel.
	Rootfs string // Path to the guest rootfs image.
}

// Artifact returns a precondition that Crostini's artifact such as the
// guest kernel is available before the test runs.
//
// When adding a test with this precondition, the return value of
// ArtifactData() must be included in Data:
//
//	testing.AddTest(&testing.Test{
//		...
//		Data:  []string{vm.ArtifactData()},
//		Pre:   vm.Artifact(),
//	})
//
// Later, in the main test function, the VM artifacts are available via
// PreData.
func Artifact() testing.Precondition { return artifactPre }

var artifactPre = &preImpl{
	name: "vm_artifact",
	// If a previous test left a lot of outstanding I/O, we may eat a lot of
	// time unmounting and setting up a new image.
	timeout: 40 * time.Second,
}

// Implementation of VM precondition.
type preImpl struct {
	name    string        // Name of this precondition.
	timeout time.Duration // Timeout for completing the precondition.
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by tast before each test is run. We use this method
// to initialize the precondition data, or return early if the precondition
// is already active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	vmPath := s.DataPath(ArtifactData())

	image, err := ExtractTermina(ctx, vmPath)
	if err != nil {
		s.Fatal("Failed to extract termina: ", err)
	}

	if err := MountComponent(ctx, image); err != nil {
		s.Fatal("Failed to mount termina image: ", err)
	}

	return PreData{
		Kernel: filepath.Join(TerminaMountDir, "vm_kernel"),
		Rootfs: filepath.Join(TerminaMountDir, "vm_rootfs.img"),
	}
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructo
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	UnmountComponent(ctx)
	DeleteImages()
}
