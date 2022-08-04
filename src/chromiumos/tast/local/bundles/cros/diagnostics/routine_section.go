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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutineSection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Diagnostics app routines run successfully",
		Contacts: []string{
			"ashleydp@google.com",
			"zentaro@google.com",
			"menghuan@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "diagnosticsPrep",
	})
}

// RoutineSection verifies routine section functionality.
func RoutineSection(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*chrome.TestConn)

	// Find the first routine action button
	ui := uiauto.New(tconn)
	cpuButton := diagnosticsapp.DxCPUTestButton.Ancestor(diagnosticsapp.DxRootNode).First()
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
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(diagnosticsapp.DxProgressBadge.Ancestor(diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Could not verify test routine has started: ", err)
	}

	if err := ui.WithTimeout(5 * time.Minute).WaitUntilExists(diagnosticsapp.DxPassedBadge.Ancestor(diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Could not verify successful run of at least one CPU routine: ", err)
	}

	// Cancel the test after first routine succeeds
	cancelBtn := diagnosticsapp.DxCancelTestButton.Ancestor(diagnosticsapp.DxRootNode)
	if err := uiauto.Combine("click Cancel",
		ui.WithTimeout(20*time.Second).WaitUntilExists(cancelBtn),
		ui.MakeVisible(cancelBtn),
		ui.WithPollOpts(pollOpts).LeftClick(cancelBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to click cancel button: ", err)
	}

	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsapp.DxCancelledBadge.Ancestor(diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Could not verify cancellation of routine: ", err)
	}
}
