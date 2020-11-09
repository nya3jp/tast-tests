// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func: AddHpPrinter,
		Desc: "Verifies the lp command enqueues print jobs for HP printers",
		Contacts: []string{
			"skau@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{hpToPrintFile, hpPclmPpd, hpPclmGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	hpToPrintFile = "to_print.pdf"

	// A PPD which uses the hbpl1 hpPrinterLanguage (aka PCLm) in the hpcups filter.
	hpPclmPpd        = "printer_add_hp_printer_pclm.ppd.gz"
	hpPclmGoldenFile = "printer_add_hp_printer_pclm_out.pclm"
)

func AddHpPrinter(ctx context.Context, s *testing.State) {
	// Test PCLm PDL.
	lpprint.Run(ctx, s, hpPclmPpd, hpToPrintFile, hpPclmGoldenFile)
}
