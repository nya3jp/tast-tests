// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/printpreview"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Print,
		Desc: "Check that ARC++ printing is working properly",
		Contacts: []string{
			"bmgordon@google.com",
			"project-bolton@google.com",
		},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome", "cups", "virtual_usb_printer"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val:               "arc_print_ippusb_golden.pdf",
			ExtraHardwareDeps: hwdep.D(hwdep.Model(stableModels...)),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraData:         []string{"arc_print_ippusb_golden.pdf"},
		}, {
			Name:              "unstable",
			Val:               "arc_print_ippusb_golden.pdf",
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(stableModels...)),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraData:         []string{"arc_print_ippusb_golden.pdf"},
		}, {
			Name:              "vm",
			Val:               "arc_print_vm_ippusb_golden.pdf",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData:         []string{"arc_print_vm_ippusb_golden.pdf"},
		}},
	})
}

// stableModels is a list of models stable enough for the CQ.
var stableModels = []string{
	"chell",

	// dedede:
	"boten",
	"drawlat",
	"magolor",

	"drallion",
	"drallion360",
	"gandof",

	// hatch:
	"akemi",
	"dratini",
	"jinlon",
	"kindred",

	// octopus:
	"blooguard",
	"blorb",
	"bobba360",
	"casta",
	"droid",
	"foob",
	"garfour",
	"grabbiter",
	"meep",
	"nospike",
	"phaser",
	"sparky360",
	"vortininja",

	// puff:
	"duffy",
	"puff",
	"wyvern",

	"reks",
	"relm",
	"samus",

	// sarien:
	"arcada",

	"sentry",
	"ultima",

	// volteer:
	"delbin",
	"eldrid",
	"voxel",

	// zork:
	"vilboz14",
}

func Print(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcPrintTest.apk"
		pkgName      = "org.chromium.arc.testapp.print"
		activityName = "MainActivity"
		descriptors  = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		attributes   = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Install virtual USB printer.
	s.Log("Installing printer")
	tmpDir, err := ioutil.TempDir("", "tast.printer.PrintIPPUSB.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)
	recordPath := filepath.Join(tmpDir, "record.pdf")

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	// Use oldContext for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	oldContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func() {
		if err := usbprinter.RemoveModules(oldContext); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}()

	printer, _, err := usbprinter.StartIPPUSB(ctx, devInfo, descriptors, attributes, recordPath)
	if err != nil {
		s.Fatal("Failed to start IPP-over-USB printer: ", err)
	}
	defer func() {
		printer.Kill()
		printer.Wait()
		if err := os.Remove(recordPath); err != nil && !os.IsNotExist(err) {
			s.Error("Failed to remove file: ", err)
		}
	}()

	// Install ArcPrintTest app.
	s.Log("Installing ArcPrintTest app")
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install ArcPrintTest app: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting MainActivity")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start MainActivity: ", err)
	}

	// Maximize the app window to ensure print preview is visible. Since the
	// test can often pass without the window being maximized, log errors
	// without failing the test.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, pkgName, ash.WMEventMaximize); err != nil {
		s.Log("Failed to set app window to Maximized state: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, pkgName, ash.WindowStateMaximized); err != nil {
		s.Log("Failed to wait for app window to enter Maximized state: ", err)
	}

	// Wait for print preview to load before selecting a printer.
	s.Log("Waiting for print preview to load")
	printpreview.WaitForPrintPreview(tconn)(ctx)

	// Select printer.
	s.Log("Selecting printer")
	const printerName = "DavieV Virtual USB Printer (USB) DavieV Virtual USB Printer (USB)"
	if err := printpreview.SelectPrinter(ctx, tconn, printerName); err != nil {
		s.Fatal("Failed to select printer: ", err)
	}

	// Wait for print preview to load before changing settings.
	s.Log("Waiting for print preview to load")
	printpreview.WaitForPrintPreview(tconn)(ctx)

	s.Log("Changing print settings")

	// Set layout to landscape.
	if err = printpreview.SetLayout(ctx, tconn, printpreview.Landscape); err != nil {
		s.Fatal("Failed to set layout: ", err)
	}

	// Set custom page selection.
	if err = printpreview.SetPages(ctx, tconn, "2-3,5,7-10"); err != nil {
		s.Fatal("Failed to select pages: ", err)
	}

	// Hide all notifications to prevent them from covering the print button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
	}

	// Wait for print preview to load before starting the print job.
	s.Log("Waiting for print preview to load")
	printpreview.WaitForPrintPreview(tconn)(ctx)

	// Click the print button to start the print job.
	s.Log("Clicking print button")
	if err = printpreview.Print(ctx, tconn); err != nil {
		s.Fatal("Failed to print: ", err)
	}

	s.Log("Waiting for print job to complete")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return errors.New("Print job has not completed yet")
		}
		testing.ContextLog(ctx, "Print job has completed")
		return nil
	}, nil); err != nil {
		s.Fatal("Print job failed to complete: ", err)
	}

	golden := s.DataPath(s.Param().(string))
	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := document.CompareFiles(ctx, recordPath, golden, diffPath); err != nil {
		s.Error("Printed file differs from golden file: ", err)
	}
}
