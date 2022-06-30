// Copyright 2022 The ChromiumOS Authors.
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
		Desc:         "Checks that saving session log from Diagnostics App appears in Holding Space",
		Contacts: []string{
			"angelsan@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// DiagnosticsAppSaveSessionLog tests the functionality of files existing in Holding Space by
// saving a session log file from the Diagnostics app.
func DiagnosticsAppSaveSessionLog(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Reset the holding space.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceOptions{}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	// Check if file already exist.
	const filename = "session_log.txt"
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get users Download path: ", err)
	}
	filePath := filepath.Join(downloadsPath, filename)
	if _, err := os.Stat(filePath); err == nil {
		// File already exists. Remove it.
		os.Remove(filePath)
	}
	// Remove session_log.txt file that will later be created.
	defer os.Remove(filePath)

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open diagnostics app: ", err)
	}

	// Find Save test details button.
	// Click Save test details button.
	// Find Save button and click Save button.
	// Click holding space tray.
	// Verify file appears in holding space.
	ui := uiauto.New(tconn)
	saveTestDetailsButton := diagnosticsapp.DxLogButton.Ancestor(dxRootnode)
	if err := uiauto.Combine("Save session_log.txt file and verify file appears in holding space",
		ui.LeftClick(saveTestDetailsButton),
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		ui.LeftClick(holdingspace.FindTray()),
		ui.WaitUntilExists(holdingspace.FindDownloadChip().Name(filename)),
	)(ctx); err != nil {
		s.Fatal("Could not verify session_log.txt in holding space: ", err)
	}

}
