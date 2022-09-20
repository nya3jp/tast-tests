// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/printpreview"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type printToPdfParams struct {
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintToPDF,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies print to pdf file appears in holding space",
		Contacts: []string{
			"dmblack@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "ash",
			Val: printToPdfParams{
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "lacros",
			Val: printToPdfParams{
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// PrintToPDF verifies that after printing to pdf, the file is displayed in the Downloads
// section of Holding Space.
func PrintToPDF(ctx context.Context, s *testing.State) {
	params := s.Param().(printToPdfParams)
	bt := params.browserType

	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)
	defer closeBrowser(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	ui := uiauto.New(tconn)

	// Open a new Chrome window with an empty browser tab for us to to test printing to
	// pdf with.
	conn, err := br.NewConn(ctx, "")
	defer conn.Close()

	// Wait for the tab to load.
	const expectedTabTitle = "about:blank"
	_, err = ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
		return (bt == browser.TypeAsh && w.WindowType == ash.WindowTypeBrowser) ||
			(bt == browser.TypeLacros && w.WindowType == ash.WindowTypeLacros) &&
				w.IsActive &&
				regexp.MustCompile(expectedTabTitle).MatchString(w.Title)
	})
	if err != nil {
		s.Fatalf("Failed to find active window with title having %q as a substring: %v",
			expectedTabTitle, err)
	}

	// Open print preview using the Ctrl+P shortcut.
	printPreviewSaveButton := nodewith.Name("Save").Role(role.Button)
	kb, err := input.Keyboard(ctx)
	if err := uiauto.Combine("Open Print Preview with shortcut Ctrl+P",
		kb.AccelAction("Ctrl+P"),
		ui.WithTimeout(time.Minute).WaitUntilExists(printPreviewSaveButton),
		printpreview.WaitForPrintPreview(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to open Print Preview: ", err)
	}

	// Select "Save as PDF" as the printer option.
	const printerName = "Save as PDF"
	if err := printpreview.SelectPrinter(ctx, tconn, printerName); err != nil {
		s.Fatal("Failed to select Save as PDF as a printer: ", err)
	}

	if err := printpreview.WaitForPrintPreview(tconn)(ctx); err != nil {
		s.Fatal("Failed to wait for Print Preview: ", err)
	}

	// Hide all notifications to prevent them from covering the "Save" button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
	}

	// Click the "Save" button.
	if err = ui.LeftClick(printPreviewSaveButton)(ctx); err != nil {
		s.Fatal("Failed to click Save: ", err)
	}

	// Download file window will popup, enter a filename for the PDF and click "Save".
	textField := nodewith.Name("File name").Role(role.TextField)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(textField)(ctx); err != nil {
		s.Fatal("Failed to find Downloads text field: ", err)
	}
	const fileName = "download"
	if err := kb.Type(ctx, fileName); err != nil {
		s.Fatal("Failed to type Download file name: ", err)
	}

	downloadSaveButton := nodewith.Name("Save").
		Role(role.Button).
		Ancestor(nodewith.Name("Save file as").Role(role.Window))
	if err = ui.LeftClick(downloadSaveButton)(ctx); err != nil {
		s.Fatal("Failed to click Download Save button: ", err)
	}

	// .pdf is automatically appended to the filename when saving.
	const fullFileName = fileName + ".pdf"

	// Store the file location for cleanup later.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	downloadLocation := filepath.Join(downloadsPath, fullFileName)
	defer os.Remove(downloadLocation)

	if err := uiauto.Combine("open bubble and confirm pdf is saved",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),
		ui.WaitUntilExists(holdingspace.FindChip().Name(fullFileName)),
	)(ctx); err != nil {
		s.Fatal("Failed to open bubble and confirm pdf is saved: ", err)
	}
}
