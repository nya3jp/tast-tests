// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/proxylpprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProxyAddHpPrinter,
		Desc: "Verifies the lp command enqueues print jobs for HP printers",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Pre:          chrome.LoggedIn(),
		Data:         []string{proxyHpToPrintFile, proxyHpPclmPpd, proxyHpPclmGoldenFile},
	})
}

const (
	proxyHpToPrintFile = "to_print.pdf"

	// A PPD which uses the hbpl1 hpPrinterLanguage (aka PCLm) in the hpcups filter.
	proxyHpPclmPpd        = "printer_add_hp_printer_pclm.ppd.gz"
	proxyHpPclmGoldenFile = "printer_add_hp_printer_pclm_out.pclm"
)

func ProxyAddHpPrinter(ctx context.Context, s *testing.State) {
	// Test PCLm PDL.
	proxylpprint.Run(ctx, s, proxyHpPclmPpd, proxyHpToPrintFile, proxyHpPclmGoldenFile)
}
