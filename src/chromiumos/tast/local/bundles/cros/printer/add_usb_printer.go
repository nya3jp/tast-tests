// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

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
	const (
		// Path to JSON descriptors file
		descriptorsPath = "/var/lib/misc/usb_printer.json"
	)

	printer, err := usbprinter.Start(ctx, descriptorsPath)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	printer.Kill()
	printer.Wait()
}
