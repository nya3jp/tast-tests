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
		Func: PinPrintRicoh,
		Desc: "Verifies that printers with Ricoh Pin printing support add the appropriate options for a variety of attributes",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data: []string{
			"to_print.pdf",
			"printer_Ricoh_JobPassword.ppd",
			"printer_Ricoh_LockedPrintPassword.ppd",
			"printer_Ricoh_password.ppd",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "jobpassword_no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Ricoh_JobPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_JobPassword_no_pin_golden.ps",
				OutDiffFile:  "jobpassword_no-pin_diff.txt",
			},
			ExtraData: []string{"printer_pin_print_ricoh_JobPassword_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "jobpassword_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Ricoh_JobPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_JobPassword_pin_golden.ps",
				OutDiffFile:  "jobpassword_pin_diff.txt",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_ricoh_JobPassword_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "lockedprintpassword_no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Ricoh_LockedPrintPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps",
				OutDiffFile:  "lockedprintpassword_no-pin_diff.txt",
			},
			ExtraData: []string{"printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "lockedprintpassword_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Ricoh_LockedPrintPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps",
				OutDiffFile:  "lockedprintpassword_pin_diff.txt",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "password_no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Ricoh_password.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_password_no_pin_golden.ps",
				OutDiffFile:  "password_no-pin_diff.txt",
			},
			ExtraData: []string{"printer_pin_print_ricoh_password_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "password_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Ricoh_password.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_password_pin_golden.ps",
				OutDiffFile:  "password_pin_diff.txt",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_ricoh_password_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func PinPrintRicoh(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.Run(ctx, s, testOpt)
}
