// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanning provides methods and constants commonly used for scanning.
package scanning

import (
	"context"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
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

	// MMPerInch is the conversion factor from inches to mm.
	MMPerInch = 25.4
)

// identifyOutputRegex parses out the width, height and colorspace from the
// output of `identify someImage`.
var identifyOutputRegex = regexp.MustCompile(`^.+ PNG (?P<width>[0-9]+)x(?P<height>[0-9]+).+ 8-bit (?P<colorspace>sRGB|Gray 256c|Gray 2c)`)

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

// calculateNumScans returns the minimum number of scans necessary to test each
// option at least once.
func calculateNumScans(numColorModes, numPageSizes, numResolutions int) int {
	numScans := numColorModes

	if numPageSizes > numScans {
		numScans = numPageSizes
	}

	if numResolutions > numScans {
		numScans = numResolutions
	}

	return numScans
}

// toIdentifyColorspace converts from `colorMode` to the colorspace output by
// `identify someImage`.
func toIdentifyColorspace(colorMode scanapp.ColorMode) (string, error) {
	switch colorMode {
	case scanapp.ColorModeBlackAndWhite:
		return "Gray 2c", nil
	case scanapp.ColorModeGrayscale:
		return "Gray 256c", nil
	case scanapp.ColorModeColor:
		return "sRGB", nil
	default:
		return "", errors.Errorf("Unable to convert color mode: %v to identify colorspace", colorMode)
	}
}

// calculateExpectedDimensions returns the expected height and width in pixels
// for an image of size `pageSize` and resolution `resolution`.
func calculateExpectedDimensions(pageSize scanapp.PageSize, resolution scanapp.Resolution, sourceDimensions SourceDimensions) (expectedHeight, expectedWidth float64, err error) {
	var heightMM float64
	var widthMM float64
	switch pageSize {
	case scanapp.PageSizeA3:
		widthMM = 297
		heightMM = 420
	case scanapp.PageSizeA4:
		widthMM = 210
		heightMM = 297
	case scanapp.PageSizeB4:
		widthMM = 257
		heightMM = 364
	case scanapp.PageSizeLegal:
		widthMM = 215.9
		heightMM = 355.6
	case scanapp.PageSizeLetter:
		widthMM = 215.9
		heightMM = 279.4
	case scanapp.PageSizeTabloid:
		widthMM = 279.4
		heightMM = 431.8
	case scanapp.PageSizeFitToScanArea:
		widthMM = sourceDimensions.WidthMM
		heightMM = sourceDimensions.HeightMM
	default:
		return -1, -1, errors.Errorf("Unrecognized page size: %v", pageSize)
	}

	resInt, err := resolution.ToInt()
	if err != nil {
		return -1, -1, err
	}

	return heightMM / MMPerInch * float64(resInt), widthMM / MMPerInch * float64(resInt), nil
}

// calculateAcceptablePixelDifference returns the allowable threshold for a
// scanned image's actual size versus its theoretical size. This threshold is
// 0.25mm with a minimum of 1 to account for rounding.
func calculateAcceptablePixelDifference(resolution scanapp.Resolution) (float64, error) {
	resInt, err := resolution.ToInt()
	if err != nil {
		return -1, err
	}

	return math.Max(1.0, 0.25/MMPerInch*float64(resInt)), nil
}

// verifyScannedImage verifies that the scanned image with location `scan` has
// the correct width, height and resolution as reported by `identify`.
func verifyScannedImage(ctx context.Context, scan string, pageSize scanapp.PageSize, resolution scanapp.Resolution, colorMode scanapp.ColorMode, sourceDimensions SourceDimensions) error {
	cmd := testexec.CommandContext(ctx, "identify", scan)
	identifyBytes, err := cmd.Output()
	if err != nil {
		return err
	}

	expectedHeight, expectedWidth, err := calculateExpectedDimensions(pageSize, resolution, sourceDimensions)
	if err != nil {
		return err
	}

	match := identifyOutputRegex.FindStringSubmatch(string(identifyBytes))
	if match == nil || len(match) < 4 {
		return errors.Errorf("Unable to parse identify output: %s", string(identifyBytes))
	}

	threshold, err := calculateAcceptablePixelDifference(resolution)
	if err != nil {
		return errors.Wrap(err, "Unable to calculate threshold")
	}

	for i, name := range identifyOutputRegex.SubexpNames() {
		if name == "width" {
			width, err := strconv.Atoi(match[i])

			if err != nil {
				return err
			}

			if math.Abs(expectedWidth-float64(width)) > threshold {
				return errors.Errorf("Width: got %d, expected %f", width, expectedWidth)
			}
		}

		if name == "height" {
			height, err := strconv.Atoi(match[i])

			if err != nil {
				return err
			}

			if math.Abs(expectedHeight-float64(height)) > threshold {
				return errors.Errorf("Height: got %d, expected %f", height, expectedHeight)
			}
		}

		if name == "colorspace" {
			colorSpace, err := toIdentifyColorspace(colorMode)
			if err != nil {
				return err
			}

			if colorSpace != match[i] {
				return errors.Errorf("Colorspace: got %s, expected %s", match[i], colorSpace)
			}
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
	Descriptors string
	Attributes  string
	EsclCaps    string
}

// SourceDimensions contain the height and width of a scan source, in mm.
type SourceDimensions struct {
	HeightMM float64 `json:"Height"`
	WidthMM  float64 `json:"Width"`
}

// SupportedSource describes the options supported by a particular scan source.
type SupportedSource struct {
	SourceType           scanapp.Source       `json:"SourceType"`
	SupportedColorModes  []scanapp.ColorMode  `json:"ColorModes"`
	SupportedPageSizes   []scanapp.PageSize   `json:"PageSizes"`
	SupportedResolutions []scanapp.Resolution `json:"Resolutions"`
	// SourceDimensions only needs to be present for flatbed sources. They can
	// be determined by running `lorgnette_cli get_json_caps --scanner=$SCANNER`
	// for any particular scanner. Note that the flatbed and ADF sources often
	// have different dimensions - make sure to choose the dimension of the
	// flatbed source.
	SourceDimensions SourceDimensions `json:"SourceDimensions"`
}

// ScannerDescriptor contains the parameters used to test the scan app on real
// hardware.
type ScannerDescriptor struct {
	ScannerName      string            `json:"Name"`
	SupportedSources []SupportedSource `json:"Sources"`
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

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithDescriptors(scannerParams.Descriptors),
		usbprinter.WithAttributes(scannerParams.Attributes),
		usbprinter.WithESCLCapabilities(scannerParams.EsclCaps),
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
		s.Fatal("Failed to restart printing system: ", err)
	}
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitMessageContains(ScannerName)); err != nil {
		s.Fatal("Failed to wait for printer notification: ", err)
	}
	if err = ippusbbridge.ContactPrinterEndpoint(ctx, printer.DevInfo, "/eSCL/ScannerCapabilities"); err != nil {
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
	//
	// TODO(b/210134772): Investigate if this remains necessary.
	if err := printer.Stop(cleanupCtx); err != nil {
		s.Error("Failed to stop printer: ", err)
	}
}

// RunHardwareTests tests that the scan app can select each of the options
// provided by `scanner`. This function is intended to be run on real hardware,
// not the virtual USB printer.
func RunHardwareTests(ctx context.Context, s *testing.State, cr *chrome.Chrome, scanner ScannerDescriptor) {
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	app, err := scanapp.LaunchWithPollOpts(ctx, testing.PollOptions{Interval: 300 * time.Millisecond, Timeout: 1 * time.Minute}, tconn)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}

	if err := app.ClickMoreSettings()(ctx); err != nil {
		s.Fatal("Failed to expand More settings: ", err)
	}

	// Loop through all of the supported options. Skip file type for now, since
	// that is not a property of the scanners themselves and we're not
	// performing any real scans.
	if err := app.SelectScanner(scanner.ScannerName)(ctx); err != nil {
		s.Fatalf("Failed to select scanner: %s: %v", scanner.ScannerName, err)
	}
	// Sleep to allow the supported sources to load and stabilize.
	// TODO(b/211712633): Once there is a way to verify the selection of a
	// listbox, add that logic to app.SelectSource() and remove this sleep.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep after selecting scanner: ", err)
	}

	rand.Seed(time.Now().UnixNano())

	for _, source := range scanner.SupportedSources {
		s.Log("Testing source: ", source.SourceType)

		if err := app.SelectSource(source.SourceType)(ctx); err != nil {
			s.Fatalf("Failed to select source: %s: %v", source.SourceType, err)
		}
		// Sleep to allow the source-specific options to load and stabilize.
		// TODO(b/211712633): Once there is a way to verify the selecion of a
		// listbox, add that logic to app.SelectColorMode(),
		// app.SelectPageSize(), app.SelectResolution() and remove this sleep.
		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			s.Fatal("Failed to sleep after selecting source: ", err)
		}

		// For flatbed sources, perform scans with randomized settings
		// combinations until each setting has been tested at least once.
		if source.SourceType == scanapp.SourceFlatbed {
			defer func() {
				if err := RemoveScans(DefaultScanPattern); err != nil {
					s.Error("Failed to remove scans: ", err)
				}
			}()

			if err := app.SelectFileType(scanapp.FileTypePNG)(ctx); err != nil {
				s.Fatal("Failed to select file type: ", scanapp.FileTypePNG)
			}
		}

		rand.Shuffle(len(source.SupportedColorModes), func(i, j int) {
			source.SupportedColorModes[i], source.SupportedColorModes[j] = source.SupportedColorModes[j], source.SupportedColorModes[i]
		})
		rand.Shuffle(len(source.SupportedPageSizes), func(i, j int) {
			source.SupportedPageSizes[i], source.SupportedPageSizes[j] = source.SupportedPageSizes[j], source.SupportedPageSizes[i]
		})
		rand.Shuffle(len(source.SupportedResolutions), func(i, j int) {
			source.SupportedResolutions[i], source.SupportedResolutions[j] = source.SupportedResolutions[j], source.SupportedResolutions[i]
		})

		numScans := calculateNumScans(len(source.SupportedColorModes), len(source.SupportedPageSizes), len(source.SupportedResolutions))
		for i := 0; i < numScans; i++ {
			colorMode := source.SupportedColorModes[i%len(source.SupportedColorModes)]
			if err := app.SelectColorMode(colorMode)(ctx); err != nil {
				s.Fatalf("Failed to select color mode: %s: %v", colorMode, err)
			}

			pageSize := source.SupportedPageSizes[i%len(source.SupportedPageSizes)]
			if err := app.SelectPageSize(pageSize)(ctx); err != nil {
				s.Fatalf("Failed to select page size: %s: %v", pageSize, err)
			}

			resolution := source.SupportedResolutions[i%len(source.SupportedResolutions)]
			if err := app.SelectResolution(resolution)(ctx); err != nil {
				s.Fatalf("Failed to select resolution: %s, %v", resolution, err)
			}

			if source.SourceType != scanapp.SourceFlatbed {
				continue
			}

			// Make sure printer connected notifications don't cover the
			// Scan button.
			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			s.Logf("Testing scan combination: {%v %v %v}", colorMode, pageSize, resolution)

			if err := uiauto.Combine("scan",
				app.Scan(),
				app.ClickDone(),
			)(ctx); err != nil {
				s.Fatal("Failed to perform scan: ", err)
			}

			scan, err := GetScan(DefaultScanPattern)
			if err != nil {
				s.Fatal("Failed to find scan: ", err)
			}

			err = verifyScannedImage(ctx, scan, pageSize, resolution, colorMode, source.SourceDimensions)
			if err != nil {
				s.Error("Failed to verify scanned image: ", err)
			}

			err = RemoveScans(DefaultScanPattern)
			if err != nil {
				s.Error("Failed to remove scans: ", err)
			}
		}
	}
}
