// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/faillog"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeSpaceConstrained,
		Desc:         "Test resizing disk of Crostini from the Settings with constrained host disk space",
		Contacts:     []string{"nverne@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"keepState"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				Name:              "stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedByDlcBuster(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "unstable",
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Pre:               crostini.StartedByDlcBuster(),
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func ResizeSpaceConstrained(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	keyboard := pre.Keyboard

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(cleanupCtx, pre)

	// Open the Linux settings.
	st, err := settings.OpenLinuxSettings(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open Linux Settings: ", err)
	}

	defer st.Close(cleanupCtx)
	defer func() { faillog.DumpUITreeAndScreenshot(cleanupCtx, tconn, "resize_space_constrained", err) }()

	const GB uint64 = 1 << 30
	targetDiskSizeBytes := []uint64{20 * GB, 10 * GB, 5 * GB, 1 * GB, 5 * GB, 10 * GB}
	currSizeStr, err := st.GetDiskSize(ctx)
	if err != nil {
		s.Fatal("Failed to get current disk size: ", err)
	}
	currSizeBytes, err := settings.ParseDiskSize(currSizeStr)
	if err != nil {
		s.Fatalf("Failed to parse disk size string %s: %v", currSizeStr, err)
	}
	const fillPath = "/home/user/"
	for _, tBytes := range targetDiskSizeBytes {
		freeSpace, err := disk.FreeSpace(fillPath)
		if err != nil {
			s.Fatalf("Failed to read free space in %s: %v", fillPath, err)
		}
		if freeSpace < tBytes {
			s.Logf("Not enough free space to run test. Have %v, need %v", freeSpace, tBytes)
			continue
		}
		fillFile, err := disk.FillUntil(fillPath, tBytes)
		if err != nil {
			s.Fatal("Failed to fill disk space: ", err)
		}
		s.Logf("Resizing from %v to %v", currSizeBytes, tBytes)
		if err = st.ResizeDisk(ctx, keyboard, tBytes, currSizeBytes < tBytes); err != nil {
			s.Fatalf("Failed to resize disk from %v to %v: %v", currSizeBytes, tBytes, err)
		}
		currSizeBytes = tBytes
		if err = os.Remove(fillFile); err != nil {
			s.Fatalf("Failed to remove fill file %s: %v", fillFile, err)
		}
	}

}
