// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Print,
		Desc:     "Tests that a USB printer can be saved and printed to",
		Contacts: []string{"gavinwill@google.com", "cros-peripherals@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func Print(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx) // Close our own chrome instance.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Open OS Settings and navigate to the Printing page
	ui := uiauto.New(tconn)
	entryFinder := nodewith.Name("Printers").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrinting", ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		s.Fatal("Failed to click entry: ", err)
	}
}
