// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"

	"chromiumos/tast/local/bundles/cros/scanapp/scanning"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LargePaperScans,
		Desc:     "Tests that the Scan app supports large paper size selection when available from printer",
		Contacts: []string{"kmoed@google.com", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Fixture:      "chromeLoggedIn",
		Data: []string{
			scanning.SourceImage,
			a3GoldenFile,
			a4GoldenFile,
			b4GoldenFile,
			legalGoldenFile,
			letterGoldenFile,
			tabloidGoldenFile,
		},
	})
}

const (
	esclCapabilities  = "/usr/local/etc/virtual-usb-printer/escl_capabilities_large_paper_sizes.json"
	a3GoldenFile      = "a3_golden_file.png"
	a4GoldenFile      = "a4_golden_file.png"
	b4GoldenFile      = "b4_golden_file.png"
	legalGoldenFile   = "legal_golden_file.png"
	letterGoldenFile  = "letter_golden_file.png"
	tabloidGoldenFile = "tabloid_golden_file.png"
)

var testSetups = []scanning.TestingStruct{
	{
		Name: "paper_size_a3",
		Settings: scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeA3,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: a3GoldenFile,
	}, {
		Name: "paper_size_a4",
		Settings: scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeA4,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: a4GoldenFile,
	}, {
		Name: "paper_size_b4",
		Settings: scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeB4,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: b4GoldenFile,
	}, {
		Name: "paper_size_legal",
		Settings: scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeLegal,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: legalGoldenFile,
	}, {
		Name: "paper_size_letter",
		Settings: scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeLetter,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: letterGoldenFile,
	}, {
		Name: "paper_size_tabloid",
		Settings: scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeTabloid,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: tabloidGoldenFile,
	},
}

func LargePaperScans(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	var scannerParams = scanning.ScannerStruct{
		Descriptors:     scanning.Descriptors,
		Attributes:      scanning.Attributes,
		EsclCaps:        esclCapabilities,
		SourceImagePath: s.DataPath(scanning.SourceImage),
	}

	scanning.RunAppSettingsTests(ctx, s, cr, testSetups, scannerParams)
}
