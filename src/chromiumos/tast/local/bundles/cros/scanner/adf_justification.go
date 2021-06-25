// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/scanner/lorgnette"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ADFJustification,
		Desc:     "Tests that scanners with specified ADF justification values have correct scan regions",
		Contacts: []string{"kmoed@google.com", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	descriptors = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
	attributes  = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
)

type scannerParams struct {
	name             string
	justification    string
	esclCapabilities string
	expectedXOffset  int
}

func ADFJustification(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Set up the virtual USB printer.
	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := usbprinter.RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}(cleanupCtx)

	for _, params := range []scannerParams{{
		name:             "Left-Justified ADF Scanner",
		justification:    "Left",
		esclCapabilities: "/usr/local/etc/virtual-usb-printer/escl_capabilities_left_justified.json",
		expectedXOffset:  0,
	}, {
		name:             "Center-Justified ADF Scanner",
		justification:    "Center",
		esclCapabilities: "/usr/local/etc/virtual-usb-printer/escl_capabilities_center_justified.json",
		expectedXOffset:  673,
	}, {
		name:             "Right-Justified ADF Scanner",
		justification:    "Right",
		esclCapabilities: "/usr/local/etc/virtual-usb-printer/escl_capabilities_right_justified.json",
		expectedXOffset:  1358,
	}} {
		runJustificationTest(ctx, s, params)
	}

}

// runJustificationTest sets up the virtual usb printer and scan request according to specified params,
// performs a scan, and compares the result with the golden image from the params.
func runJustificationTest(ctx context.Context, s *testing.State, params scannerParams) {
	s.Log("Performing scan on ", params.name)

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	if err := cups.RestartPrintingSystem(ctx, devInfo); err != nil {
		s.Fatal("Failed to restart printing system: ", err)
	}

	tmpDir, err := ioutil.TempDir("", "tast.scanner.ADFJustification.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)

	printer, err := usbprinter.StartScanner(ctx, devInfo, descriptors, attributes, params.esclCapabilities, "", tmpDir)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if printer != nil {
			usbprinter.StopPrinter(ctx, printer, devInfo)
		}
	}()
	if err := cups.EnsurePrinterIdle(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for CUPS configuration: ", err)
	}

	scanPath := filepath.Join(tmpDir, "scanned.png")
	scanFile, err := os.Create(scanPath)
	if err != nil {
		s.Fatal("Failed to open scan output file: ", err)
	}

	// Requesting total width is 100 mm
	region := &lpb.ScanRegion{
		TopLeftX:     0,
		TopLeftY:     0,
		BottomRightX: 100,
		BottomRightY: 100,
	}

	deviceName := fmt.Sprintf("ippusb:escl:TestScanner:%s_%s/eSCL", devInfo.VID, devInfo.PID)
	startScanRequest := &lpb.StartScanRequest{
		DeviceName: deviceName,
		Settings: &lpb.ScanSettings{
			Resolution: 300,
			SourceName: "ADF",
			ColorMode:  lpb.ColorMode_MODE_COLOR,
			ScanRegion: region,
		},
	}

	l, err := lorgnette.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to lorgnette: ", err)
	}

	s.Log("Starting scan")
	startScanResponse, err := l.StartScan(ctx, startScanRequest)
	if err != nil {
		s.Fatal("Failed to call StartScan: ", err)
	}
	// Lorgnette was started automatically when we called StartScan, make sure to
	// close it when we exit.
	defer lorgnette.StopService(ctx)

	switch startScanResponse.State {
	case lpb.ScanState_SCAN_STATE_IN_PROGRESS:
		// Do nothing.
	case lpb.ScanState_SCAN_STATE_FAILED:
		s.Fatal("Failed to start scan: ", startScanResponse.FailureReason)
	default:
		s.Fatal("Unexpected ScanState: ", startScanResponse.State.String())
	}

	getNextImageRequest := &lpb.GetNextImageRequest{
		ScanUuid: startScanResponse.ScanUuid,
	}

	s.Log("Getting next image")
	getNextImageResponse, err := l.GetNextImage(ctx, getNextImageRequest, scanFile.Fd())
	if err != nil {
		s.Fatal("Failed to call GetNextImage: ", err)
	}

	if !getNextImageResponse.Success {
		s.Fatal("Failed to get next image: ", getNextImageResponse.FailureReason)
	}

	s.Log("Waiting for completion signal")
	if err = l.WaitForScanCompletion(ctx, startScanResponse.ScanUuid); err != nil {
		s.Fatal("Failed to wait for scan completion: ", err)
	}

	s.Log("Reading in scan settings log file")
	logFile, err := os.Open(tmpDir + "/scansettings_log.json")
	if err != nil {
		s.Fatal("Failed to open scan settings log file: ", err)
	}
	defer logFile.Close()

	var settingsDataStr string
	if err := json.NewDecoder(logFile).Decode(&settingsDataStr); err != nil {
		s.Fatal("Failed to decode log file: ", err)
	}

	var loggedSettings map[string]interface{}
	if err := json.Unmarshal([]byte(settingsDataStr), &loggedSettings); err != nil {
		s.Fatal("Failed to convert json settings string to dictionary: ", err)
	}
	var loggedRegion map[string]interface{}
	for _, r := range loggedSettings["Regions"].([]interface{}) {
		loggedRegion = r.(map[string]interface{})
	}

	s.Log("Comparing logged scan region to expected scan region")
	actualXOffset := int(loggedRegion["XOffset"].(float64))
	if params.expectedXOffset != actualXOffset {
		s.Fatal("Logged offset not equal to expected offset, expected: ", params.expectedXOffset, " actual: ", actualXOffset)
	}
}
