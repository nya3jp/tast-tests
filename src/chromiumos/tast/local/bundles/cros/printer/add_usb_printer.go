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
		Func:         AddUsbPrinter,
		Desc:         "Verifies setup of a basic USB printer",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups", "usbip"},
	})
}

func AddUsbPrinter(ctx context.Context, s *testing.State) {
	const action = "add"
	const vid = "04a9" // USB vendor ID of the virtual printer.
	const pid = "27e8" // USB product ID of the virtual printer.
	const descriptorsPath = "/var/lib/misc/usb_printer.json"

	printer, err := usbprinter.Setup(ctx, action, vid, pid, descriptorsPath)
	if err != nil {
		s.Fatal("failed to attach virtual printer: ", err)
	}
	defer func() {
		printer.Kill()
		printer.Wait()
	}()
}
