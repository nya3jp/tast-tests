// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScanApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that scanned filed saved from Scan App appear in Holding Space",
		Contacts: []string{
			"angelsan@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

type virtualScannerParams struct {
	name             string
	esclCapabilities string
}

var settings = scanapp.ScanSettings{
	Source:     scanapp.SourceFlatbed,
	FileType:   scanapp.FileTypePNG,
	ColorMode:  scanapp.ColorModeColor,
	PageSize:   scanapp.PageSizeLetter,
	Resolution: scanapp.Resolution300DPI,
}

// ScanApp tests the functionality of files existing in Holding Space by
// saving a scanned file from the Scan app.
func ScanApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Reset the holding space.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceOptions{}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	params := virtualScannerParams{
		name:             "Center-Justified ADF Scanner",
		esclCapabilities: "/usr/local/etc/virtual-usb-printer/escl_capabilities_center_justified.json",
	}
	s.Log("Performing scan on ", params.name)

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WithESCLCapabilities(params.esclCapabilities),
		usbprinter.ExpectUdevEventOnStop(),
		usbprinter.WaitUntilConfigured())
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(ctx)
	if err = ippusbbridge.WaitForSocket(ctx, printer.DevInfo); err != nil {
		s.Fatal("Failed to wait for ippusb_bridge socket: ", err)
	}
	if err = cups.RestartPrintingSystem(ctx); err != nil {
		s.Fatal("Failed to reset printing system: ", err)
	}
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitMessageContains(printer.VisibleName)); err != nil {
		s.Fatal("Failed to wait for printer notification: ", err)
	}
	if err = ippusbbridge.ContactPrinterEndpoint(ctx, printer.DevInfo, "/eSCL/ScannerCapabilities"); err != nil {
		s.Fatal("Failed to get scanner status over ippusb_bridge socket: ", err)
	}

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users MyFiles path: ", err)
	}
	pattern := "scan*_*.*"
	defaultScanPattern := filepath.Join(myFilesPath, pattern)
	// Remove scans after the test completes.
	defer func() {
		scans, err := filepath.Glob(defaultScanPattern)
		if err != nil {
			s.Fatal("Failed to remove scans: ", err)
		}

		for _, scan := range scans {
			if err = os.Remove(scan); err != nil {
				s.Fatalf("Failed to remove %s", scan)
			}
		}
	}()

	// Launch the Scan app, configure the settings, perform a scan.
	s.Log("Launching Scan app")
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
	defer func() {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}()

	// Make sure printer connected notifications don't cover the Scan button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	s.Log("Starting scan")
	scanSettings := settings
	scanSettings.Scanner = printer.VisibleName
	if err := uiauto.Combine("scan",
		app.ClickMoreSettings(),
		app.SetScanSettings(scanSettings),
		app.Scan(),
	)(ctx); err != nil {
		s.Fatal("Failed to perform scan: ", err)
	}

	// Verify there is a scanned file.
	scans, err := filepath.Glob(defaultScanPattern)
	if err != nil {
		s.Fatal("Failed to find scanned file: ", err)
	}
	if len(scans) != 1 {
		s.Fatalf("Found too many scans: got %v; want 1", len(scans))
	}

	// Verify the scan can be found in the holding space.
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("open holdingspace and verify scanned png file appears in holding space",
		ui.LeftClick(holdingspace.FindTray()),
		ui.WaitUntilExists(holdingspace.FindDownloadChip().Name(filepath.Base(scans[0]))),
	)(ctx); err != nil {
		s.Fatal("Failed to open holdingspace and verify scanned png file appears in holding space: ", err)
	}
}
