// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AddEpsonPrinter,
		Desc: "Verifies the lp command enqueues print jobs with Epson config",
		Contacts: []string{
			"skau@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{epsonPPDFile, epsonModPPD, epsonToPrintFile, epsonGoldenFile, epsonColorGoldenFile, epsonMonochromeGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// epsonPPDFile is ppd.gz file to be registered via debugd.
	epsonPPDFile = "printer_add_epson_printer_EpsonWF3620.ppd"

	epsonModPPD = "printer_add_epson_printer_EpsonGenericColorModel.ppd"

	// epsonToPrintFile is a PDF file to be printed.
	epsonToPrintFile = "to_print.pdf"

	// epsonGoldenFile contains a golden LPR request data.
	epsonGoldenFile = "printer_add_epson_printer_golden.ps"

	epsonColorGoldenFile      = "printer_add_epson_printer_color_golden.bin"
	epsonMonochromeGoldenFile = "printer_add_epson_printer_monochrome_golden.bin"
)

func AddEpsonPrinter(ctx context.Context, s *testing.State) {
	const (
		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		diffFile           = "printer_add_epson_printer_diff.txt"
		colorDiffFile      = "color.diff"
		monochromeDiffFile = "monochrome.diff"
	)

	// Tests printing with the old Ink PPDs.
	lpprint.Run(ctx, s, epsonPPDFile, epsonToPrintFile, epsonGoldenFile, diffFile)

	// Tests printing with the modified ColorModel PPD in color and monochrome.
	lpprint.RunWithOptions(ctx, s, epsonModPPD, epsonToPrintFile, epsonColorGoldenFile, colorDiffFile, "print-color-mode=color")
	lpprint.RunWithOptions(ctx, s, epsonModPPD, epsonToPrintFile, epsonMonochromeGoldenFile, monochromeDiffFile, "print-color-mode=monochrome")
}
