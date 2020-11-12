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
		Func: PinPrintUnsupported,
		Desc: "Verifies that printers without OEM pin support ignore job-password commands",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data: []string{
			"printer_unsupported_GenericPostScript.ppd.gz",
			"to_print.pdf",
			"printer_pin_print_unsupported_golden.ps",
		},
		Attr: []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_unsupported_GenericPostScript.ppd.gz",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_unsupported_golden.ps",
			},
			ExtraData: []string{},
		}, {
			Name: "pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_unsupported_GenericPostScript.ppd.gz",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_unsupported_golden.ps",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{},
		}},
	})
}

func PinPrintUnsupported(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.Run(ctx, s, testOpt)
}
