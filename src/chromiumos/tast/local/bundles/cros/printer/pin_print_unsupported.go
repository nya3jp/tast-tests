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
		Func: PinPrintUnsupported,
		Desc: "Verifies that printers without OEM pin support ignore job-password commands",
		Contacts: []string{
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data: []string{
			"printer_unsupported_GenericPostScript.ppd.gz",
			"to_print.pdf",
			"printer_pin_print_unsupported_golden.ps",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "no_pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_unsupported_GenericPostScript.ppd.gz",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_unsupported_golden.ps",
				OutDiffFile:  "no-pin_diff.txt",
			},
			ExtraData: []string{},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "pin",
			Val: &ippprint.Params{
				PpdFile:      "printer_unsupported_GenericPostScript.ppd.gz",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_pin_print_unsupported_golden.ps",
				OutDiffFile:  "pin_diff.txt",
				Options:      []ippprint.Option{ippprint.WithJobPassword("1234")},
			},
			ExtraData: []string{},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func PinPrintUnsupported(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.Run(ctx, s, testOpt)
}
