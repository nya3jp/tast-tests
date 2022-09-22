// Copyright 2022 The ChromiumOS Authors
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
		Func:         ScanBackFlipped,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the Scan app properly flips reverse sides of duplex pages",
		Contacts: []string{
			"cros-peripherals@google.com",
			"project-bolton@google.com",
		},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"chrome", "virtual_usb_printer"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
		Data: []string{
			scanning.SourceImage,
			pdfBackFlippedGoldenFile,
		},
	})
}

const (
	pdfBackFlippedGoldenFile = "adf_duplex_pdf_color_letter_100_dpi.pdf"
)

var backFlippedTests = []scanning.TestingStruct{
	{
		Name: "adf_duplex_pdf_color_letter_100_dpi",
		Settings: scanapp.ScanSettings{
			Source:     scanapp.SourceADFTwoSided,
			FileType:   scanapp.FileTypePDF,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeLetter,
			Resolution: scanapp.Resolution100DPI,
		},
		GoldenFile: pdfBackFlippedGoldenFile,
	},
}

func ScanBackFlipped(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	var scannerParams = scanning.ScannerStruct{
		Descriptors: scanning.FlipTestDescriptors,
		Attributes:  scanning.Attributes,
		EsclCaps:    scanapp.EsclCapabilities,
	}
	scanning.RunAppSettingsTests(ctx, s, cr, backFlippedTests, scannerParams)
}
