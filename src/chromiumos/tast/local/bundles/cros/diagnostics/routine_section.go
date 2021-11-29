// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutineSection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Diagnostics app routines run successfully",
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

	// Find the first routine action button
	ui := uiauto.New(tconn)
	cpuButton := diagnosticsapp.DxCPUTestButton.Ancestor(dxRootnode).First()
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(cpuButton)(ctx); err != nil {
		s.Fatal("Failed to find the cpu test routine button: ", err)
	}

	// If needed, scroll down to make the cpu button visible
	if err := ui.MakeVisible(cpuButton)(ctx); err != nil {
		s.Fatal("Failed to locate cpu button within the screen bounds: ", err)
	}

	// Test on power routine
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := ui.WithPollOpts(pollOpts).LeftClick(cpuButton)(ctx); err != nil {
		s.Fatal("Could not click the CPU test button: ", err)
	}
	s.Log("Starting CPU test routine")

	// TODO(crbug/1174688): Detect this through a routine process instead of relying on the UI.
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(diagnosticsapp.DxProgressBadge.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Could not verify test routine has started: ", err)
	}

	if err := ui.WithTimeout(5 * time.Minute).WaitUntilExists(diagnosticsapp.DxPassedBadge.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Could not verify successful run of at least one CPU routine: ", err)
	}

	cancelBtn := diagnosticsapp.DxCancelTestButton.Ancestor(dxRootnode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(cancelBtn)(ctx); err != nil {
		s.Fatal("Failed to find a cancel button: ", err)
	}

	// Cancel the test after first routine succeeds
	if err := ui.LeftClick(cancelBtn)(ctx); err != nil {
		s.Fatal("Could not click the cancel button: ", err)
	}

	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsapp.DxCancelledBadge.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Could not verify cancellation of routine: ", err)
	}
}
