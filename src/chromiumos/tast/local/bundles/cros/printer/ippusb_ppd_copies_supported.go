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
		Func:         IPPUSBPPDCopiesSupported,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the 'copies-supported' attribute of the printer is used to populate the cupsManualCopies and cupsMaxCopies values in the corresponding generated PPD",
		Contacts:     []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Data:         []string{"ippusb_copies_supported.json"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

// IPPUSBPPDCopiesSupported tests that the "cupsManualCopies" and
// "cupsMaxCopies" PPD fields will be correctly populated when configuring an
// IPP-over-USB printer whose "copies-supported" attribute has an upper limit
// greater than 1 (i.e., it supports copies).
func IPPUSBPPDCopiesSupported(ctx context.Context, s *testing.State) {
	usbprintertests.RunIPPUSBPPDTest(ctx, s, s.DataPath("ippusb_copies_supported.json"), map[string]string{
		"*cupsManualCopies": "False",
		"*cupsMaxCopies":    "99",
	})
}
