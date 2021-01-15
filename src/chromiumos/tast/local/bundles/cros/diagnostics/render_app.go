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
		Func: RenderApp,
		Desc: "Diagnostics app launches and renders components",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// RenderApp verifies launching an app from the launcher.
func RenderApp(ctx context.Context, s *testing.State) {
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

	// Verify cpu chart is drawn
	if _, err := dxRootnode.DescendantWithTimeout(
		ctx, diagnosticsapp.DxCPUChart, 20*time.Second); err != nil {
		s.Fatal("Failed to find CPU chart: ", err)
	}

	// Verify session log button is rendered
	if _, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxLogButton, 20*time.Second); err != nil {
		s.Fatal("Failed to render Diagnostics app: ", err)
	}

	// Verify test routine buttons are rendered
	buttons, err := dxRootnode.Descendants(ctx, diagnosticsapp.DxActionButtons)
	if err != nil {
		s.Fatal("Failed to find test routine buttons")
	}
	defer buttons.Release(ctx)

	if len(buttons) != 3 {
		s.Fatal("Could not find all test routine buttons")
	}

}
