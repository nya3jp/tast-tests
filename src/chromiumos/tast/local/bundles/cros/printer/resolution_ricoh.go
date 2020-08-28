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
		Func: ResolutionRicoh,
		Desc: "Verifies that Ricoh printers add the appropriate options for the IPP printer-resolution attribute",
		Contacts: []string{
			"batrapranav@chromium.org",
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data: []string{
			"to_print.pdf",
			"printer_resolution_Ricoh.ppd",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "default_resolution",
			Val: &pinprint.Params{
				PpdFile:      "printer_resolution_Ricoh.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_ricoh_600dpi_golden.ps",
				OutDiffFile:  "printer_resolution_default_diff.txt",
			},
			ExtraData: []string{"printer_resolution_ricoh_600dpi_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "600dpi",
			Val: &pinprint.Params{
				PpdFile:      "printer_resolution_Ricoh.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_ricoh_600dpi_golden.ps",
				OutDiffFile:  "printer_resolution_600dpi_diff.txt",
				Options:      []pinprint.Option{"printer-resolution=600dpi"},
			},
			ExtraData: []string{"printer_resolution_ricoh_600dpi_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "1200dpi",
			Val: &pinprint.Params{
				PpdFile:      "printer_resolution_Ricoh.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_ricoh_1200dpi_golden.ps",
				OutDiffFile:  "printer_resolution_1200dpi_diff.txt",
				Options:      []pinprint.Option{"printer-resolution=1200dpi"},
			},
			ExtraData: []string{"printer_resolution_ricoh_1200dpi_golden.ps"},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func ResolutionRicoh(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*pinprint.Params)

	pinprint.Run(ctx, s, testOpt)
}
