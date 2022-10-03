// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
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
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Data:         []string{"print_usb_ps.ppd.gz"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

func ViewPPD(ctx context.Context, s *testing.State) {
	const (
		settingsLabel   = "Printers"
		settingsPage    = "osPrinting"
		printerName     = "DavieV Virtual USB Printer (USB)"
		cupsPrinterName = "virtual-usb-printer"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	s.Log("Installing printer")
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}
	printer, err := usbprinter.Start(ctx,
		usbprinter.WithDescriptors("usb_printer.json"),
	)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop virtual printer: ", err)
		}
	}(cleanupCtx)

	usbURI := fmt.Sprintf("usb://%s/%s", printer.DevInfo.VID, printer.DevInfo.PID)
	if err := lp.CupsAddPrinter(ctx, cupsPrinterName, usbURI, s.DataPath("print_usb_ps.ppd.gz")); err != nil {
		s.Fatal("Failed to configure printer: ", err)
	}
	s.Log("Printer configured with name: ", cupsPrinterName)

	// Open OS Settings and navigate to the Printing page.
	ui := uiauto.New(tconn)
	entryFinder := nodewith.Name(settingsLabel).Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, settingsPage, ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	// Hide all notifications to prevent them from covering the printer entry.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	// Set up our printer.  Choose a random manufacturer (Brother) and the first
	// model in the list.
	printerButton := nodewith.Role(role.Button).NameContaining(printerName).Ancestor(ossettings.WindowFinder)
	manufacturerButton := nodewith.NameContaining("Manufacturer").Ancestor(ossettings.WindowFinder)
	if err := uiauto.Combine("setup printer",
		ui.LeftClick(entryFinder),
		ui.WithTimeout(time.Minute).WaitUntilExists(printerButton),
		ui.LeftClick(printerButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(manufacturerButton),
		ui.LeftClick(manufacturerButton),
		kb.TypeAction("Brother"),
		kb.TypeKeyAction(input.KEY_DOWN),
		kb.TypeKeyAction(input.KEY_ENTER),
		kb.TypeKeyAction(input.KEY_TAB),
		kb.TypeKeyAction(input.KEY_DOWN),
		kb.TypeKeyAction(input.KEY_ENTER),
		kb.TypeKeyAction(input.KEY_TAB),
		kb.TypeKeyAction(input.KEY_TAB),
		kb.TypeKeyAction(input.KEY_TAB),
		kb.TypeKeyAction(input.KEY_TAB),
		kb.TypeKeyAction(input.KEY_ENTER),
	)(ctx); err != nil {
		s.Fatal("Failed to setup printer: ", err)
	}

	// Now, edit the printer and select the View PPD button.
	viewPpdButton := nodewith.ClassName("ppd-button").Role(role.Button).Ancestor(ossettings.WindowFinder)
	if err := uiauto.Combine("click Edit Printer, view PPD button",
		ui.WithTimeout(time.Minute).WaitUntilExists(printerButton),
		ui.LeftClick(printerButton),
		kb.TypeKeyAction(input.KEY_DOWN),
		kb.TypeKeyAction(input.KEY_ENTER),
		ui.WithTimeout(time.Minute).WaitUntilExists(viewPpdButton),
		ui.MakeVisible(viewPpdButton),
		ui.LeftClick(viewPpdButton),
	)(ctx); err != nil {
		s.Fatal("Failed to view PPD: ", err)
	}

	// Make sure the tab with the PPD results gets displayed.
	ppdTab := nodewith.Name("PPD for " + printerName).First()
	if err := uiauto.Combine("wait for PPD results",
		ui.WithTimeout(time.Minute).WaitUntilExists(ppdTab),
	)(ctx); err != nil {
		s.Fatal("Failed to display PPD results: ", err)
	}
}
