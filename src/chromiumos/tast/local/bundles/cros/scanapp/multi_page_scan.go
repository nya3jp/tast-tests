// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/printing/document"
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

	printer, err := scanapp.StartPrinter(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(cleanupCtx)

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
			Scanner:    printer.VisibleName,
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

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users MyFiles path: ", err)
	}
	defaultScanPattern := filepath.Join(myFilesPath, scanapp.DefaultScanFilePattern)
	for _, test := range multiPageScanTests {
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_multi_page_scan")
			defer func() {
				if err := scanapp.RemoveScans(defaultScanPattern); err != nil {
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

			scan, err := scanapp.GetScan(defaultScanPattern)
			if err != nil {
				s.Fatal("Failed to find scan: ", err)
			}

			diffPath := filepath.Join(s.OutDir(), "multi_page_scan_diff.txt")
			if err := document.CompareFiles(ctx, scan, s.DataPath(test.goldenFile), diffPath); err != nil {
				s.Error("Scan differs from golden file: ", err)
			}
		})
	}
}
