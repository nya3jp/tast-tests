// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
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
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{starPPD, starlmPPD, starToPrintFile, starGoldenFile, starlmGoldenFile},
	})
}

const (
	// A PPD which uses the rastertostar filter.
	starPPD = "printer_add_star_printer_rastertostar.ppd.gz"

	// A PPD which uses the rastertostarlm filter.
	starlmPPD = "printer_add_star_printer_rastertostarlm.ppd.gz"

	// A PDF file to be rendered to the appropriate format.
	starToPrintFile = "to_print.pdf"

	starGoldenFile   = "printer_add_star_printer_rastertostar.bin"
	starlmGoldenFile = "printer_add_star_printer_rastertostarlm.bin"
)

func AddStarPrinter(ctx context.Context, s *testing.State) {
	// Tests printing using the rastertostar filter.
	lpprint.Run(ctx, s, starPPD, starToPrintFile, starGoldenFile)
	// Tests printing using the rastertostarlm filter.
	lpprint.Run(ctx, s, starlmPPD, starToPrintFile, starlmGoldenFile)
}
