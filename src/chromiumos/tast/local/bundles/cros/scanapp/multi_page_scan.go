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
	"chromiumos/tast/local/chrome/ash"
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
		Func:         MultiPageScan,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the Scan app can be used to perform multi-page flatbed PDF scans",
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
		Fixture:      "virtualUsbPrinterModulesLoaded",
		Data: []string{
			scanning.SourceImage,
			singlePagePdfGoldenFile,
			twoPagePdfGoldenFile,
		},
	})
}

const (
	singlePagePdfGoldenFile = "multi_page_flatbed_single_page.pdf"
	twoPagePdfGoldenFile    = "multi_page_flatbed_two_page.pdf"
)

var multiPageScanTests = []struct {
	name       string
	removePage bool
	rescanPage bool
	goldenFile string
}{{
	name:       "multi_page_base",
	removePage: false,
	rescanPage: false,
	goldenFile: twoPagePdfGoldenFile,
}, {
	name:       "multi_page_remove_page",
	removePage: true,
	rescanPage: false,
	goldenFile: singlePagePdfGoldenFile,
}, {
	name:       "multi_page_rescan_page",
	removePage: false,
	rescanPage: true,
	goldenFile: twoPagePdfGoldenFile,
},
}

func MultiPageScan(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	crWithFeature, err := chrome.New(ctx, chrome.EnableFeatures("ScanAppMultiPageScan"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer crWithFeature.Close(cleanupCtx) // Close our own chrome instance
	cr := crWithFeature

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WithESCLCapabilities(scanning.EsclCapabilities),
		usbprinter.ExpectUdevEventOnStop(),
		usbprinter.WaitUntilConfigured())
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(cleanupCtx)
	if err := ippusbbridge.WaitForSocket(ctx, printer.DevInfo); err != nil {
		s.Fatal("Failed to wait for ippusb_bridge socket: ", err)
	}
	if err := cups.RestartPrintingSystem(ctx); err != nil {
		s.Fatal("Failed to restart printing system: ", err)
	}
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitMessageContains(scanning.ScannerName)); err != nil {
		s.Fatal("Failed to wait for printer notification: ", err)
	}
	if err := ippusbbridge.ContactPrinterEndpoint(ctx, printer.DevInfo, "/eSCL/ScannerCapabilities"); err != nil {
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

	if err := uiauto.Combine("set scan settings",
		app.SetScanSettings(scanapp.ScanSettings{
			Scanner:    scanning.ScannerName,
			Source:     scanapp.SourceFlatbed,
			FileType:   scanapp.FileTypePDF,
			ColorMode:  scanapp.ColorModeColor,
			PageSize:   scanapp.PageSizeLetter,
			Resolution: scanapp.Resolution300DPI,
		}),
		app.ClickMultiPageScanCheckbox(),
	)(ctx); err != nil {
		s.Fatal("Failed to set scan settings: ", err)
	}

	for _, test := range multiPageScanTests {
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_multi_page_scan")
			defer func() {
				if err := scanning.RemoveScans(scanning.DefaultScanPattern); err != nil {
					s.Error("Failed to remove scans: ", err)
				}
			}()

			// Make sure printer connected notifications don't cover the Scan button.
			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			// Start a multi-page scan session and scan 2 pages.
			if err := uiauto.Combine("multi-page scan",
				app.MultiPageScan( /*PageNumber=*/ 1),
				app.MultiPageScan( /*PageNumber=*/ 2),
			)(ctx); err != nil {
				s.Fatal("Failed to perform multi-page scan: ", err)
			}

			if test.removePage {
				if err := app.RemovePage()(ctx); err != nil {
					s.Fatal("Failed to remove page from scan: ", err)
				}
			}

			if test.rescanPage {
				if err := app.RescanPage()(ctx); err != nil {
					s.Fatal("Failed to rescan page in scan: ", err)
				}
			}

			// Click save to create the final PDF and compare it to the golden file.
			if err := uiauto.Combine("save scan",
				app.ClickSave(),
				app.ClickDone(),
			)(ctx); err != nil {
				s.Fatal("Failed to save scan scan: ", err)
			}

			scan, err := scanning.GetScan(scanning.DefaultScanPattern)
			if err != nil {
				s.Fatal("Failed to find scan: ", err)
			}

			diffPath := filepath.Join(s.OutDir(), "multi_page_scan_diff.txt")
			if err := document.CompareFiles(ctx, scan, s.DataPath(test.goldenFile), diffPath); err != nil {
				s.Error("Scan differs from golden file: ", err)
			}
		})
	}

	// Intentionally stop the printer early to trigger shutdown in
	// ippusb_bridge. Without this, cleanup may have to wait for other processes
	// to finish using the printer (e.g. CUPS background probing).
	//
	// TODO(b/210134772): Investigate if this remains necessary.
	if err := printer.Stop(cleanupCtx); err != nil {
		s.Error("Failed to stop printer: ", err)
	}
}
