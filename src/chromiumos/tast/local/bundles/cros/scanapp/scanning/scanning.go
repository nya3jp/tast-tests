// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package scanning provides methods and constants commonly used for scanning.
package scanning

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
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
	// SourceImage is the image used to configure the virtual USB scanner.
	SourceImage = "scan_source.jpg"

	// Attributes is the path to the attributes used to configure the virtual
	// USB scanner.
	Attributes = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"

	// Descriptors is the path to the descriptors used to configure the virtual
	// USB scanner with default settings.
	Descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"

	// FlipTestDescriptors is the path to the descriptors used to configure the virtual
	// USB scanner with settings that will trigger duplex back page rotation in Chrome.
	FlipTestDescriptors = "/usr/local/etc/virtual-usb-printer/ippusb_backflip_printer.json"

	// EsclCapabilities is the path to the capabilities used to configure the
	// virtual USB scanner.
	EsclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"

	// DefaultScanPattern is the pattern used to find files in the default
	// scan-to location.
	DefaultScanPattern = filesapp.MyFilesPath + "/scan*_*.*"

	// MMPerInch is the conversion factor from inches to mm.
	MMPerInch = 25.4
)

// HardwareTestMode controls how the hardware tests are run.
type HardwareTestMode int

const (
	// HardwareTestRunAllCombinations tests each combination of color mode, page
	// size and resolution.
	HardwareTestRunAllCombinations HardwareTestMode = iota
	// HardwareTestRunRandomizedCombinations tests the minimal number of
	// combinations necessary to test each color mode, page size and resolution
	// at least once. Combinations will be randomized each run.
	HardwareTestRunRandomizedCombinations
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

// generateSettingsLists adjusts the input color modes, page sizes and
// resolutions depending on the given `mode`. More specifically:
// HardwareTestRunAllCombinations: no adjustment necessary.
// HardwareTestRunRandomizedCombinations: shuffle each input array.
func generateSettingsLists(mode HardwareTestMode, colorModes []scanapp.ColorMode, pageSizes []scanapp.PageSize, resolutions []scanapp.Resolution) ([]scanapp.ColorMode, []scanapp.PageSize, []scanapp.Resolution, error) {
	switch mode {
	case HardwareTestRunAllCombinations:
		// No need to do anything here.
	case HardwareTestRunRandomizedCombinations:
		rand.Shuffle(len(colorModes), func(i, j int) {
			colorModes[i], colorModes[j] = colorModes[j], colorModes[i]
		})
		rand.Shuffle(len(pageSizes), func(i, j int) {
			pageSizes[i], pageSizes[j] = pageSizes[j], pageSizes[i]
		})
		rand.Shuffle(len(resolutions), func(i, j int) {
			resolutions[i], resolutions[j] = resolutions[j], resolutions[i]
		})
	default:
		return nil, nil, nil, errors.Errorf("unknown HardwareTestMode: %d", mode)
	}

	return colorModes, pageSizes, resolutions, nil
}

// calculateNumScans returns the minimum number of scans necessary to test the
// given `mode`.
func calculateNumScans(mode HardwareTestMode, numColorModes, numPageSizes, numResolutions int) (int, error) {
	switch mode {
	case HardwareTestRunAllCombinations:
		return numColorModes * numPageSizes * numResolutions, nil
	case HardwareTestRunRandomizedCombinations:
		numScans := numColorModes

		if numPageSizes > numScans {
			numScans = numPageSizes
		}

		if numResolutions > numScans {
			numScans = numResolutions
		}

		return numScans, nil
	default:
		return -1, errors.Errorf("unknown HardwareTestMode: %d", mode)
	}
}

// getNextScanCombination returns the scan dimensions for the next scan
// depending on the scan number `numScan` and test mode `mode`.
func getNextScanCombination(mode HardwareTestMode, numScan int, colorModes []scanapp.ColorMode, pageSizes []scanapp.PageSize, resolutions []scanapp.Resolution) (colorMode scanapp.ColorMode, pageSize scanapp.PageSize, resolution scanapp.Resolution, err error) {
	switch mode {
	case HardwareTestRunAllCombinations:
		// Iterate through each possible combination. Every scan combination
		// advances the resolution index once (rolling over when it reaches the
		// end of the resolutions slice). When the resolution index rolls over,
		// increment the page size index. Similarly, when the page size index
		// rolls over, increment the color modes index.
		colorMode = colorModes[numScan/(len(pageSizes)*len(resolutions))]
		pageSize = pageSizes[(numScan/len(resolutions))%len(pageSizes)]
		resolution = resolutions[numScan%len(resolutions)]
	case HardwareTestRunRandomizedCombinations:
		// For randomized combinations, advance each slice index once (rolling
		// over when it reaches the end of the slice).
		colorMode = colorModes[numScan%len(colorModes)]
		pageSize = pageSizes[numScan%len(pageSizes)]
		resolution = resolutions[numScan%len(resolutions)]
	default:
		err = errors.Errorf("unknown HardwareTestMode: %d", mode)
	}

	return
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

// getScannerURI runs `lorgnette_cli list` and parses the output to find the URI
// for the scanner with `name`. The function prefers HTTPS URIs over HTTP.
func getScannerURI(ctx context.Context, name string) (string, error) {
	out, err := testexec.CommandContext(ctx, "lorgnette_cli", "list").Output()
	if err != nil {
		return "", err
	}

	r, err := regexp.Compile(fmt.Sprintf(`^airscan:escl:%s:(?P<uri>.*/eSCL/)$`, regexp.QuoteMeta(name)))
	if err != nil {
		return "", err
	}

	uri := ""
	lines := strings.Split(string(out), "\n")
Loop:
	for _, line := range lines {
		match := r.FindStringSubmatch(line)
		if match == nil || len(match) < 2 {
			continue
		}

		for i, submatch := range r.SubexpNames() {
			if submatch == "uri" {
				uri = match[i]
				if strings.HasPrefix(uri, "https") {
					break Loop
				}
			}
		}
	}

	if uri == "" {
		return "", errors.Errorf("failed to parse output: %s", string(out))
	}

	return uri, nil
}

// getScannerStatus queries the ScannerStatus endpoint for the scanner with eSCL
// URI `uri` and returns the scanner's status.
func getScannerStatus(uri string) (string, error) {
	// Deliberately ignore certificate errors because printers normally
	// have self-signed certificates.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
	}

	response, err := client.Get(uri + "ScannerStatus")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.Status != "200 OK" {
		return "", errors.Errorf("unexpected HTTP response status: %s", response.Status)
	}

	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var scannerStatus ScannerStatus
	err = xml.Unmarshal(bytes, &scannerStatus)
	if err != nil {
		return "", err
	}

	return scannerStatus.State, nil
}

// ensureScannerIdle ensures that the scanner with the given URI `uri` reports
// Idle as its status.
func ensureScannerIdle(ctx context.Context, uri string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		status, err := getScannerStatus(uri)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get scanner status"))
		}
		if status != "Idle" {
			return errors.Errorf("scanner status is: %s", status)
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 1 * time.Second})
}

// ScannerStatus structures the data returned by a call to a scanner's
// /eSCL/ScannerStatus endpoint. Unused data from the response is left out of
// the struct.
type ScannerStatus struct {
	State string `xml:"State"`
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
	s.Logf("Started virtual printer: %s", printer.VisibleName)
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
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitMessageContains(printer.VisibleName)); err != nil {
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
		settings := test.Settings
		settings.Scanner = printer.VisibleName
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
				app.SetScanSettings(settings),
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
				saveScanPath := filepath.Join(s.OutDir(), test.Name+filepath.Ext(scan))
				if err := fsutil.MoveFile(scan, saveScanPath); err != nil {
					s.Error("Unable to preserve scanned file output: ", err)
				}
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
func RunHardwareTests(ctx context.Context, s *testing.State, cr *chrome.Chrome, scanner ScannerDescriptor, mode HardwareTestMode) {
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	uri, err := getScannerURI(ctx, scanner.ScannerName)
	if err != nil {
		s.Fatal("Failed to get scanner URI: ", err)
	}

	// Make sure the scanner is idle so enumeration will succeed.
	if err := ensureScannerIdle(ctx, uri); err != nil {
		s.Fatal("Scanner not idle: ", err)
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

	if mode == HardwareTestRunRandomizedCombinations {
		rand.Seed(time.Now().UnixNano())
	}

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

		colorModes, pageSizes, resolutions, err := generateSettingsLists(mode, source.SupportedColorModes, source.SupportedPageSizes, source.SupportedResolutions)
		if err != nil {
			s.Fatal("Failed to adjust scan dimensions: ", err)
		}

		numScans, err := calculateNumScans(mode, len(colorModes), len(pageSizes), len(resolutions))
		if err != nil {
			s.Fatal("Failed to calculate number of scans: ", err)
		}

		for i := 0; i < numScans; i++ {
			colorMode, pageSize, resolution, err := getNextScanCombination(mode, i, colorModes, pageSizes, resolutions)
			if err != nil {
				s.Fatal("Failed to get next scan combination: ", err)
			}

			if err := app.SelectColorMode(colorMode)(ctx); err != nil {
				s.Fatalf("Failed to select color mode: %s: %v", colorMode, err)
			}

			if err := app.SelectPageSize(pageSize)(ctx); err != nil {
				s.Fatalf("Failed to select page size: %s: %v", pageSize, err)
			}

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

			if err := ensureScannerIdle(ctx, uri); err != nil {
				s.Fatal("Scanner not idle: ", err)
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
