// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintIPPUSB,
		Desc:         "Tests ipp-over-usb printing",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups", "virtual_usb_printer"},
		Data:         []string{"print_ippusb_to_print.pdf", "print_ippusb_golden.pdf"},
	})
}

func PrintIPPUSB(ctx context.Context, s *testing.State) {
	const (
		// Path to JSON USB descriptors file.
		descriptors = "/etc/virtual-usb-printer/ippusb_printer.json"
		// Path to JSON IPP attributes file.
		attributes = "/etc/virtual-usb-printer/ipp_attributes.json"
		// Where to record the document received by the virtual printer.
		record = "/tmp/print_ippusb_test.pdf"
		// diffFile is the name of the file containing the diff between the golden
		// file and the document produced by the print request if they differ.
		diffFile = "print_ippusb_diff.txt"
	)

	toPrint := s.DataPath("print_ippusb_to_print.pdf")
	golden := s.DataPath("print_ippusb_golden.pdf")
	diffPath := filepath.Join(s.OutDir(), diffFile)

	usbprinter.RunPrintTest(ctx, s, descriptors, attributes, record, "", toPrint,
		golden, diffPath)
}
