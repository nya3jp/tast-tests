// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ViewPPD,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that a user can view the PPD for an installed printer",
		Contacts:     []string{"nmuggli@google.com", "cros-peripherals@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"chrome", "cros_internal", "cups"},
		Fixture:      "chromeLoggedIn",
	})
}

// createPrinter creates a new printer using appsocket protocol so we don't have
// to have an actual printer available.  Printer name, manufacturer, and model
// are all provided by the caller.
func createPrinter(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	tconn *chrome.TestConn, ui *uiauto.Context,
	printerName, printerManufacturer, printerModel string) {
	s.Log("Creating printer: ", printerName, ", ", printerManufacturer, ", ", printerModel)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	// Open OS Settings and navigate to the Printing page.
	entryFinder := nodewith.Name("Printers").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrinting", ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	addPrinterButton := nodewith.Role(role.Button).Name("Add printer").Ancestor(ossettings.WindowFinder)
	nameField := nodewith.Role(role.TextField).Name("Name").Ancestor(ossettings.WindowFinder)
	addressField := nodewith.Role(role.TextField).Name("Address").Ancestor(ossettings.WindowFinder)
	protocolField := nodewith.Name("Protocol").Ancestor(ossettings.WindowFinder)
	appSocketItem := nodewith.NameContaining("AppSocket").Ancestor(ossettings.WindowFinder)
	addButton := nodewith.Role(role.Button).Name("Add").Ancestor(ossettings.WindowFinder)
	manufacturerButton := nodewith.Role(role.TextField).Name("Manufacturer").Ancestor(ossettings.WindowFinder)
	manufacturerSelection := nodewith.Role(role.Button).Name(printerManufacturer).Ancestor(ossettings.WindowFinder)
	modelButton := nodewith.Role(role.TextField).Name("Model").Ancestor(ossettings.WindowFinder)
	modelSelection := nodewith.Role(role.Button).Name(printerModel).Ancestor(ossettings.WindowFinder)
	printerButton := nodewith.Role(role.Button).Name(printerName).Ancestor(ossettings.WindowFinder)
	if err := uiauto.Combine("create printer",
		ui.LeftClick(entryFinder),
		ui.WithTimeout(time.Minute).WaitUntilExists(addPrinterButton),
		ui.LeftClick(addPrinterButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(nameField),
		ui.LeftClick(nameField),
		kb.TypeAction(printerName),
		ui.LeftClick(addressField),
		kb.TypeAction("localhost"),
		ui.LeftClick(protocolField),
		ui.LeftClick(appSocketItem),
		ui.LeftClick(addButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(manufacturerButton),
		ui.LeftClick(manufacturerButton),
		kb.TypeAction(printerManufacturer),
		ui.LeftClick(manufacturerSelection),
		ui.LeftClick(modelButton),
		kb.TypeAction(printerModel),
		ui.LeftClick(modelSelection),
		ui.LeftClick(addButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(printerButton),
	)(ctx); err != nil {
		s.Fatal("Failed to create printer: ", err)
	}
}

// checkPpd chooses the edit button for the printer name specified by the caller
// and click on the View PPD button.  Check that the appropriate web page opens.
// If containsEula is true, this will additionally check to make sure the EULA
// link is present and works.
func checkPpd(ctx context.Context, s *testing.State, ui *uiauto.Context,
	printerName string, containsEula bool) {
	// Edit the printer and select the View PPD button.
	printerButton := nodewith.Role(role.Button).Name(printerName).Ancestor(ossettings.WindowFinder)
	editText := nodewith.Role(role.StaticText).Name("Edit").Ancestor(ossettings.WindowFinder)
	viewPpdButton := nodewith.ClassName("ppd-button").Role(role.Button).Ancestor(ossettings.WindowFinder)
	if err := uiauto.Combine("click Edit Printer, view PPD button",
		ui.WithTimeout(time.Minute).WaitUntilExists(printerButton),
		ui.LeftClick(printerButton),
		ui.LeftClick(editText),
		ui.WithTimeout(time.Minute).WaitUntilExists(viewPpdButton),
		ui.MakeVisible(viewPpdButton),
		ui.LeftClick(viewPpdButton),
	)(ctx); err != nil {
		s.Fatal("Failed to view PPD: ", err)
	}

	// Make sure the tab with the PPD results gets displayed and that it does not
	// contain the error message.
	webView := nodewith.Role(role.RootWebArea).Name(printerName)
	ppdTab := nodewith.Role(role.Heading).Name("PPD for " + printerName).Ancestor(webView)
	errorMsg := nodewith.Role(role.StaticText).NameContaining("Unable to retrieve PPD")
	if err := uiauto.Combine("wait for PPD results",
		ui.WithTimeout(time.Minute).WaitUntilExists(ppdTab),
		ui.Gone(errorMsg),
	)(ctx); err != nil {
		s.Fatal("Failed to display PPD results: ", err)
	}
	if containsEula {
		eulaLink := nodewith.Role(role.Link).Name("End User License Agreement").Ancestor(webView)
		licensePage := nodewith.Role(role.RootWebArea).Name("Credits")
		if err := uiauto.Combine("check for EULA",
			ui.WithTimeout(time.Minute).WaitUntilExists(eulaLink),
			ui.LeftClick(eulaLink),
			ui.WithTimeout(time.Minute).WaitUntilExists(licensePage),
		)(ctx); err != nil {
			s.Fatal("Failed to check for EULA: ", err)
		}
	}
}

func ViewPPD(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	ui := uiauto.New(tconn)

	// Test a printer that does not have the EULA
	printerName := "test-printer"
	createPrinter(ctx, s, cr, tconn, ui, printerName, "Brother", "Brother DCP-1200")
	checkPpd(ctx, s, ui, printerName, false)

	// Test with a printer that has an EULA
	printerName = "test-printer-with-eula"
	createPrinter(ctx, s, cr, tconn, ui, printerName, "Xerox", "Xerox B230")
	checkPpd(ctx, s, ui, printerName, true)
}
