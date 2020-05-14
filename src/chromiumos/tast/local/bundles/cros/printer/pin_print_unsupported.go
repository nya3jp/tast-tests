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
		Func: PinPrintUnsupported,
		Desc: "Verifies that printers without OEM pin support ignore job-password commands",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{unsupportedPPDFile, unsupportedToPrintFile, unsupportedGoldenFile},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

const (
	// unsupportedPPDFile is ppd.gz file to be registered via debugd.
	unsupportedPPDFile = "printer_pin_print_unsupported_GenericPostScript.ppd.gz"

	// The file to be printed.
	unsupportedToPrintFile = "to_print.pdf"

	// unsupportedGoldenFile containing the print job output.
	unsupportedGoldenFile = "printer_pin_print_unsupported_golden.ps"
)

func PinPrintUnsupported(ctx context.Context, s *testing.State) {
	const (
		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		noPinDiffFile = "no_pin_diff.txt"
		pinDiffFile   = "pin_diff.txt"
	)

	// Both jobs should match the same golden because the option is ignored.
	pinprint.Run(ctx, s, unsupportedPPDFile, unsupportedToPrintFile, unsupportedGoldenFile, noPinDiffFile, "")
	pinprint.Run(ctx, s, unsupportedPPDFile, unsupportedToPrintFile, unsupportedGoldenFile, pinDiffFile, "job-password=1234")
}
