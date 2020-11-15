// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ippprint"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinPrintSharp,
		Desc: "Verifies that printers with Sharp Pin printing support add the appropriate options when given the job-password attribute",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data: []string{
			"printer_Sharp.ppd",
			"to_print.pdf",
		},
		Attr: []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Sharp.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_sharp_no_pin_golden.ps",
			},
			ExtraData: []string{"printer_pin_print_sharp_no_pin_golden.ps"},
		}, {
			Name: "pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_Sharp.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_sharp_pin_golden.ps",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{"printer_pin_print_sharp_pin_golden.ps"},
		}},
	})
}

func PinPrintSharp(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.Run(ctx, s, testOpt)
}
