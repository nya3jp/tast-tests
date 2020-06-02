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
		Func: PinPrintHP,
		Desc: "Verifies that printers with HP Pin printing support add the appropriate options when given the job-password attribute",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{hpPPDFile, hpToPrintFile, hpNoPinGoldenFile, hpPinGoldenFile},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

const (
	// hpPPDFile is a ppd where PIN printing is specified via
	// HPDigit / HPPinPrnt options.
	hpPPDFile = "printer_pin_print_HP.ppd"

	// The file to be printed.
	hpToPrintFile = "to_print.pdf"

	// The golden file where no PIN is specified.
	hpNoPinGoldenFile = "printer_pin_print_hp_no_pin_golden.ps"

	// Golden file with PIN printing specified.
	hpPinGoldenFile = "printer_pin_print_hp_pin_golden.ps"
)

func PinPrintHP(ctx context.Context, s *testing.State) {
	const (
		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		noPinDiffFile = "no_pin_diff.txt"
		pinDiffFile   = "pin_diff.txt"
	)

	// Both jobs should match the same golden because the option is ignored.
	pinprint.Run(ctx, s, hpPPDFile, hpToPrintFile, hpNoPinGoldenFile, noPinDiffFile, "")
	pinprint.Run(ctx, s, hpPPDFile, hpToPrintFile, hpPinGoldenFile, pinDiffFile, "job-password=1234")
}
