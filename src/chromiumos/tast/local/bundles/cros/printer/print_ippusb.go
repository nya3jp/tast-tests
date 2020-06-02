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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintIPPUSB,
		Desc:         "Tests ipp-over-usb printing",
		Contacts:     []string{"skau@chromium.org", "project-bolton@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
		Data:         []string{"print_ippusb_to_print.pdf", "print_ippusb_golden.pdf"},
		Pre:          chrome.LoggedIn(),
	})
}

func PrintIPPUSB(ctx context.Context, s *testing.State) {
	const (
		descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		attributes  = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	)

	tmpDir, err := ioutil.TempDir("", "tast.printer.PrintIPPUSB.")
	if err != nil {
		s.Fatal("Failed to create temporary directory")
	}
	defer os.RemoveAll(tmpDir)
	recordPath := filepath.Join(tmpDir, "record.pdf")

	usbprintertests.RunPrintTest(ctx, s, descriptors, attributes, recordPath, "", s.DataPath("print_ippusb_to_print.pdf"), s.DataPath("print_ippusb_golden.pdf"))
}
