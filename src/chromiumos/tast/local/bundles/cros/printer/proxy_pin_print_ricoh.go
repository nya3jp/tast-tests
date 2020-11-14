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
		Func: ProxyPinPrintRicoh,
		Desc: "Verifies that printers with Ricoh Pin printing support add the appropriate options for a variety of attributes",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
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
			Val: &proxyippprint.Params{
				PpdFile:      "printer_Ricoh_JobPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_JobPassword_no_pin_golden.ps",
			},
			ExtraData: []string{"printer_pin_print_ricoh_JobPassword_no_pin_golden.ps"},
		}, {
			Name: "jobpassword_pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_Ricoh_JobPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_JobPassword_pin_golden.ps",
				Options:      []proxyippprint.Option{proxyippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_ricoh_JobPassword_pin_golden.ps"},
		}, {
			Name: "lockedprintpassword_no_pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_Ricoh_LockedPrintPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps",
			},
			ExtraData: []string{"printer_pin_print_ricoh_LockedPrintPassword_no_pin_golden.ps"},
		}, {
			Name: "lockedprintpassword_pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_Ricoh_LockedPrintPassword.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps",
				Options:      []proxyippprint.Option{proxyippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_ricoh_LockedPrintPassword_pin_golden.ps"},
		}, {
			Name: "password_no_pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_Ricoh_password.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_password_no_pin_golden.ps",
			},
			ExtraData: []string{"printer_pin_print_ricoh_password_no_pin_golden.ps"},
		}, {
			Name: "password_pin",
			Val: &proxyippprint.Params{
				PpdFile:      "printer_Ricoh_password.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_ricoh_password_pin_golden.ps",
				Options:      []proxyippprint.Option{proxyippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_ricoh_password_pin_golden.ps"},
		}},
	})
}

func ProxyPinPrintRicoh(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*proxyippprint.Params)

	proxyippprint.Run(ctx, s, testOpt)
}
