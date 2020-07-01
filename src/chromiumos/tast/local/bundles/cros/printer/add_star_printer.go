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
		Func: AddStarPrinter,
		Desc: "Verifies the lp command enqueues print jobs for Star printers",
		Contacts: []string{
			"skau@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{starPPD, starlmPPD, starToPrintFile, starGoldenFile, starlmGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// A PPD which uses the rastertostar filter.
	starPPD = "printer_add_star_printer_rastertostar.ppd"

	// A PPD which uses the rastertostarlm filter.
	starlmPPD = "printer_add_star_printer_rastertostarlm.ppd"

	// A PDF file to be rendered to the appropriate format.
	starToPrintFile = "to_print.pdf"

	starGoldenFile   = "printer_add_star_printer_golden.bin"
	starlmGoldenFile = "printer_add_starlm_printer_golden.bin"
)

func AddStarPrinter(ctx context.Context, s *testing.State) {
	const (
		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		starDiffFile   = "star.diff"
		starlmDiffFile = "starlm.diff"
	)

	// Tests printing using the rastertostar filter.
	addtest.Run(ctx, s, starPPD, starToPrintFile, starGoldenFile, starDiffFile)
	// Tests printing using the rastertostarlm filter.
	addtest.Run(ctx, s, starlmPPD, starToPrintFile, starlmGoldenFile, starlmDiffFile)
}
