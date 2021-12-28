// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmwareupdate

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	fwudpapp "chromiumos/tast/local/chrome/uiauto/firmwareupdateapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RenderApp,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that application launches",
		Contacts: []string{
			"ashleydp@google.com",         // Test author
			"cros-peripherals@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// RenderApp attempts to open the Firmware Update application.
func RenderApp(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("FirmwareUpdaterApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Create cleanup context to reserve time for faillog dump.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Launches Firmware Update and tests that app opens.
	if _, err := fwudpapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Firmware Updater app: ", err)
	}

	ui := uiauto.New(tconn)

	// Verify application header is rendered.
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(fwudpapp.AppHeader)(ctx); err != nil {
		s.Fatal("Failed to locate app header: ", err)
	}
}
