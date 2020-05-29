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

type ricohParams struct {
	ppdFile    string
	printFile  string
	goldenFile string
	diffFile   string
	options    string
}

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
			"printer_pin_print_Ricoh_JobPassword.ppd",
			"printer_pin_print_Ricoh_LockedPrintPassword.ppd",
			"printer_pin_print_Ricoh_password.ppd",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "jobpassword_no_pin",
			Val: ricohParams{
				ppdFile:    "printer_pin_print_Ricoh_JobPassword.ppd",
				printFile:  "to_print.pdf",
				goldenFile: "printer_pin_print_ricoh_JobPassword_no_pin_golden.ps",
				diffFile:   "jobpassword_no-pin_diff.txt",
				options:    "",
			},
			ExtraData: []string{"printer_pin_print_ricoh_JobPassword_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "jobpassword_pin",
			Val: ricohParams{
				ppdFile:    "printer_pin_print_Ricoh_JobPassword.ppd",
				printFile:  "to_print.pdf",
				goldenFile: "printer_pin_print_ricoh_JobPassword_pin_golden.ps",
				diffFile:   "jobpassword_pin_diff.txt",
				options:    "job-password=1234",
			},
			ExtraData: []string{"printer_pin_print_ricoh_JobPassword_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "lockedprintpassword_no_pin",
			Val: ricohParams{
				ppdFile:    "printer_pin_print_Ricoh_LockedPrintPassword.ppd",
				printFile:  "to_print.pdf",
				goldenFile: "printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps",
				diffFile:   "lockedprintpassword_no-pin_diff.txt",
				options:    "",
			},
			ExtraData: []string{"printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "lockedprintpassword_pin",
			Val: ricohParams{
				ppdFile:    "printer_pin_print_Ricoh_LockedPrintPassword.ppd",
				printFile:  "to_print.pdf",
				goldenFile: "printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps",
				diffFile:   "lockedprintpassword_pin_diff.txt",
				options:    "job-password=1234",
			},
			ExtraData: []string{"printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "password_no_pin",
			Val: ricohParams{
				ppdFile:    "printer_pin_print_Ricoh_password.ppd",
				printFile:  "to_print.pdf",
				goldenFile: "printer_pin_print_ricoh_password_no_pin_golden.ps",
				diffFile:   "password_no-pin_diff.txt",
				options:    "",
			},
			ExtraData: []string{"printer_pin_print_ricoh_password_no_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "password_pin",
			Val: ricohParams{
				ppdFile:    "printer_pin_print_Ricoh_password.ppd",
				printFile:  "to_print.pdf",
				goldenFile: "printer_pin_print_ricoh_password_pin_golden.ps",
				diffFile:   "password_pin_diff.txt",
				options:    "job-password=1234",
			},
			ExtraData: []string{"printer_pin_print_ricoh_password_pin_golden.ps"},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func PinPrintRicoh(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(ricohParams)

	pinprint.Run(ctx, s, testOpt.ppdFile, testOpt.printFile, testOpt.goldenFile, testOpt.diffFile, testOpt.options)
}
