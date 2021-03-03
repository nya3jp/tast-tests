// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Print,
		Desc:     "Tests that a USB printer can be saved and printed to",
		Contacts: []string{"gavinwill@google.com", "cros-peripherals@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
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
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx) // Close our own chrome instance.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Install virtual USB printer.
	s.Log("Installing printer")
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
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
	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		s.Fatal("Failed to click entry: ", err)
	}

	// Save the USB printer.
	savePrinterButton := nodewith.ClassName("save-printer-button").Ancestor(ossettings.WindowFinder)
	if err := ui.LeftClick(savePrinterButton)(ctx); err != nil {
		s.Fatal("Failed to click Save button: ", err)
	}
	editPrinterButton := nodewith.ClassName("icon-more-vert").Ancestor(ossettings.WindowFinder)
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(editPrinterButton)(ctx); err != nil {
		s.Fatal("Failed to find edit printer button: ", err)
	}
}
