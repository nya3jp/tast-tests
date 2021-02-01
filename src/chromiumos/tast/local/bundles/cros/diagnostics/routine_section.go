// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RoutineSection,
		Desc: "Diagnostics app routines run successfully",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// RoutineSection verifies routine section functionality.
func RoutineSection(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("DiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}
	defer dxRootnode.Release(ctx)

	// Find the first routine action button
	cpuButton, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxCPUTestButton, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find the cpu test routine button")
	}
	defer cpuButton.Release(ctx)

	// Test on power routine
	if err := cpuButton.LeftClick(ctx); err != nil {
		s.Fatal("Could not click the CPU test button: ", err)
	}
	s.Log("Started CPU test routine")

	reportBtn, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxViewReportButton, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find the view report button")
	}
	defer reportBtn.Release(ctx)

	// Expand the view report button to see progress
	// TODO(joonbug): Adapt to report page being open by default when UX change is finalized
	if err := reportBtn.LeftClick(ctx); err != nil {
		s.Fatal("Could not expand the test report view: ", err)
	}

	if err := dxRootnode.WaitUntilDescendantExists(ctx, diagnosticsapp.DxSuccessBadge, 5*time.Minute); err != nil {
		s.Fatal("Could not verify successful run of at least one CPU routine: ", err)
	}

	cancelBtn, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxCancelTestButton, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find a cancel button")
	}
	defer cancelBtn.Release(ctx)

	// Cancel the test after first routine succeeds
	if err := cancelBtn.LeftClick(ctx); err != nil {
		s.Fatal("Could not click the cancel button: ", err)
	}

	if err := dxRootnode.WaitUntilDescendantExists(ctx, diagnosticsapp.DxCancelledBadge, 20*time.Second); err != nil {
		s.Fatal("Could not verify cancellation of routine: ", err)
	}
}
