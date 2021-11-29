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
		Func:         RenderApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Diagnostics app launches and renders components",
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

	// Verify cpu chart is drawn
	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsapp.DxCPUChart.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Failed to find CPU chart: ", err)
	}

	// Verify session log button is rendered
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsapp.DxLogButton.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Failed to render log button: ", err)
	}

	// Verify test routine button is rendered
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsapp.DxCPUTestButton.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Failed to find cpu routine button: ", err)
	}

	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsapp.DxMemoryTestButton.Ancestor(dxRootnode).First())(ctx); err != nil {
		s.Fatal("Failed to find memory routine buttons: ", err)
	}
}
