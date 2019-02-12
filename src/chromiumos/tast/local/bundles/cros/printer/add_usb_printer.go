// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddUSBPrinter,
		Desc:         "Verifies setup of a basic USB printer",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups", "virtual_usb_printer"},
	})
}

func AddUSBPrinter(ctx context.Context, s *testing.State) {
	// Path to JSON descriptors file
	const descriptorsPath = "/etc/virtual-usb-printer/usb_printer.json"

	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)

	printer, err := usbprinter.Start(ctx, descriptorsPath)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}

	// Test cleanup.
	printer.Kill()
	printer.Wait()

	if err := usbprinter.RemoveModules(ctx); err != nil {
		s.Error("Failed to remove kernel modules: ", err)
	}

	cancel()
}
