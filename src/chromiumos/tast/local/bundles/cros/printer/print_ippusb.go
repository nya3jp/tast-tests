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
		Func:         PrintIPPUSB,
		Desc:         "Tests ipp-over-usb printing",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "cups", "virtual_usb_printer"},
		Data:         []string{"print_ippusb_to_print.pdf", "print_ippusb_golden.pdf"},
		Pre:          chrome.LoggedIn(),
	})
}

func PrintIPPUSB(ctx context.Context, s *testing.State) {
	const (
		descriptors = "/etc/virtual-usb-printer/ippusb_printer.json"
		attributes  = "/etc/virtual-usb-printer/ipp_attributes.json"
	)

	tmpDir, err := ioutil.TempDir("", "tast.printer.PrintIPPUSB.")
	if err != nil {
		s.Fatal("Failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)
	recordPath := filepath.Join(tmpDir, "record.pdf")

	usbprinter.RunPrintTest(ctx, s, descriptors, attributes, recordPath, "", s.DataPath("print_ippusb_to_print.pdf"), s.DataPath("print_ippusb_golden.pdf"))
}
