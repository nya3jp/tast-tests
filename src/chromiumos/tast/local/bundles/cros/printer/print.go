// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/printer/uitools"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/printmanagementapp"
	"chromiumos/tast/local/chrome/uiauto/printpreview"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Print,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that a virtual USB printer can be saved and printed to",
		Contacts:     []string{"gavinwill@google.com", "cros-peripherals@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Params: []testing.Param{
			{
				Val:     browser.TypeAsh,
				Fixture: "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
			},
			{
				Name:              "lacros",
				Val:               browser.TypeLacros,
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           "virtualUsbPrinterModulesLoadedWithLacros",
			},
		},
	})
}

func Print(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	// tconn is the ash TestConn.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect ash test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	s.Log("Installing printer")
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WaitUntilConfigured())
	if err != nil {
		s.Fatal("Failed to start IPP-over-USB printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(ctx)

	// Open OS Settings and navigate to the Printing page.
	ui := uiauto.New(tconn)
	entryFinder := uitools.PrintersFinder.Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, uitools.SettingsPageName, ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	const printerName = "DavieV Virtual USB Printer (USB)"
	savePrinterButton := nodewith.ClassName("save-printer-button").NameContaining(printerName).Ancestor(ossettings.WindowFinder)
	editPrinterButton := nodewith.ClassName("icon-more-vert").Ancestor(ossettings.WindowFinder)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	// Hide all notifications to prevent them from covering the printer entry.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
	}

	if err := uiauto.Combine("click Settings Printer entry, save printer",
		ui.LeftClick(entryFinder),
		ui.LeftClick(savePrinterButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(editPrinterButton),
	)(ctx); err != nil {
		s.Fatal("Failed to save virtual USB printer and open Print Preview: ", err)
	}

	// Create a browser (either ash or lacros, based on browser type).
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatal("Failed to launch browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, "chrome://version/")
	if err != nil {
		s.Fatal("Failed to connect to broswer: ", err)
	}
	defer conn.Close()

	if err := uiauto.Combine("open Print Preview with shortcut Ctrl+P",
		kb.AccelAction("Ctrl+P"),
		printpreview.WaitForPrintPreview(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to open Print Preview: ", err)
	}

	// Select printer and click Print button.
	s.Log("Selecting printer")
	if err := printpreview.SelectPrinter(ctx, tconn, printerName); err != nil {
		s.Fatal("Failed to select printer: ", err)
	}

	if err := printpreview.WaitForPrintPreview(tconn)(ctx); err != nil {
		s.Fatal("Failed to wait for Print Preview: ", err)
	}

	if err = printpreview.Print(ctx, tconn); err != nil {
		s.Fatal("Failed to print: ", err)
	}

	s.Log("Waiting for print job to complete")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return errors.New("Print job has not completed yet")
		}
		testing.ContextLog(ctx, "Print job has completed")
		return nil
	}, nil); err != nil {
		s.Fatal("Print job failed to complete: ", err)
	}

	// Launch Print Management app.
	printManagementApp, err := printmanagementapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Print Management app: ", err)
	}

	if err := uiauto.Combine("Verify print job",
		printManagementApp.VerifyHistoryLabel(),
		printManagementApp.VerifyPrintJob(),
	)(ctx); err != nil {
		s.Fatal("Failed to verify print job: ", err)
	}
}
