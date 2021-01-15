// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RoutineSection,
		Desc: "Diagnostics app  and renders components",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// RoutineSection verifies launching an app from the launcher.
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

	// Wait until at least one action button is rendered
	if _, err := dxRootnode.DescendantWithTimeout(
		ctx, diagnosticsapp.DxActionButtons, 20*time.Second); err != nil {
		s.Fatal("Action buttons never found: ", err)
	}

	buttons, err := dxRootnode.Descendants(ctx, diagnosticsapp.DxActionButtons)
	if err != nil {
		s.Fatal("Failed to find test routine buttons")
	}
	defer buttons.Release(ctx)

	// Test on power routine
	if err := buttons[0].LeftClick(ctx); err != nil {
		s.Fatal("Could not click routine test button: ", err)
	}
	s.Log("Started power routine")

	// TODO(joonbug): Check for cancellation when implemented

	_, err = dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxSuccessBadge, 3*time.Minute)
	if err != nil {
		s.Fatal("Could not verify successful run of power routine: ", err)
	}

}
