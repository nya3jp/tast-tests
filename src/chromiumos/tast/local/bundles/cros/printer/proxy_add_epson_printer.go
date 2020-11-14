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
		Func: ProxyAddEpsonPrinter,
		Desc: "Verifies the lp command enqueues print jobs with Epson config",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Data:         []string{proxyEpsonPPDFile, proxyEpsonModPPD, proxyEpsonToPrintFile, proxyEpsonGoldenFile, proxyEpsonColorGoldenFile, proxyEpsonMonochromeGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// epsonPPDFile is ppd.gz file to be registered via debugd.
	proxyEpsonPPDFile = "printer_add_epson_printer_EpsonWF3620.ppd"

	proxyEpsonModPPD = "printer_add_epson_printer_EpsonGenericColorModel.ppd"

	// epsonToPrintFile is a PDF file to be printed.
	proxyEpsonToPrintFile = "to_print.pdf"

	// epsonGoldenFile contains a golden LPR request data.
	proxyEpsonGoldenFile = "printer_add_epson_printer_golden.ps"

	proxyEpsonColorGoldenFile      = "printer_add_epson_printer_color_golden.bin"
	proxyEpsonMonochromeGoldenFile = "printer_add_epson_printer_monochrome_golden.bin"
)

func ProxyAddEpsonPrinter(ctx context.Context, s *testing.State) {
	// Tests printing with the old Ink PPDs.
	proxylpprint.Run(ctx, s, proxyEpsonPPDFile, proxyEpsonToPrintFile, proxyEpsonGoldenFile)

	// Tests printing with the modified ColorModel PPD in color and monochrome.
	proxylpprint.RunWithOptions(ctx, s, proxyEpsonModPPD, proxyEpsonToPrintFile, proxyEpsonColorGoldenFile, "print-color-mode=color")
	proxylpprint.RunWithOptions(ctx, s, proxyEpsonModPPD, proxyEpsonToPrintFile, proxyEpsonMonochromeGoldenFile, "print-color-mode=monochrome")
}
