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
		Func: CollateEpson,
		Desc: "Verifies that Epson printers add the appropriate options for the IPP multiple-document-handling attribute",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{"2page.pdf"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "software_collate",
			Val: &ippprint.Params{
				PpdFile:      "printer_EpsonWF3620.ppd",
				PrintFile:    "2page.pdf",
				ExpectedFile: "printer_collate_epson_software_collate_golden.bin",
				Options:      []ippprint.Option{ippprint.WithCopies(2), ippprint.Collate()},
			},
			ExtraData: []string{"printer_EpsonWF3620.ppd", "printer_collate_epson_software_collate_golden.bin"},
		}, {
			Name: "software_uncollated",
			Val: &ippprint.Params{
				PpdFile:      "printer_EpsonWF3620.ppd",
				PrintFile:    "2page.pdf",
				ExpectedFile: "printer_collate_epson_software_uncollated_golden.bin",
				Options:      []ippprint.Option{ippprint.WithCopies(2)},
			},
			ExtraData: []string{"printer_EpsonWF3620.ppd", "printer_collate_epson_software_uncollated_golden.bin"},
		}, {
			Name: "hardware_collate",
			Val: &ippprint.Params{
				PpdFile:      "printer_EpsonWFC20590.ppd",
				PrintFile:    "2page.pdf",
				ExpectedFile: "printer_collate_epson_hardware_collate_golden.ps",
				Options:      []ippprint.Option{ippprint.WithCopies(2), ippprint.Collate()},
			},
			ExtraData: []string{"printer_EpsonWFC20590.ppd", "printer_collate_epson_hardware_collate_golden.ps"},
		}, {
			Name: "hardware_uncollated",
			Val: &ippprint.Params{
				PpdFile:      "printer_EpsonWFC20590.ppd",
				PrintFile:    "2page.pdf",
				ExpectedFile: "printer_collate_epson_hardware_uncollated_golden.ps",
				Options:      []ippprint.Option{ippprint.WithCopies(2)},
			},
			ExtraData: []string{"printer_EpsonWFC20590.ppd", "printer_collate_epson_hardware_uncollated_golden.ps"},
		}},
	})
}

func CollateEpson(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(*ippprint.Params)

	ippprint.Run(ctx, s, testOpt)
}
