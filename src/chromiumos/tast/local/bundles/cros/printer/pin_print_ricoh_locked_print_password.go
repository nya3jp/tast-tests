// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/pinprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinPrintRicohLockedPrintPassword,
		Desc: "Verifies that printers with Ricoh Pin printing support add the appropriate options when given the job-password attribute",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{ricohLockedPrintPasswordPPDFile, ricohLockedPrintPasswordToPrintFile, ricohNoPinLockedPrintPasswordGoldenFile, ricohPinLockedPrintPasswordGoldenFile},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

const (
	// ricohLockedPrintPasswordPPDFile is a ppd where PINs are passed via the LockedPrintPassword option.
	ricohLockedPrintPasswordPPDFile = "printer_pin_print_Ricoh_LockedPrintPassword.ppd"

	// The file to be printed.
	ricohLockedPrintPasswordToPrintFile = "to_print.pdf"

	// The golden file where no PIN is specified.
	ricohNoPinLockedPrintPasswordGoldenFile = "printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps"

	// Golden file with PIN printing specified.
	ricohPinLockedPrintPasswordGoldenFile = "printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps"
)

func PinPrintRicohLockedPrintPassword(ctx context.Context, s *testing.State) {
	const (
		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		noPinDiffFile = "no_pin_diff.txt"
		pinDiffFile   = "pin_diff.txt"
	)

	pinprint.Run(ctx, s, ricohLockedPrintPasswordPPDFile, ricohLockedPrintPasswordToPrintFile, ricohNoPinLockedPrintPasswordGoldenFile, noPinDiffFile, "")
	pinprint.Run(ctx, s, ricohLockedPrintPasswordPPDFile, ricohLockedPrintPasswordToPrintFile, ricohPinLockedPrintPasswordGoldenFile, pinDiffFile, "job-password=1234")
}
