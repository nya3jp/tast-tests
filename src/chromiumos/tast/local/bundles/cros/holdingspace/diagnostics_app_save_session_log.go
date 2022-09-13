// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiagnosticsAppSaveSessionLog,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that session logs saved from Diagnostics App appear in Holding Space",
		Contacts: []string{
			"angelsan@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// DiagnosticsAppSaveSessionLog tests the functionality of files existing in Holding Space by
// saving a session log from the Diagnostics app.
func DiagnosticsAppSaveSessionLog(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Reset the holding space.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceOptions{}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	// Ensure session log does not exist.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get users Download path: ", err)
	}
	const filename = "session_log.txt"
	filePath := filepath.Join(downloadsPath, filename)
	if _, err := os.Stat(filePath); err == nil {
		os.Remove(filePath)
	}
	defer os.Remove(filePath)

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open diagnostics app: ", err)
	}

	ui := uiauto.New(tconn)
	uiauto.Combine("Save session log and verify file appears in holding space",
		ui.LeftClick(diagnosticsapp.DxLogButton.Ancestor(dxRootnode)),
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.LeftClick(holdingspace.FindTray()),
		ui.WaitUntilExists(holdingspace.FindDownloadChip().Name(filename)),
	)(ctx)
}
