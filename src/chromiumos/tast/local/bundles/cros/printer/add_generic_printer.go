// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func: AddGenericPrinter,
		Desc: "Verifies the lp command enqueues print jobs",
		Contacts: []string{
			"skau@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{genericPPDFile, genericToPrintFile, genericGoldenFile},
		Attr:         []string{"group:mainline"},
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
	lpprint.Run(ctx, s, genericPPDFile, genericToPrintFile, genericGoldenFile)
}
