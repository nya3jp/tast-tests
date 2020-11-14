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
		Func: ProxyAddStarPrinter,
		Desc: "Verifies the lp command enqueues print jobs for Star printers",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Data:         []string{proxyStarPPD, proxyStarlmPPD, proxyStarToPrintFile, proxyStarGoldenFile, proxyStarlmGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// A PPD which uses the rastertostar filter.
	proxyStarPPD = "printer_add_star_printer_rastertostar.ppd.gz"

	// A PPD which uses the rastertostarlm filter.
	proxyStarlmPPD = "printer_add_star_printer_rastertostarlm.ppd.gz"

	// A PDF file to be rendered to the appropriate format.
	proxyStarToPrintFile = "to_print.pdf"

	proxyStarGoldenFile   = "printer_add_star_printer_rastertostar.bin"
	proxyStarlmGoldenFile = "printer_add_star_printer_rastertostarlm.bin"
)

func ProxyAddStarPrinter(ctx context.Context, s *testing.State) {
	// Tests printing using the rastertostar filter.
	proxylpprint.Run(ctx, s, proxyStarPPD, proxyStarToPrintFile, proxyStarGoldenFile)
	// Tests printing using the rastertostarlm filter.
	proxylpprint.Run(ctx, s, proxyStarlmPPD, proxyStarToPrintFile, proxyStarlmGoldenFile)
}
