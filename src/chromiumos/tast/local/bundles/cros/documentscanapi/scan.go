// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Scan,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that a scan can be performed using the Document Scan API",
		Contacts:     []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Data:         []string{"manifest.json", "background.js", "scan.css", "scan.html", "scan.js", "scan_escl_ipp_source.jpg", "scan_escl_ipp_golden.png"},
		SoftwareDeps: []string{"chrome", "virtual_usb_printer"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_scanning",
		},
		Fixture: "virtualUsbPrinterModulesLoaded",
		Params: []testing.Param{
			{
				Val: browser.TypeAsh,
			},
			{
				Name:              "lacros",
				Val:               browser.TypeLacros,
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

const esclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"

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

	bt := s.Param().(browser.Type)
	var extOpt []chrome.Option
	if bt == browser.TypeLacros {
		extOpt = append(extOpt, chrome.LacrosUnpackedExtension(extDir))
	} else {
		extOpt = append(extOpt, chrome.UnpackedExtension(extDir))
	}
	cr, err := browserfixt.NewChrome(ctx, bt, lacrosfixt.NewConfig(), extOpt...)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatal("Failed to launch browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WithESCLCapabilities(esclCapabilities),
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
		s.Fatal("Failed to wait for ippusb socket: ", err)
	}
	if err = ippusbbridge.ContactPrinterEndpoint(ctx, printer.DevInfo, "/eSCL/ScannerCapabilities"); err != nil {
		s.Fatal("Failed to get scanner status over ippusb_bridge socket: ", err)
	}

	extURL := "chrome-extension://" + scanTargetExtID + "/scan.html"
	conn, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL(extURL))
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

	scanButton := nodewith.Name("Scan").Role(role.Button).Ancestor(nodewith.Name("Scanner Control").HasClass("RootView").First())

	if err := uiauto.Combine("wait for scan button",
		ui.WithTimeout(10*time.Second).WaitUntilExists(scanButton),
	)(ctx); err != nil {
		s.Fatal("Scan button failed to appear: ", err)
	}

	if err := uiauto.Combine("click button and wait",
		ui.DoDefault(scanButton),
		ui.WithTimeout(30*time.Second).WaitUntilGone(scanButton),
	)(ctx); err != nil {
		s.Fatal("Failed to perform scan: ", err)
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
