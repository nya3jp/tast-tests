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
		Func:         PrintUSB,
		Desc:         "Tests that USB print job can be successfully sent",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups", "virtual_usb_printer"},
		Data: []string{"print_usb_ps.ppd.gz", "print_usb_to_print.pdf",
			"print_usb_golden.ps"},
	})
}

func PrintUSB(ctx context.Context, s *testing.State) {
	const (
		// Path to JSON USB descriptors file.
		descriptors = "/etc/virtual-usb-printer/usb_printer.json"
		// Where to record the document received by the virtual printer.
		record = "/tmp/print_usb_test.ps"
	)

	ppd := s.DataPath("print_usb_ps.ppd.gz")
	toPrint := s.DataPath("print_usb_to_print.pdf")
	golden := s.DataPath("print_usb_golden.ps")

	usbprinter.RunPrintTest(ctx, s, descriptors, "", record, ppd, toPrint, golden)
}
