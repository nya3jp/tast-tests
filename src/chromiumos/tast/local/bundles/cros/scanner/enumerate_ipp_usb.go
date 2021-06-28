// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"regexp"
	"time"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/scanner/lorgnette"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

type scannerInfo struct {
	name             string
	descriptors      string
	attributes       string
	esclCapabilities string
	platenImage      string
	shouldEnumerate  bool
}

var ippUsbFormat = regexp.MustCompile("^ippusb:escl:.*:(....)_(....)/.*")

func init() {
	testing.AddTest(&testing.Test{
		Func:     EnumerateIPPUSB,
		Desc:     "Tests that IPP-USB devices are correctly found",
		Contacts: []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// isMatchingScanner returns true iff scanner refers to an ippusb device with the same VID and PID as devInfo.
func isMatchingScanner(scanner *lpb.ScannerInfo, devInfo usbprinter.DevInfo) bool {
	if match := ippUsbFormat.FindStringSubmatch(scanner.Name); match != nil {
		return devInfo.VID == match[1] && devInfo.PID == match[2]
	}

	return false
}

// runEnumerationTest sets up virtual-usb-printer to emulate the device specified in info,
// calls lorgnette's ListScanners, and checks to see if the device was listed in the response.
func runEnumerationTest(ctx context.Context, s *testing.State, info scannerInfo) {
	s.Logf("Checking if %s is listed", info.name)

	devInfo, err := usbprinter.LoadPrinterIDs(info.descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", info.descriptors, err)
	}

	if err := cups.RestartPrintingSystem(ctx, devInfo); err != nil {
		s.Fatal("Failed to restart printing system: ", err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, info.descriptors, info.attributes, info.esclCapabilities, "", "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		usbprinter.StopPrinter(ctx, printer, devInfo)
	}()
	if info.attributes != "" || info.esclCapabilities != "" {
		// Only wait for CUPS if we added an IPP-USB device.  It won't attempt to
		// auto-configure non-IPP devices, so this would never finish.
		if err := cups.EnsurePrinterIdle(ctx, devInfo); err != nil {
			s.Fatal("Failed to wait for CUPS configuration: ", err)
		}
	}

	s.Log("Requesting scanner list")
	l, err := lorgnette.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to lorgnette: ", err)
	}
	defer func() {
		// Lorgnette was auto started during testing.  Kill it to avoid
		// affecting subsequent tests.
		lorgnette.StopService(ctx)
	}()
	scanners, err := l.ListScanners(ctx)
	if err != nil {
		s.Fatal("Failed to call ListScanners: ", err)
	}

	found := false
	for _, scanner := range scanners {
		if isMatchingScanner(scanner, devInfo) {
			found = true
			break
		}
	}
	if found != info.shouldEnumerate {
		s.Errorf("%s enumerated=%v, want=%v", info.name, found, info.shouldEnumerate)
	}
}

func EnumerateIPPUSB(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := usbprinter.RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}(cleanupCtx)

	for _, info := range []scannerInfo{{
		name:             "Non-IPP USB printer",
		descriptors:      "/usr/local/etc/virtual-usb-printer/usb_printer.json",
		attributes:       "",
		esclCapabilities: "",
		shouldEnumerate:  false,
	}, {
		name:             "IPP USB printer without eSCL",
		descriptors:      "/usr/local/etc/virtual-usb-printer/ippusb_printer.json",
		attributes:       "/usr/local/etc/virtual-usb-printer/ipp_attributes.json",
		esclCapabilities: "",
		shouldEnumerate:  false,
	}, {
		name:             "IPP USB printer with eSCL",
		descriptors:      "/usr/local/etc/virtual-usb-printer/ippusb_printer.json",
		attributes:       "/usr/local/etc/virtual-usb-printer/ipp_attributes.json",
		esclCapabilities: "/usr/local/etc/virtual-usb-printer/escl_capabilities.json",
		shouldEnumerate:  true,
	}} {
		runEnumerationTest(ctx, s, info)
	}
}
