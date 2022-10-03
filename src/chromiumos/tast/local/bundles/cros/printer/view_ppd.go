// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/printer/uinames"
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
func createPrinter(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, ui *uiauto.Context, printerName, printerManufacturer, printerModel string) {
	s.Log("Creating printer: ", printerName, ", ", printerManufacturer, ", ", printerModel)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	// Open OS Settings and navigate to the Printing page.
	entryFinder := nodewith.Name(uinames.PrintersName).Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, uinames.SettingsPageName, ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	// Open add printer dialog
	addPrinterButton := nodewith.Role(role.Button).Name(uinames.AddPrinterName).Ancestor(ossettings.WindowFinder)
	if err := uiauto.Combine("click add printer button",
		ui.WithTimeout(10*time.Second).WaitUntilExists(entryFinder),
		ui.MakeVisible(entryFinder),
		ui.LeftClick(entryFinder),
		ui.WithTimeout(10*time.Second).WaitUntilExists(addPrinterButton),
		ui.LeftClick(addPrinterButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click add printer button: ", err)
	}

	// Input basic parameters
	editDialog := nodewith.Role(role.Dialog).Name(uinames.AddPrinterManuallyName).Ancestor(ossettings.WindowFinder)
	nameField := nodewith.Role(role.TextField).Name(uinames.NameName).Ancestor(editDialog)
	addressField := nodewith.Role(role.TextField).Name(uinames.AddressName).Ancestor(editDialog)
	protocolField := nodewith.Name(uinames.ProtocolName).Ancestor(editDialog)
	appSocketItem := nodewith.NameContaining(uinames.AppSocketName).Ancestor(editDialog)
	addButton := nodewith.Role(role.Button).Name(uinames.AddName).Ancestor(editDialog)
	if err := uiauto.Combine("input basic printer parameters",
		ui.WithTimeout(10*time.Second).WaitUntilExists(nameField),
		ui.LeftClick(nameField),
		kb.TypeAction(printerName),
		ui.LeftClick(addressField),
		kb.TypeAction("localhost"),
		ui.LeftClick(protocolField),
		ui.LeftClick(appSocketItem),
		ui.LeftClick(addButton),
	)(ctx); err != nil {
		s.Fatal("Failed to input basic printer parameters: ", err)
	}

	// Input advanced parameters
	advancedConfigDialog := nodewith.Role(role.Dialog).Name(uinames.AdvancedConfigName).Ancestor(ossettings.WindowFinder)
	manufacturerButton := nodewith.Role(role.TextField).Name(uinames.ManufacturerName).Ancestor(advancedConfigDialog)
	manufacturerSelection := nodewith.Role(role.Button).Name(printerManufacturer).Ancestor(advancedConfigDialog)
	modelButton := nodewith.Role(role.TextField).Name(uinames.ModelName).Ancestor(advancedConfigDialog)
	modelSelection := nodewith.Role(role.Button).Name(printerModel).Ancestor(advancedConfigDialog)
	advancedConfigAddButton := nodewith.Role(role.Button).Name(uinames.AddName).Ancestor(advancedConfigDialog)
	printerButton := nodewith.Role(role.Button).Name(printerName).Ancestor(ossettings.WindowFinder)
	if err := uiauto.Combine("input advanced printer parameters",
		ui.WithTimeout(10*time.Second).WaitUntilExists(manufacturerButton),
		ui.LeftClick(manufacturerButton),
		kb.TypeAction(printerManufacturer),
		ui.LeftClick(manufacturerSelection),
		ui.LeftClick(modelButton),
		kb.TypeAction(printerModel),
		ui.LeftClick(modelSelection),
		ui.LeftClick(advancedConfigAddButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(printerButton),
	)(ctx); err != nil {
		s.Fatal("Failed to input advanced printer parameters: ", err)
	}
}

// checkPpd chooses the edit button for the printer specified by the caller and
// clicks on the View PPD button.  It then checks that the appropriate web page
// opens. If containsEula is true, this will additionally check to make sure the
// EULA link is present and works.
func checkPpd(ctx context.Context, s *testing.State, ui *uiauto.Context,
	printerName string, containsEula bool) {
	// Edit the printer and select the View PPD button.
	printerButton := nodewith.Role(role.Button).Name(printerName).Ancestor(ossettings.WindowFinder)
	editText := nodewith.Role(role.StaticText).Name(uinames.EditName).Ancestor(ossettings.WindowFinder)
	editPrinterDialog := nodewith.Role(role.Dialog).Name(uinames.EditPrinterName).Ancestor(ossettings.WindowFinder)
	viewPpdButton := nodewith.Role(role.Button).Name(uinames.ViewPpdName).Ancestor(editPrinterDialog)
	if err := uiauto.Combine("click Edit Printer, view PPD button",
		ui.WithTimeout(10*time.Second).WaitUntilExists(printerButton),
		ui.LeftClick(printerButton),
		ui.LeftClick(editText),
		ui.WithTimeout(10*time.Second).WaitUntilExists(viewPpdButton),
		ui.MakeVisible(viewPpdButton),
		ui.LeftClick(viewPpdButton),
	)(ctx); err != nil {
		s.Fatal("Failed to view PPD: ", err)
	}

	// Make sure the tab with the PPD results gets displayed and that it does not
	// contain the error message.
	webView := nodewith.Role(role.RootWebArea).Name(printerName)
	ppdTab := nodewith.Role(role.Heading).Name(uinames.PpdHeaderName + printerName).Ancestor(webView)
	// All PPDs will beging with this line.  Make sure this shows up.
	ppdText := nodewith.Role(role.StaticText).NameContaining(uinames.PpdStartTextName).Ancestor(webView)
	// This error message should not show up.
	errorMsg := nodewith.Role(role.StaticText).NameContaining(uinames.PpdRetrieveErrorName)
	if err := uiauto.Combine("wait for PPD results",
		ui.WithTimeout(time.Minute).WaitUntilExists(ppdTab),
		ui.WithTimeout(time.Minute).WaitUntilExists(ppdText),
		ui.Gone(errorMsg),
	)(ctx); err != nil {
		s.Fatal("Failed to display PPD results: ", err)
	}
	if containsEula {
		eulaLink := nodewith.Role(role.Link).Name(uinames.EulaName).Ancestor(webView)
		licensePage := nodewith.Role(role.RootWebArea).Name(uinames.CreditsName)
		if err := uiauto.Combine("check for EULA",
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
