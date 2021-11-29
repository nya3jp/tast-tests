// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"regexp"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/local/bundles/cros/scanner/lorgnette"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

type scannerInfo struct {
	name            string
	options         []usbprinter.Option
	platenImage     string
	shouldEnumerate bool
}

var ippUsbFormat = regexp.MustCompile("^ippusb:escl:.*:(....)_(....)/.*")

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnumerateIPPUSB,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that IPP-USB devices are correctly found",
		Contacts:     []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
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

	if err := cups.RestartPrintingSystem(ctx); err != nil {
		s.Fatal("Failed to restart printing system: ", err)
	}

	printer, err := usbprinter.Start(ctx, info.options...)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(ctx)

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
		if isMatchingScanner(scanner, printer.DevInfo) {
			found = true
			break
		}
	}
	if found != info.shouldEnumerate {
		s.Errorf("%s enumerated=%v, want=%v", info.name, found, info.shouldEnumerate)
	}
}

func EnumerateIPPUSB(ctx context.Context, s *testing.State) {
	for _, info := range []scannerInfo{{
		name: "Non-IPP USB printer",
		options: []usbprinter.Option{
			usbprinter.WithDescriptors("usb_printer.json"),
			usbprinter.ExpectUdevEventOnStop(),
		},
		shouldEnumerate: false,
	}, {
		name: "IPP USB printer without eSCL",
		options: []usbprinter.Option{
			usbprinter.WithIPPUSBDescriptors(),
			usbprinter.WithGenericIPPAttributes(),
			usbprinter.WaitUntilConfigured(),
			usbprinter.ExpectUdevEventOnStop(),
		},
		shouldEnumerate: false,
	}, {
		name: "IPP USB printer with eSCL",
		options: []usbprinter.Option{
			usbprinter.WithIPPUSBDescriptors(),
			usbprinter.WithGenericIPPAttributes(),
			usbprinter.WithESCLCapabilities("/usr/local/etc/virtual-usb-printer/escl_capabilities.json"),
			usbprinter.WaitUntilConfigured(),
			usbprinter.ExpectUdevEventOnStop(),
		},
		shouldEnumerate: true,
	}} {
		runEnumerationTest(ctx, s, info)
	}
}
