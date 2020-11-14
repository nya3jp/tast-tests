// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AddHpPrinter,
		Desc: "Verifies the lp command enqueues print jobs for HP printers",
		Contacts: []string{
			"skau@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"cros_internal", "cups"},
		// TODO(b/174612982): Remove once the test works on kefka-kernelnext.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kefka")),
		Data:         []string{hpToPrintFile, hpPclmPpd, hpPclmGoldenFile},
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
