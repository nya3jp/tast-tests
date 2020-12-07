// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ippprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProxyPinPrintLexmark,
		Desc: "Verifies that printers with Lexmark Pin printing support add the appropriate options when given the job-password attribute",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Data: []string{
			"printer_Lexmark.ppd",
			"to_print.pdf",
		},
		// TODO(crbug.com/1131304): Mark as informational until the test can be fixed.
		Attr: []string{"group:mainline", "informational"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_lexmark_no_pin_golden.ps",
			},
			ExtraData: []string{"printer_pin_print_lexmark_no_pin_golden.ps"},
		}, {
			Name: "pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_lexmark_pin_golden.ps",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_lexmark_pin_golden.ps"},
		}},
	})
}

func ProxyPinPrintLexmark(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.ProxyRun(ctx, s, testOpt)
}
