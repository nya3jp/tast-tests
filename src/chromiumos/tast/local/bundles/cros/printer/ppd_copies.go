// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PPDCopies,
		Desc:         "Verifies that the 'copies-supported' attribute of the printer is used to populate the cupsManualCopies and cupsMaxCopies values in the corresponding generated PPD",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
		Data:         []string{"ipp_copies.json"},
		Pre:          chrome.LoggedIn(),
	})
}

func PPDCopies(ctx context.Context, s *testing.State) {
	const descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
	usbprinter.RunPPDTest(ctx, s, descriptors, s.DataPath("ipp_copies.json"), map[string]string{
		"*cupsManualCopies": "false",
		"*cupsMaxCopies":    "99",
	})
}
