// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/printer/usbprintertests"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintIPPUSB,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests ipp-over-usb printing",
		Contacts:     []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Data:         []string{"print_ippusb_to_print.pdf", "print_ippusb_golden.pdf"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

func PrintIPPUSB(ctx context.Context, s *testing.State) {
	tmpDir, err := ioutil.TempDir("", "tast.printer.PrintIPPUSB.")
	if err != nil {
		s.Fatal("Failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)
	recordPath := filepath.Join(tmpDir, "record.pdf")

	usbprintertests.RunPrintTest(ctx, s,
		[]usbprinter.Option{
			usbprinter.WithIPPUSBDescriptors(),
			usbprinter.WithGenericIPPAttributes(),
			usbprinter.WithRecordPath(recordPath),
			usbprinter.WaitUntilConfigured(),
		},
		recordPath,
		"",
		s.DataPath("print_ippusb_to_print.pdf"),
		s.DataPath("print_ippusb_golden.pdf"))
}
