// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package documentscanapi

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Scan,
		Desc:         "Tests that a scan can be performed using the Document Scan API",
		Contacts:     []string{"kmoed@google.com", "project-bolton@google.com"},
		Data:         []string{"manifest.json", "background.js", "scan.css", "scan.html", "scan.js", "scan_escl_ipp_source.jpg", "scan_escl_ipp_golden.png"},
		SoftwareDeps: []string{"chrome", "virtual_usb_printer"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_scanning",
		},
	})
}

const (
	descriptors      = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
	attributes       = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	esclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"
)

// Scan tests the chrome.documentScan API.
func Scan(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	extDir, err := ioutil.TempDir("", "tast.documentscanapi.Scan.")
	if err != nil {
		s.Fatal("Failed to create temp extension dir: ", err)
	}
	defer os.RemoveAll(extDir)

	scanTargetExtID, err := setUpDocumentScanExtension(ctx, s, extDir)
	if err != nil {
		s.Fatal("Failed setup of Document Scan extension: ", err)
	}

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
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

	printer, err := usbprinter.StartScanner(ctx, devInfo, descriptors, attributes, esclCapabilities, s.DataPath("scan_escl_ipp_source.jpg"), "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if printer != nil {
			usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
		}
	}()
	if err = ippusbbridge.WaitForSocket(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for ippusb socket: ", err)
	}
	if err = cups.EnsurePrinterIdle(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for printer to be idle: ", err)
	}
	if err = ippusbbridge.ContactPrinterEndpoint(ctx, devInfo, "/eSCL/ScannerCapabilities"); err != nil {
		s.Fatal("Failed to get scanner status over ippusb_bridge socket: ", err)
	}

	extURL := "chrome-extension://" + scanTargetExtID + "/scan.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(extURL))
	if err != nil {
		s.Fatalf("Failed to connect to extension URL at %v: %v", extURL, err)
	}
	defer conn.Close()

	// APIs are not immediately available to extensions: https://crbug.com/789313.
	s.Log("Waiting for chrome.documentScan API to become available")
	if err := conn.WaitForExprFailOnErr(ctx, "chrome.documentScan"); err != nil {
		s.Fatal("chrome.documentScan API unavailable: ", err)
	}

	s.Log("Clicking Scan button")
	ui := uiauto.New(tconn)
	scanButton := nodewith.Name("Scan").Role(role.Button)
	if err := ui.WithInterval(1000*time.Millisecond).LeftClickUntil(scanButton, ui.Gone(scanButton))(ctx); err != nil {
		s.Fatal("Failed to click Scan button: ", err)
	}

	s.Log("Extracting scanned image")
	var imageSource string
	if err := conn.Eval(ctx, "document.getElementById('scannedImage').src", &imageSource); err != nil {
		s.Fatal("Failed to get image source: ", err)
	}

	base64ImageHeader := "data:image/png;base64,"
	if !strings.HasPrefix(imageSource, base64ImageHeader) {
		s.Fatal("Image source does not start with Base64 data header")
	}

	base64Image := strings.TrimPrefix(imageSource, base64ImageHeader)
	imageData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		s.Fatal("Failed to decode image source: ", err)
	}

	scanPath := filepath.Join(extDir, "scanned.png")
	scanFile, err := os.Create(scanPath)
	if err != nil {
		s.Fatal("Failed to open scan output file: ", err)
	}

	if _, err := scanFile.Write(imageData); err != nil {
		s.Fatal("Failed to write out image file: ", err)
	}

	s.Log("Comparing image to golden")
	diff := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", "1", scanPath, s.DataPath("scan_escl_ipp_golden.png"))
	if err := diff.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Scanned file differed from golden image: ", err)
		diff.DumpLog(ctx)
	}
}

// setUpDocumentScanExtension moves the extension files into the extension directory and returns extension ID.
func setUpDocumentScanExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
	for _, name := range []string{"manifest.json", "background.js", "scan.html", "scan.js", "scan.css"} {
		if err := fsutil.CopyFile(s.DataPath(name), filepath.Join(extDir, name)); err != nil {
			return "", errors.Wrapf(err, "failed to copy file %q: %v", name, err)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %q: %v", extDir, err)
	}

	return extID, nil
}
