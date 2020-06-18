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
		Func: PinPrintHP,
		Desc: "Verifies that printers with HP Pin printing support add the appropriate options when given the job-password attribute",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data: []string{
			"printer_pin_print_HP.ppd",
			"to_print.pdf",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "no_pin",
			Val: pinprint.Params{
				PpdFile:    "printer_pin_print_HP.ppd",
				PrintFile:  "to_print.pdf",
				GoldenFile: "printer_pin_print_hp_no_pin_golden.ps",
				DiffFile:   "no-pin_diff.txt",
				Options:    "",
			},
			ExtraData: []string{"printer_pin_print_hp_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "pin",
			Val: pinprint.Params{
				PpdFile:    "printer_pin_print_HP.ppd",
				PrintFile:  "to_print.pdf",
				GoldenFile: "printer_pin_print_hp_pin_golden.ps",
				DiffFile:   "pin_diff.txt",
				Options:    "job-password=1234",
			},
			ExtraData: []string{"printer_pin_print_hp_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func PinPrintHP(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(pinprint.Params)

	pinprint.Run(ctx, s, testOpt.PpdFile, testOpt.PrintFile, testOpt.GoldenFile, testOpt.DiffFile, testOpt.Options)
}
