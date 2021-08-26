// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/scanapp/scanning"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
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

var testSetups = []struct {
	name       string
	settings   scanapp.ScanSettings
	goldenFile string
}{{
	name: "paper_size_a3",
	settings: scanapp.ScanSettings{
		Scanner:    scanning.ScannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA3,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: a3GoldenFile,
}, {
	name: "paper_size_a4",
	settings: scanapp.ScanSettings{
		Scanner:    scanning.ScannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeA4,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: a4GoldenFile,
}, {
	name: "paper_size_b4",
	settings: scanapp.ScanSettings{
		Scanner:    scanning.ScannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeB4,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: b4GoldenFile,
}, {
	name: "paper_size_legal",
	settings: scanapp.ScanSettings{
		Scanner:    scanning.ScannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLegal,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: legalGoldenFile,
}, {
	name: "paper_size_letter",
	settings: scanapp.ScanSettings{
		Scanner:    scanning.ScannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLetter,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: letterGoldenFile,
}, {
	name: "paper_size_tabloid",
	settings: scanapp.ScanSettings{
		Scanner:    scanning.ScannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeTabloid,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: tabloidGoldenFile,
},
}

func LargePaperScans(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Set up the virtual USB printer.
	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := usbprinter.RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}(cleanupCtx)

	devInfo, err := usbprinter.LoadPrinterIDs(scanning.Descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", scanning.Descriptors, err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, scanning.Descriptors, scanning.Attributes, esclCapabilities, s.DataPath(scanning.SourceImage), "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if printer != nil {
			usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
		}
	}()
	if err = ippusbbridge.WaitForSocket(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for ippusb_bridge socket: ", err)
	}
	if err = cups.EnsurePrinterIdle(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for printer to be idle: ", err)
	}
	if err = ippusbbridge.ContactPrinterEndpoint(ctx, devInfo, "/eSCL/ScannerCapabilities"); err != nil {
		s.Fatal("Failed to get scanner status over ippusb_bridge socket: ", err)
	}

	// Launch the Scan app, configure the settings, and perform scans.
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}

	if err := app.ClickMoreSettings()(ctx); err != nil {
		s.Fatal("Failed to expand More settings: ", err)
	}

	for _, test := range testSetups {
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+test.name)
			defer func() {
				if err := scanning.RemoveScans(scanning.DefaultScanPattern); err != nil {
					s.Error("Failed to remove scans: ", err)
				}
			}()

			if err := uiauto.Combine("scan",
				app.SetScanSettings(test.settings),
				app.Scan(),
				app.ClickDone(),
			)(ctx); err != nil {
				s.Fatal("Failed to perform scan: ", err)
			}

			scan, err := scanning.GetScan(scanning.DefaultScanPattern)
			if err != nil {
				s.Fatal("Failed to find scan: ", err)
			}

			diffPath := filepath.Join(s.OutDir(), test.name+"_diff.txt")
			if err := document.CompareFiles(ctx, scan, s.DataPath(test.goldenFile), diffPath); err != nil {
				s.Error("Scan differs from golden file: ", err)
			}
		})
	}

	// Intentionally stop the printer early to trigger shutdown in
	// ippusb_bridge. Without this, cleanup may have to wait for other processes
	// to finish using the printer (e.g. CUPS background probing).
	usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
	printer = nil
}
