// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"syscall"

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
		descriptors = "/etc/virtual-usb-printer/ippusb_printer.json"
		attributes  = "/etc/virtual-usb-printer/ipp_attributes.json"
	)

	recordFile, err := ioutil.TempFile("", "tast.printer.PrintIPPUSB.")
	if err != nil {
		s.Fatal("Failed to create record file: ", err)
	}
	recordFile.Close()
	recordPath := recordFile.Name()
	// Unlink the file since we can't print to a file which already exists.
	syscall.Unlink(recordPath)

	usbprinter.RunPrintTest(ctx, s, descriptors, attributes, recordPath, "", s.DataPath("print_ippusb_to_print.pdf"), s.DataPath("print_ippusb_golden.pdf"))
}
