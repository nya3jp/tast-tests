// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanning provides methods and constants commonly used for scanning.
package scanning

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

const (
	// ScannerName is the name of the virtual USB scanner.
	ScannerName = "DavieV Virtual USB Printer (USB)"

	// SourceImage is the image used to configure the virtual USB scanner.
	SourceImage = "scan_source.jpg"

	// Attributes is the path to the attributes used to configure the virtual
	// USB scanner.
	Attributes = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	// Descriptors is the path to the descriptors used to configure the virtual
	// USB scanner.
	Descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
	// EsclCapabilities is the path to the capabilities used to configure the
	// virtual USB scanner.
	EsclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"

	// DefaultScanPattern is the pattern used to find files in the default
	// scan-to location.
	DefaultScanPattern = filesapp.MyFilesPath + "/scan*_*.*"
)

// GetScan returns the filepath of the scanned file found using pattern.
func GetScan(pattern string) (string, error) {
	scans, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(scans) != 1 {
		return "", errors.Errorf("found too many scans: got %v; want 1", len(scans))
	}

	return scans[0], nil
}

// RemoveScans removes all of the scanned files found using pattern.
func RemoveScans(pattern string) error {
	scans, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, scan := range scans {
		if err = os.Remove(scan); err != nil {
			return errors.Wrapf(err, "failed to remove %s", scan)
		}
	}

	return nil
}

// TestingStruct contains the parameters used when testing the scanapp settings
// in RunAppSettingsTests.
type TestingStruct struct {
	Name       string
	Settings   scanapp.ScanSettings
	GoldenFile string
}

// ScannerStruct contains the necessary parameters for setting up the virtual usb printer.
type ScannerStruct struct {
	Descriptors     string
	Attributes      string
	EsclCaps        string
	SourceImagePath string
}

// RunAppSettingsTests takes in the Chrome instance and the specific testing parameters
// and performs the test, including attaching the virtual USB printer, launching
// the scanapp, clicking through the settings, and verifying proper image output.
func RunAppSettingsTests(ctx context.Context, s *testing.State, cr *chrome.Chrome, testParams []TestingStruct, scannerParams ScannerStruct) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	devInfo, err := usbprinter.LoadPrinterIDs(scannerParams.Descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", scannerParams.Descriptors, err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, scannerParams.Descriptors, scannerParams.Attributes, scannerParams.EsclCaps, scannerParams.SourceImagePath, "")
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
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitMessageContains(ScannerName)); err != nil {
		s.Fatal("Failed to wait for printer notification: ", err)
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

	for _, test := range testParams {
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+test.Name)
			defer func() {
				if err := RemoveScans(DefaultScanPattern); err != nil {
					s.Error("Failed to remove scans: ", err)
				}
			}()

			// Make sure printer connected notifications don't cover the Scan button.
			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			if err := uiauto.Combine("scan",
				app.SetScanSettings(test.Settings),
				app.Scan(),
				app.ClickDone(),
			)(ctx); err != nil {
				s.Fatal("Failed to perform scan: ", err)
			}

			scan, err := GetScan(DefaultScanPattern)
			if err != nil {
				s.Fatal("Failed to find scan: ", err)
			}

			diffPath := filepath.Join(s.OutDir(), test.Name+"_diff.txt")
			if err := document.CompareFiles(ctx, scan, s.DataPath(test.GoldenFile), diffPath); err != nil {
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
