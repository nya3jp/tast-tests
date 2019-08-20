// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddUSBPrinter,
		Desc:         "Verifies setup of a basic USB printer",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
		Pre:          chrome.LoggedIn(),
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

	name, err := usbprinter.LoadPrinterName(descriptors)
	if err != nil {
		s.Fatalf("Failed to load name from %v: %v", descriptors, err)
	}
	s.Log(name)

	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := usbprinter.RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)

	printer, err := usbprinter.Start(ctx, devInfo, descriptors, "", "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}

	// Test cleanup.
	printer.Kill()
	printer.Wait()
	cancel()
}
