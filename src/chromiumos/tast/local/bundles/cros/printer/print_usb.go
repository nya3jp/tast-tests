// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintUSB,
		Desc:         "Tests that USB print job can be successfully sent",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "cups", "virtual_usb_printer"},
		Data: []string{"print_usb_ps.ppd.gz", "print_usb_to_print.pdf",
			"print_usb_golden.ps"},
		Pre: chrome.LoggedIn(),
	})
}

func PrintUSB(ctx context.Context, s *testing.State) {
	const (
		descriptors = "/etc/virtual-usb-printer/usb_printer.json"
	)

	tmpDir, err := ioutil.TempDir("", "tast.printer.PrintUSB.")
	if err != nil {
		s.Fatal("Failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)
	recordPath := filepath.Join(tmpDir, "record.pdf")

	usbprinter.RunPrintTest(ctx, s, descriptors, "", recordPath, s.DataPath("print_usb_ps.ppd.gz"), s.DataPath("print_usb_to_print.pdf"), s.DataPath("print_usb_golden.ps"))
}
