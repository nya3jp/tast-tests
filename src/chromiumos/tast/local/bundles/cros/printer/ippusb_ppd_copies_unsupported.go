// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/usbprintertests"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPPUSBPPDCopiesUnsupported,
		Desc:         "Verifies that the 'copies-supported' attribute of the printer is used to populate the cupsManualCopies and cupsMaxCopies values in the corresponding generated PPD",
		Contacts:     []string{"skau@chromium.org", "project-bolton@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
		Data:         []string{"ippusb_copies_unsupported.json"},
		Pre:          chrome.LoggedIn(),
	})
}

// IPPUSBPPDCopiesUnsupported tests that the "cupsManualCopies" and
// "cupsMaxCopies" PPD fields will be correctly populated when configuring an
// IPP-over-USB printer whose "copies-supported" IPP attribute has an upper
// limit of 1 (i.e., it does not support copies).
func IPPUSBPPDCopiesUnsupported(ctx context.Context, s *testing.State) {
	const descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
	usbprintertests.RunIPPUSBPPDTest(ctx, s, descriptors, s.DataPath("ippusb_copies_unsupported.json"), map[string]string{
		"*cupsManualCopies": "True",
		"*cupsMaxCopies":    "1",
	})
}
