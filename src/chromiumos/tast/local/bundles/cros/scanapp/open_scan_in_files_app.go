// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/scanapp/scanning"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenScanInFilesApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that a scan can be opened in the Files app",
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
		Data:         []string{scanning.SourceImage},
	})
}

var settings = scanapp.ScanSettings{
	Source:     scanapp.SourceFlatbed,
	FileType:   scanapp.FileTypePNG,
	ColorMode:  scanapp.ColorModeColor,
	PageSize:   scanapp.PageSizeLetter,
	Resolution: scanapp.Resolution300DPI,
}

func OpenScanInFilesApp(ctx context.Context, s *testing.State) {
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
	defaultScanPattern := filepath.Join(myFilesPath, scanning.DefaultScanFilePattern)
	// Remove scans after the test completes.
	defer func() {
		if err := scanning.RemoveScans(defaultScanPattern); err != nil {
			s.Error("Failed to remove scans: ", err)
		}
	}()

	// Launch the Scan app, configure the settings, perform a scan, and open the
	// scan in the Files app.
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
		app.ClickMyFilesLink(),
	)(ctx); err != nil {
		s.Fatal("Failed to perform scan: ", err)
	}

	// Verify the scan can be found in the Files app.
	scan, err := scanning.GetScan(defaultScanPattern)
	if err != nil {
		s.Fatal("Failed to find scan: ", err)
	}

	_, file := filepath.Split(scan)

	f, err := filesapp.App(ctx, tconn, apps.Files.ID)
	if err != nil {
		s.Fatal("Failed to get Files app: ", err)
	}

	s.Logf("Searching for %s in Files app: ", file)
	if err := f.WaitForFile(file)(ctx); err != nil {
		s.Fatal("Failed to find scan in Files app: ", err)
	}

	if err := f.Close(ctx); err != nil {
		s.Fatal("Failed to close Files app: ", err)
	}
}
