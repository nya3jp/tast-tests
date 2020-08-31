// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/pinprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinPrintLexmark,
		Desc: "Verifies that printers with Lexmark Pin printing support add the appropriate options when given the job-password attribute",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data: []string{
			"printer_pin_print_Lexmark.ppd",
			"to_print.pdf",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "no_pin",
			Val: &pinprint.Params{
				PpdFile:      "printer_pin_print_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_lexmark_no_pin_golden.ps",
				OutDiffFile:  "no-pin_diff.txt",
			},
			ExtraData: []string{"printer_pin_print_lexmark_no_pin_golden.ps"},
		}, {
			Name: "pin",
			Val: &pinprint.Params{
				PpdFile:      "printer_pin_print_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_lexmark_pin_golden.ps",
				OutDiffFile:  "pin_diff.txt",
				Options:      []pinprint.Option{pinprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_lexmark_pin_golden.ps"},
		}},
	})
}

func PinPrintLexmark(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*pinprint.Params)

	pinprint.Run(ctx, s, testOpt)
}
