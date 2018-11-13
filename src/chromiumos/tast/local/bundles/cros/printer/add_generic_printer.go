// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/addtest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddGenericPrinter,
		Desc:         "Verifies the lp command enqueues print jobs",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups"},
		Data:         []string{genericPpdFile, genericToPrintFile, genericGoldenFile},
	})
}

const (
	// ppdFile is ppd.gz file to be registered via debugd.
	genericPpdFile = "printer_add_generic_printer_GenericPostScript.ppd.gz"

	// toPrintFile is a PDF file to be printed.
	genericToPrintFile = "to_print.pdf"

	// goldenFile contains a golden LPR request data.
	genericGoldenFile = "printer_add_generic_printer_golden.ps"
)

func AddGenericPrinter(ctx context.Context, s *testing.State) {
	// diffFile is the name of the file containing the diff between the
	// golden data and actual request in case of failure.
	const diffFile = "printer_add_generic_printer_diff.txt"

	addtest.Run(ctx, s, genericPpdFile, genericToPrintFile, genericGoldenFile, diffFile)
}
