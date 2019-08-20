// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SaveIPPUSBPrinter,
		Desc:         "Tests that automatic printer appears with a save button, and clicking it moves to a saved printer",
		Contacts:     []string{"baileyberro@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
	})
}

func SaveIPPUSBPrinter(ctx context.Context, s *testing.State) {
	const (
		descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		attributes  = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	)

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	connect, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to start a new tab: ", err)
	}
	err = connect.Navigate(ctx, "chrome://os-settings/cupsPrinters")
	if err != nil {
		s.Fatal("Failed to open OS Settings")
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	printerName, err := usbprinter.LoadPrinterName(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer Name from %v: %v", descriptors, err)
	}
	s.Log(printerName)

	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := usbprinter.RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}(ctx)

	printer, err := usbprinter.Start(ctx, devInfo, descriptors, attributes, "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	_ = printer

	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Failed to wait for printer to appear in Settings")
	}

	// Verify that |printer| appears in the list of nearby printers.
	var nearbyPrinters []string
	nearbyPrinters, err = getNearbyPrinterEntries(ctx, connect)
	if err != nil {
		s.Fatal("Failed to get list of printers: ", err)
	}
	if !containsPrinterName(nearbyPrinters, printerName) {
		s.Fatalf("%q not found in nearby printers, but is expected to be found", printerName)
	}

	// Press the save printer button
	err = clickSavePrinterButton(ctx, connect, printerName)

	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Failed to wait for printer to be saved")
	}

	// Verify that |printer| appears in the list of saved printers.
	var savedPrinters []string
	savedPrinters, err = getSavedPrinterEntries(ctx, connect)
	if err != nil {
		s.Fatal("Failed to get list of printers: ", err)
	}
	if !containsPrinterName(savedPrinters, printerName) {
		s.Fatalf("%q not found in saved printers, but is expected to be found", printerName)
	}

	// Test cleanup.
	printer.Kill()
	printer.Wait()
}

// getNearbyPrinterEntries returns an array of printerNames
func getNearbyPrinterEntries(ctx context.Context, kconn *chrome.Conn) ([]string, error) {
	var printerNames []string
	err := kconn.Eval(ctx, `
	(() => {
		const elems = document.querySelector("body > os-settings-ui")
			.shadowRoot.querySelector("#main")
			.shadowRoot.querySelector("os-settings-page")
			.shadowRoot.querySelector("#advancedPage > settings-section.expanded > os-settings-printing-page")
			.shadowRoot.querySelector("#pages > settings-subpage > settings-cups-printers")
			.shadowRoot.querySelector("settings-cups-nearby-printers")
			.shadowRoot.querySelector("settings-cups-printers-entry-list")
			.shadowRoot.querySelectorAll("settings-cups-printers-entry");
		console.log(Array.prototype.map.call(elems, (x) => x.printerEntry.printerInfo.printerName));
		return Array.prototype.map.call(elems, (x) => x.printerEntry.printerInfo.printerName);
	})()
`, &printerNames)
	return printerNames, err
}

func getSavedPrinterEntries(ctx context.Context, kconn *chrome.Conn) ([]string, error) {
	var printerNames []string
	err := kconn.Eval(ctx, `
	(() => {
		const elems = document.querySelector("body > os-settings-ui")
			.shadowRoot.querySelector("#main")
			.shadowRoot.querySelector("os-settings-page")
			.shadowRoot.querySelector("#advancedPage > settings-section.expanded > os-settings-printing-page")
			.shadowRoot.querySelector("#pages > settings-subpage > settings-cups-printers")
			.shadowRoot.querySelector("settings-cups-saved-printers")
			.shadowRoot.querySelector("settings-cups-printers-entry-list")
			.shadowRoot.querySelectorAll("settings-cups-printers-entry");
		console.log(Array.prototype.map.call(elems, (x) => x.printerEntry.printerInfo.printerName));
		return Array.prototype.map.call(elems, (x) => x.printerEntry.printerInfo.printerName);
	})()
`, &printerNames)
	return printerNames, err
}

func clickSavePrinterButton(ctx context.Context, kconn *chrome.Conn, printer string) error {
	err := kconn.Eval(ctx, fmt.Sprintf(`
	(() => {
		const elems = document.querySelector("body > os-settings-ui")
			.shadowRoot.querySelector("#main")
			.shadowRoot.querySelector("os-settings-page")
			.shadowRoot.querySelector("#advancedPage > settings-section.expanded > os-settings-printing-page")
			.shadowRoot.querySelector("#pages > settings-subpage > settings-cups-printers")
			.shadowRoot.querySelector("settings-cups-nearby-printers")
			.shadowRoot.querySelector("settings-cups-printers-entry-list")
			.shadowRoot.querySelectorAll("settings-cups-printers-entry");
		for (var i = 0; i < elems.length; i++) {
			console.log(elems[i].printerEntry.printerInfo.printerName);
			if (elems[i].printerEntry.printerInfo.printerName == %q) {
				elems[i].shadowRoot.querySelector(".icon-add-circle").click();
				return;
			}
		}
		throw new Error('Save printer button not found');
	})()
`, printer), nil)
	return err
}

func containsPrinterName(printers []string, printer string) bool {
	for _, p := range printers {
		if p == printer {
			return true
		}
	}
	return false
}
