// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

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
		LacrosStatus: testing.LacrosVariantExists,
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

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "print_to_pdf")

	// Open a new Chrome window with an empty browser tab for us to to test printing to
	// pdf with.
	conn, err := br.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create a new Chrome window: ", err)
	}
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

	const fileName = "download"
	// .pdf is automatically appended to the filename when saving.
	const fullFileName = fileName + ".pdf"
	// Defer cleanup of the downloaded PDF file.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	downloadLocation := filepath.Join(downloadsPath, fullFileName)
	defer os.Remove(downloadLocation)

	ui := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)
	if err := uiauto.Combine("Save as PDF and verify presence in holding space",
		// Open print preview using the Ctrl+P shortcut.
		kb.AccelAction("Ctrl+P"),
		printpreview.WaitForPrintPreview(tconn),

		// Select "Save as PDF" as the printer option.
		func(ctx context.Context) error {
			return printpreview.SelectPrinter(ctx, tconn, "Save as PDF")
		},
		printpreview.WaitForPrintPreview(tconn),

		// Click the "Save" button.
		ui.LeftClick(nodewith.Name("Save").Role(role.Button)),

		// Download file window will popup, enter a filename for the PDF and click "Save".
		ui.EnsureFocused(nodewith.Name("File name").Role(role.TextField)),
		kb.TypeAction(fileName),
		ui.LeftClick(nodewith.Name("Save").
			Role(role.Button).
			Ancestor(nodewith.Name("Save file as").Role(role.Window))),

		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),
		ui.WaitUntilExists(holdingspace.FindChip().Name(fullFileName)),
	)(ctx); err != nil {
		s.Fatal("Failed to save as PDF and verify presence in holding space: ", err)
	}
}
