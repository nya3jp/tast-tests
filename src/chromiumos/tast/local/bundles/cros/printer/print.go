// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/printmanagementapp"
	"chromiumos/tast/local/chrome/ui/printpreview"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Print,
		Desc:     "Tests that a virtual USB printer can be saved and printed to",
		Contacts: []string{"gavinwill@google.com", "cros-peripherals@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
	})
}

func Print(ctx context.Context, s *testing.State) {
	const (
		attributes    = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
		descriptors   = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		settingsLabel = "Printers"
		settingsPage  = "osPrinting"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	s.Log("Installing printer")
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	defer func() {
		if err := usbprinter.RemoveModules(cleanupCtx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}()

	printer, _, err := usbprinter.StartIPPUSB(ctx, devInfo, descriptors, attributes, "" /*record*/)
	if err != nil {
		s.Fatal("Failed to start IPP-over-USB printer: ", err)
	}
	defer func() {
		printer.Kill()
		printer.Wait()
	}()

	// Open OS Settings and navigate to the Printing page.
	ui := uiauto.New(tconn)
	entryFinder := nodewith.Name(settingsLabel).Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, settingsPage, ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	const printerName = "DavieV Virtual USB Printer (USB)"
	const printerLabel = "DavieV Virtual USB Printer (USB) DavieV Virtual USB Printer (USB)"
	savePrinterButton := nodewith.ClassName("save-printer-button").NameContaining(printerName).Ancestor(ossettings.WindowFinder)
	editPrinterButton := nodewith.ClassName("icon-more-vert").Ancestor(ossettings.WindowFinder)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	if err := uiauto.Combine("click Settings Printer entry, save printer, and open Print Preview with shortcut Ctrl+P",
		ui.LeftClick(entryFinder),
		ui.LeftClick(savePrinterButton),
		ui.WithTimeout(time.Minute).WaitUntilExists(editPrinterButton),
		kb.AccelAction("Ctrl+P"),
		printpreview.WaitForPrintPreview(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to save virtual USB printer and open Print Preview: ", err)
	}

	// Select printer and click Print button.
	s.Log("Selecting printer")
	if err := printpreview.SelectPrinter(ctx, tconn, printerLabel); err != nil {
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
