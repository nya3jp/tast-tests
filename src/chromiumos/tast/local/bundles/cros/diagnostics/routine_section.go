// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
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
		s.Fatal("Failed to find the cpu test routine button: ", err)
	}
	defer cpuButton.Release(ctx)

	// If needed, scroll down to make the cpu button visible
	if err := cpuButton.MakeVisible(ctx); err != nil {
		s.Fatal("Failed to locate cpu button within the screen bounds: ", err)
	}

	// Test on power routine
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := cpuButton.StableLeftClick(ctx, &pollOpts); err != nil {
		s.Fatal("Could not click the CPU test button: ", err)
	}
	s.Log("Starting CPU test routine")

	// TODO(crbug/1174688): Detect this through a routine process instead of relying on the UI.
	if err := dxRootnode.WaitUntilDescendantExists(ctx, diagnosticsapp.DxProgressBadge, time.Minute); err != nil {
		s.Fatal("Could not verify test routine has started: ", err)
	}

	if err := dxRootnode.WaitUntilDescendantExists(ctx, diagnosticsapp.DxPassedBadge, 5*time.Minute); err != nil {
		s.Fatal("Could not verify successful run of at least one CPU routine: ", err)
	}

	cancelBtn, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxCancelTestButton, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find a cancel button: ", err)
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
