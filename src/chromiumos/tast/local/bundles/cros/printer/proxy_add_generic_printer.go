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
		Func: ProxyAddGenericPrinter,
		Desc: "Verifies the lp command enqueues print jobs",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Data:         []string{proxyGenericPPDFile, proxyGenericToPrintFile, proxyGenericGoldenFile},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline"},
	})
}

const (
	// genericPPDFile is ppd.gz file to be registered via debugd.
	proxyGenericPPDFile = "printer_add_generic_printer_GenericPostScript.ppd.gz"

	// genericToPrintFile is a PDF file to be printed.
	proxyGenericToPrintFile = "to_print.pdf"

	// genericGoldenFile contains a golden LPR request data.
	proxyGenericGoldenFile = "printer_add_generic_printer_golden.ps"
)

func ProxyAddGenericPrinter(ctx context.Context, s *testing.State) {
	proxylpprint.Run(ctx, s, proxyGenericPPDFile, proxyGenericToPrintFile, proxyGenericGoldenFile)
}
