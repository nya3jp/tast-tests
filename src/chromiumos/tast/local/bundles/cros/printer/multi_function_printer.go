// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/printer/usbprintertests"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/local/scanner/lorgnette"
	"chromiumos/tast/local/usbutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiFunctionPrinter,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests printer/scanner/storage combo device",
		Contacts:     []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"paper-io_scanning",
			"informational",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Data:         []string{printSource, printGolden, printPPD, scanSource, scanGolden},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

const (
	printSource = "print_ippusb_to_print.pdf"
	printGolden = "print_ippusb_golden.pdf"
	printPPD    = "printer_add_generic_printer_GenericPostScript.ppd.gz"
	scanSource  = "scan_escl_ipp_source.jpg"
	scanGolden  = "scan_escl_ipp_golden.png"

	// These are the expected names of kernel drivers attached to a particular
	// USB interface. There are also two special values:
	//   *  skip, meaning do not verify that interface.  This is useful for
	//      an interface that may change over time or is not of interest.
	//   *  none, meaning that the interface is explicitly expected to have no driver attached.
	skipDriverVerification = "skip"
	usbStorageDriver       = "usb-storage"
	usbNoDriver            = "none"
)

var classDrivers = map[string]string{
	"07": skipDriverVerification, // Printer.  Skip verification because the test deliberately changes it.
	"08": usbStorageDriver,       // Storage
	"ff": usbNoDriver,            // Vendor
}

func verifyPrinterInterfaces(ctx context.Context, dev usbprinter.DevInfo) error {
	devices, err := usbutil.AttachedDevices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to enumerate attached USB devices")
	}

	for _, d := range devices {
		if d.VendorID != dev.VID || d.ProdID != dev.PID {
			continue
		}

		for _, i := range d.Interfaces {
			expectedDriver, ok := classDrivers[i.Class]
			if !ok {
				return errors.Errorf("class driver mapping for interface %d class %s not found", i.InterfaceNumber, i.Class)
			}
			if expectedDriver == skipDriverVerification {
				continue
			}

			if i.Driver == nil {
				if expectedDriver != usbNoDriver {
					return errors.Errorf("interface %d class driver %s is nil; want %s", i.InterfaceNumber, i.Class, expectedDriver)
				}
			} else {
				if *i.Driver != expectedDriver {
					return errors.Errorf("interface %d class driver %s is %s; want %s", i.InterfaceNumber, i.Class, *i.Driver, expectedDriver)
				}
			}
		}
	}

	return nil
}

// MultiFunctionPrinter interacts with a printer-scanner-storage MFP through IPP-USB printing,
// USB printer class printing, and IPP-USB scanning.  It verifies that the different access types
// don't interfere with each other, and confirms that the non-printing storage interfaces are
// left alone during printing/scanning.
func MultiFunctionPrinter(ctx context.Context, s *testing.State) {
	tmpDir, err := ioutil.TempDir("", "tast.printer.MultiFunctionPrinter.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)
	recordPath := filepath.Join(tmpDir, "printed.pdf")

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	pr, err := usbprinter.Start(ctx, []usbprinter.Option{
		usbprinter.WithDescriptors("ippusb_printer_plus_storage.json"),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WithRecordPath(recordPath),
		usbprinter.WithESCLCapabilities("escl_capabilities.json"),
		usbprinter.WaitUntilConfigured(),
	}...)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := pr.Stop(ctx); err != nil {
			s.Error("Failed to stop virtual printer: ", err)
		}
		// Remove the recorded file. Ignore errors if the path doesn't exist.
		if err := os.Remove(recordPath); err != nil && !os.IsNotExist(err) {
			s.Error("Failed to remove file: ", err)
		}
	}(ctx)

	foundPrinterName := pr.ConfiguredName
	s.Log("Printer configured with name: ", foundPrinterName)

	if err := verifyPrinterInterfaces(ctx, pr.DevInfo); err != nil {
		s.Fatal("Printer interfaces are not correct: ", err)
	}

	s.Log("Sending IPP-USB print job")
	usbprintertests.RunPrintJob(ctx, s, foundPrinterName, usbprintertests.PrintJobSetup{
		ToPrint:     s.DataPath(printSource),
		PrintedFile: recordPath,
		GoldenFile:  s.DataPath(printGolden)})

	if err := verifyPrinterInterfaces(ctx, pr.DevInfo); err != nil {
		s.Fatal("Printer interfaces are not correct: ", err)
	}

	usbPrinterName := "usbtest-" + foundPrinterName
	usbURI := fmt.Sprintf("usb://%s/%s", pr.DevInfo.VID, pr.DevInfo.PID)
	lp.CupsAddPrinter(ctx, usbPrinterName, usbURI, s.DataPath(printPPD))

	s.Log("Sending USB printer class job")
	usbprintertests.RunPrintJob(ctx, s, usbPrinterName, usbprintertests.PrintJobSetup{
		ToPrint:     s.DataPath(printSource),
		PrintedFile: recordPath,
		GoldenFile:  s.DataPath(printGolden)})

	if err := verifyPrinterInterfaces(ctx, pr.DevInfo); err != nil {
		s.Fatal("Printer interfaces are not correct: ", err)
	}

	scannerDevice := fmt.Sprintf("ippusb:escl:TestScanner:%s_%s/eSCL", pr.DevInfo.VID, pr.DevInfo.PID)
	startScanRequest := &lpb.StartScanRequest{
		DeviceName: scannerDevice,
		Settings: &lpb.ScanSettings{
			Resolution: 300,
			SourceName: "Flatbed",
			ColorMode:  lpb.ColorMode_MODE_COLOR,
		},
	}

	s.Log("Performing scan from ", scannerDevice)
	scanPath, err := lorgnette.RunScan(ctx, startScanRequest, tmpDir)
	if err != nil {
		s.Fatal("Failed to run scan: ", err)
	}

	s.Log("Comparing scanned file to golden image")
	diff := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", "1", scanPath, s.DataPath(scanGolden))
	if err := diff.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Scanned file differed from golden image: ", err)
	}

	if err := verifyPrinterInterfaces(ctx, pr.DevInfo); err != nil {
		s.Fatal("Printer interfaces are not correct: ", err)
	}
}
