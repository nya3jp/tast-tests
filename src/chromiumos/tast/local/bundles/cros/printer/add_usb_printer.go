// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AddUSBPrinter,
		Desc:     "Verifies setup of a basic USB printer",
		Contacts: []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"cros_internal", "cups", "virtual_usb_printer"},
		Fixture:      "virtualUsbPrinterModulesLoaded",
	})
}

func AddUSBPrinter(ctx context.Context, s *testing.State) {
	// Path to JSON descriptors file
	const descriptors = "/usr/local/etc/virtual-usb-printer/usb_printer.json"

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	printer, err := usbprinter.Start(ctx, devInfo, descriptors, "", "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}

	// Test cleanup.
	printer.Kill()
	printer.Wait()
}
