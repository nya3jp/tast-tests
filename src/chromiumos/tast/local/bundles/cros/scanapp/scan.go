// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/scanapp"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Scan,
		Desc: "Tests that the Scan app can be used to perform scans",
		Contacts: []string{
			"jschettler@chromium.org",
			"cros-peripherals@google.com",
			"project-bolton@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "virtual_usb_printer"},
		Data: []string{
			sourceImage,
			pngGoldenFile,
			jpgGoldenFile,
			pdfGoldenFile,
		},
	})
}

const defaultScanPattern = filesapp.MyFilesPath + "/scan*_*.*"

const scannerName = "DavieV Virtual USB Printer (USB)"

const (
	sourceImage      = "scan_source.jpg"
	pngGoldenFile    = "flatbed_png_color_letter_300_dpi.png"
	jpgGoldenFile    = "adf_simplex_jpg_grayscale_a4_150_dpi.jpg"
	pdfGoldenFile    = "adf_duplex_pdf_grayscale_max_300_dpi.pdf"
	descriptors      = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
	attributes       = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	esclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"
)

var tests = []struct {
	name       string
	settings   scanapp.ScanSettings
	goldenFile string
}{{
	name: "flatbed_png_color_letter_300_dpi",
	settings: scanapp.ScanSettings{
		Scanner:    scannerName,
		Source:     scanapp.SourceFlatbed,
		FileType:   scanapp.FileTypePNG,
		ColorMode:  scanapp.ColorModeColor,
		PageSize:   scanapp.PageSizeLetter,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: pngGoldenFile,
}, {
	name: "adf_simplex_jpg_grayscale_a4_150_dpi",
	settings: scanapp.ScanSettings{
		Scanner:  scannerName,
		Source:   scanapp.SourceADFOneSided,
		FileType: scanapp.FileTypeJPG,
		// TODO(jschettler): Change this to black and white when the virtual
		// USB printer correctly reports the color mode.
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeA4,
		Resolution: scanapp.Resolution150DPI,
	},
	goldenFile: jpgGoldenFile,
}, {
	name: "adf_duplex_pdf_grayscale_max_300_dpi",
	settings: scanapp.ScanSettings{
		Scanner:    scannerName,
		Source:     scanapp.SourceADFTwoSided,
		FileType:   scanapp.FileTypePDF,
		ColorMode:  scanapp.ColorModeGrayscale,
		PageSize:   scanapp.PageSizeFitToScanArea,
		Resolution: scanapp.Resolution300DPI,
	},
	goldenFile: pdfGoldenFile,
}}

func getScan(pattern string) (string, error) {
	scans, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(scans) != 1 {
		return "", errors.New("found too many scans")
	}

	return scans[0], nil
}

func removeScans(pattern string) error {
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

func Scan(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Create a Chrome instance with the Scan app enabled.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ScanningUI"))
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

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, descriptors, attributes, esclCapabilities, s.DataPath(sourceImage))
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if printer != nil {
			usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
		}
	}()
	defer ippusbbridge.Kill(cleanupCtx, devInfo)

	// Launch the Scan app, configure the settings, and perform scans.
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}

	if err := app.ClickMoreSettings()(ctx); err != nil {
		s.Fatal("Failed to expand More settings: ", err)
	}

	for _, test := range tests {
		s.Run(ctx, test.name, func(ctx context.Context, s *testing.State) {
			defer func() {
				if err := removeScans(defaultScanPattern); err != nil {
					s.Error("Failed to remove scans: ", err)
				}
			}()

			if err := uiauto.Run(ctx,
				app.SetScanSettings(test.settings),
				app.Scan(),
				app.ClickDone(),
			); err != nil {
				info, _ := uiauto.RootDebugInfo(ctx, tconn)
				s.Log(info)
				s.Fatal("Failed to perform scan: ", err)
			}

			scan, err := getScan(defaultScanPattern)
			if err != nil {
				s.Fatal("Failed to find scan: ", err)
			}

			diffPath := filepath.Join(s.OutDir(), test.name, "_diff.txt")
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
