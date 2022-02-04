// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/usbprintertests"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPPUSBPPDNoCopies,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the 'copies-supported' attribute of the printer is used to populate the cupsManualCopies and cupsMaxCopies values in the corresponding generated PPD",
		Contacts:     []string{"skau@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Data:         []string{"ippusb_no_copies.json"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

// IPPUSBPPDNoCopies tests that the "cupsManualCopies" and "cupsMaxCopies" PPD
// fields will be correctly populated when configuring an IPP-over-USB printer
// which does not provide a value for the "copies-supported" attribute.
func IPPUSBPPDNoCopies(ctx context.Context, s *testing.State) {
	usbprintertests.RunIPPUSBPPDTest(ctx, s, s.DataPath("ippusb_no_copies.json"), map[string]string{
		"*cupsManualCopies": "True",
		"*cupsMaxCopies":    "1",
	})
}
