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
		descriptors = "/etc/virtual-usb-printer/usb_printer.json"
	)

	recordFile, err := ioutil.TempFile("", "tast.printer.PrintUSB.")
	if err != nil {
		s.Fatal("Failed to create record file: ", err)
	}
	recordFile.Close()
	recordPath := recordFile.Name()
	// Unlink the file since we can't print to a file which already exists.
	syscall.Unlink(recordPath)

	usbprinter.RunPrintTest(ctx, s, descriptors, "", recordPath, s.DataPath("print_usb_ps.ppd.gz"), s.DataPath("print_usb_to_print.pdf"), s.DataPath("print_usb_golden.ps"))
}
