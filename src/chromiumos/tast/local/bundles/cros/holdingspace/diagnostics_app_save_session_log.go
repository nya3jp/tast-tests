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

	// Reset the holding space and `MarkTimeOfFirstAdd` to make the `HoldingSpaceTrayIcon`
	// show.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceOptions{MarkTimeOfFirstAdd: true}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	// Check if file already exist
	const testFile = "session_log.txt"
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get users Download path: ", err)
	}
	filePath := filepath.Join(downloadsPath, testFile)
	if _, err := os.Stat(filePath); err == nil {
		// s.Fatal("File %q already exists: %s", filePath, err)
		os.Remove(filePath)
	}

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open diagnostics app: ", err)
	}

	// Find Save test details button. If needed, scroll down to make the Save test details button visible.
	// Click Save test details button.
	ui := uiauto.New(tconn)
	saveTestDetailsButton := diagnosticsapp.DxLogButton.Ancestor(dxRootnode)
	if err := uiauto.Combine("find and click Save test details button",
		ui.WithTimeout(20*time.Second).WaitUntilExists(saveTestDetailsButton),
		ui.MakeVisible(saveTestDetailsButton),
		ui.LeftClick(saveTestDetailsButton),
	)(ctx); err != nil {
		s.Fatal("Could not click the Save test details button: ", err)
	}

	// Click Save button
	saveButton := nodewith.Name("Save").Role(role.Button)
	if err := uiauto.Combine("click Save",
		ui.WithTimeout(10*time.Second).WaitUntilExists(saveButton),
		ui.LeftClick(saveButton),
	)(ctx); err != nil {
		s.Fatal("Could not click Save button")
	}

	// Remove created session log file.
	defer os.Remove(filePath)

	// Verify file is showing in holding space
	uia := uiauto.New(tconn)
	if err := uiauto.Combine("Click on holding space and find session_log.txt file",
		uia.LeftClick(holdingspace.FindTray()),
		uia.WaitUntilExists(holdingspace.FindDownloadChip().Name(testFile)),
	)(ctx); err != nil {
		s.Fatalf("Failed to find %q in holding space: %s", testFile, err)
	}

}
