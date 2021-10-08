// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/scanapp/scanning"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Scan,
		Desc: "Tests that the Scan app can be used to perform scans",
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
		Timeout:      20 * time.Minute,
		Fixture:      "chromeLoggedIn",
		Data: []string{
			scanning.SourceImage,
			pngGoldenFile,
			jpgGoldenFile,
			pdfGoldenFile,
		},
	})
}

const (
	pngGoldenFile = "test_png.png"
	jpgGoldenFile = "adf_simplex_jpg_grayscale_a4_150_dpi.jpg"
	pdfGoldenFile = "adf_duplex_pdf_grayscale_max_300_dpi.pdf"
)

var tests = []scanning.TestingStruct{
	{
		Name: "flatbed_png_color_letter_300_dpi",
		Settings: scanapp.ScanSettings{
			Scanner:    "Canon MF741C/743C (USB)",
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePNG,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeLetter,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: pngGoldenFile,
	}, {
		Name: "adf_simplex_jpg_grayscale_a4_150_dpi",
		Settings: scanapp.ScanSettings{
			Scanner:  "Canon MF741C/743C (USB)",
			Source:   scanapp.SourceADFOneSided,
			FileType: scanapp.FileTypeJPG,
			// TODO(b/181773386): Change this to black and white when the virtual
			// USB printer correctly reports the color mode.
			ColorMode:  scanapp.ColorModeGrayscale,
			PageSize:   scanapp.PageSizeA4,
			Resolution: scanapp.Resolution150DPI,
		},
		GoldenFile: jpgGoldenFile,
	}, {
		Name: "adf_duplex_pdf_grayscale_max_300_dpi",
		Settings: scanapp.ScanSettings{
			Scanner:    "Canon MF741C/743C (USB)",
			Source:     scanapp.SourceADFTwoSided,
			FileType:   scanapp.FileTypePDF,
			ColorMode:  scanapp.ColorModeGrayscale,
			PageSize:   scanapp.PageSizeFitToScanArea,
			Resolution: scanapp.Resolution300DPI,
		},
		GoldenFile: pdfGoldenFile,
	},
}

func Scan(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	var scannerParams = scanning.UsbScanner{
		Name: "Canon MF741C/743C (USB)",
		DevInfo: usbprinter.DevInfo{
			VID: "04a9",
			PID: "27fc",
		},
	}

	scanning.RunAppSettingsTestsUsb(ctx, s, cr, tests, scannerParams)
}
