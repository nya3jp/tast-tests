// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/cryptohome"
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

	printer, err := scanapp.StartPrinter(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(ctx)

	settings.Scanner = printer.VisibleName

	app, err := scanapp.LaunchAndStartScanWithSettings(ctx, tconn, settings)
	if err != nil {
		s.Fatal("Failed to Launch scan app and start scan: ", err)
	}
	defer func() {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}()

	if err := uiauto.Combine("Launch Files App by clicking My Files link",
		app.ClickMyFilesLink(),
	)(ctx); err != nil {
		s.Fatal("Failed to open Files App: ", err)
	}

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users MyFiles path: ", err)
	}
	defaultScanPattern := filepath.Join(myFilesPath, scanapp.DefaultScanFilePattern)
	// Remove scans after the test completes.
	defer func() {
		if err := scanapp.RemoveScans(defaultScanPattern); err != nil {
			s.Error("Failed to remove scans: ", err)
		}
	}()

	// Verify the scan can be found in the Files app.
	scan, err := scanapp.GetScan(defaultScanPattern)
	if err != nil {
		s.Fatal("Failed to find scan: ", err)
	}

	_, file := filepath.Split(scan)

	f, err := filesapp.App(ctx, tconn, apps.FilesSWA.ID)
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
