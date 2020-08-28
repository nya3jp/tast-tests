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
		Func: ResolutionLexmark,
		Desc: "Verifies that Lexmark printers add the appropriate options for the IPP printer-resolution attribute",
		Contacts: []string{
			"batrapranav@chromium.org",
			"bmalcolm@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data: []string{
			"to_print.pdf",
			"printer_Lexmark.ppd",
		},
		Attr: []string{"group:mainline"},
		Pre:  chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "default_resolution",
			Val: &ippprint.Params{
				PpdFile:      "printer_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_lexmark_default_golden.ps",
				OutDiffFile:  "printer_resolution_default_diff.txt",
			},
			ExtraData: []string{"printer_resolution_lexmark_default_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "600dpi",
			Val: &ippprint.Params{
				PpdFile:      "printer_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_lexmark_600dpi_golden.ps",
				OutDiffFile:  "printer_resolution_600dpi_diff.txt",
				Options:      []ippprint.Option{ippprint.WithResolution("600dpi")},
			},
			ExtraData: []string{"printer_resolution_lexmark_600dpi_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "1200dpi",
			Val: &ippprint.Params{
				PpdFile:      "printer_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_lexmark_1200dpi_golden.ps",
				OutDiffFile:  "printer_resolution_1200dpi_diff.txt",
				Options:      []ippprint.Option{ippprint.WithResolution("1200dpi")},
			},
			ExtraData: []string{"printer_resolution_lexmark_1200dpi_golden.ps"},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "2400x600dpi",
			Val: &ippprint.Params{
				PpdFile:      "printer_Lexmark.ppd",
				PrintFile:    "to_print.pdf",
				ExpectedFile: "printer_resolution_lexmark_2400x600dpi_golden.ps",
				OutDiffFile:  "printer_resolution_2400x600dpi_diff.txt",
				Options:      []ippprint.Option{ippprint.WithResolution("2400x600dpi")},
			},
			ExtraData: []string{"printer_resolution_lexmark_2400x600dpi_golden.ps"},
			ExtraAttr: []string{"informational"},
		}},
	})
}

func ResolutionLexmark(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.Run(ctx, s, testOpt)
}
