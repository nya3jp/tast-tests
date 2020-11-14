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
		Func: ProxyAddDymoPrinter,
		Desc: "Verifies the lp command enqueues print jobs for Dymo printers",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Data:         []string{proxyDymolwPPD, proxyDymolmPPD, proxyDymoToPrintFile, proxyDymolwGoldenFile, proxyDymolmGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// A PPD which uses the raster2dymolw filter.
	proxyDymolwPPD = "printer_add_dymo_printer_lw450.ppd"

	// A PPD which uses the raster2dymolm filter.
	proxyDymolmPPD = "printer_add_dymo_printer_lm450.ppd"

	// A PDF file to be rendered to the appropriate format.
	proxyDymoToPrintFile = "to_print.pdf"

	proxyDymolwGoldenFile = "printer_add_dymo_lw_printer_golden.bin"
	proxyDymolmGoldenFile = "printer_add_dymo_lm_printer_golden.bin"
)

func ProxyAddDymoPrinter(ctx context.Context, s *testing.State) {
	// Tests printing with the old Ink PPDs.
	proxylpprint.Run(ctx, s, proxyDymolwPPD, proxyDymoToPrintFile, proxyDymolwGoldenFile)
	proxylpprint.Run(ctx, s, proxyDymolmPPD, proxyDymoToPrintFile, proxyDymolmGoldenFile)
}
