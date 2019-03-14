// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func: AddGenericPrinter,
		Desc: "Verifies the lp command enqueues print jobs",
		Contacts: []string{
			"xiaochu@chromium.org",  // Original autotest author
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome_login", "cups"},
		Data:         []string{genericPPDFile, genericToPrintFile, genericGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// genericPPDFile is ppd.gz file to be registered via debugd.
	genericPPDFile = "printer_add_generic_printer_GenericPostScript.ppd.gz"

	// genericToPrintFile is a PDF file to be printed.
	genericToPrintFile = "to_print.pdf"

	// genericGoldenFile contains a golden LPR request data.
	genericGoldenFile = "printer_add_generic_printer_golden.ps"
)

func AddGenericPrinter(ctx context.Context, s *testing.State) {
	// diffFile is the name of the file containing the diff between the
	// golden data and actual request in case of failure.
	const diffFile = "printer_add_generic_printer_diff.txt"

	addtest.Run(ctx, s, genericPPDFile, genericToPrintFile, genericGoldenFile, diffFile)
}
