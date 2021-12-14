// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/scanapp/scanning"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Hardware,
		Desc: "Tests that the Scan app can be used on real hardware",
		Contacts: []string{
			"cros-peripherals@google.com",
			"project-bolton@google.com",
		},
		Attr: []string{
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"chrome"}, // TODO(pmoy): Add "printscan" once this test runs on the lab scanners.
		VarDeps:      []string{"scanapp.Hardware.scannerOne"},
		Timeout:      30 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func Hardware(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	var scanners = []scanning.ScannerDescriptor{
		{
			ScannerName: s.RequiredVar("scanapp.Hardware.scannerOne"),
			SupportedSources: []scanning.SupportedSource{
				{
					SourceType:           scanapp.SourceFlatbed,
					SupportedColorModes:  []scanapp.ColorMode{scanapp.ColorModeColor, scanapp.ColorModeGrayscale},
					SupportedPageSizes:   []scanapp.PageSize{scanapp.PageSizeA4, scanapp.PageSizeLegal, scanapp.PageSizeLetter, scanapp.PageSizeFitToScanArea},
					SupportedResolutions: []scanapp.Resolution{scanapp.Resolution300DPI},
				},
				{
					SourceType:           scanapp.SourceADFOneSided,
					SupportedColorModes:  []scanapp.ColorMode{scanapp.ColorModeColor, scanapp.ColorModeGrayscale},
					SupportedPageSizes:   []scanapp.PageSize{scanapp.PageSizeA4, scanapp.PageSizeLegal, scanapp.PageSizeLetter, scanapp.PageSizeFitToScanArea},
					SupportedResolutions: []scanapp.Resolution{scanapp.Resolution300DPI},
				},
				{
					SourceType:           scanapp.SourceADFTwoSided,
					SupportedColorModes:  []scanapp.ColorMode{scanapp.ColorModeColor, scanapp.ColorModeGrayscale},
					SupportedPageSizes:   []scanapp.PageSize{scanapp.PageSizeA4, scanapp.PageSizeLegal, scanapp.PageSizeLetter, scanapp.PageSizeFitToScanArea},
					SupportedResolutions: []scanapp.Resolution{scanapp.Resolution300DPI},
				},
			},
		},
	}

	scanning.RunHardwareTests(ctx, s, cr, scanners)
}
