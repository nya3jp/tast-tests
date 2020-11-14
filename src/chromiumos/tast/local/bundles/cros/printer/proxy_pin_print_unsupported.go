// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/proxyippprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProxyPinPrintUnsupported,
		Desc: "Verifies that printers without OEM pin support ignore job-password commands",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Data: []string{
			"printer_unsupported_GenericPostScript.ppd.gz",
			"to_print.pdf",
			"printer_pin_print_unsupported_golden.ps",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "no_pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_unsupported_GenericPostScript.ppd.gz",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_unsupported_golden.ps",
			},
			ExtraData: []string{},
		}, {
			Name: "pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_unsupported_GenericPostScript.ppd.gz",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_unsupported_golden.ps",
				Options:      []proxyippprint.Option{proxyippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{},
		}},
	})
}

func ProxyPinPrintUnsupported(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*proxyippprint.Params)

	proxyippprint.Run(ctx, s, testOpt)
}
