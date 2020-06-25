// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/addtest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AddDymoPrinter,
		Desc: "Verifies the lp command enqueues print jobs for Dymo printers",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{dymolwPPD, dymolmPPD, dymoToPrintFile, dymolwGoldenFile, dymolmGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// A PPD which uses the raster2dymolw filter.
	dymolwPPD = "printer_add_dymo_printer_lw450.ppd"

	// A PPD which uses the raster2dymolm filter.
	dymolmPPD = "printer_add_dymo_printer_lm450.ppd"

	// A PDF file to be rendered to the appropriate format.
	dymoToPrintFile = "to_print.pdf"

	dymolwGoldenFile = "printer_add_dymo_lw_printer_golden.bin"
	dymolmGoldenFile = "printer_add_dymo_lm_printer_golden.bin"
)

func AddDymoPrinter(ctx context.Context, s *testing.State) {
	const (
		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		dymolwDiffFile = "dymolw.diff"
		dymolmDiffFile = "dymolm.diff"
	)

	// Tests printing with the old Ink PPDs.
	addtest.Run(ctx, s, dymolwPPD, dymoToPrintFile, dymolwGoldenFile, dymolwDiffFile)
	addtest.Run(ctx, s, dymolmPPD, dymoToPrintFile, dymolmGoldenFile, dymolmDiffFile)
}
