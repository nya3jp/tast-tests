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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/printpreview"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func waitForNotificationHidden(ctx context.Context, root *ui.Node) error {
	params := ui.FindParams{
		Name: "USB printer connected",
		Role: ui.RoleTypeStaticText,
	}
	if err := root.WaitForDescendant(ctx, params, false, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for notification to be hidden")
	}
	return nil
}

func waitForPrintPreview(ctx context.Context, root *ui.Node) error {
	params := ui.FindParams{
		Name: "Loading preview",
	}
	// Wait for the loading text to appear to indicate print preview has
	// launched and is loading. There should be a sufficient amount of time
	// between the text appearing and being removed, but the test may fail here
	// if the text is removed too quickly.
	if err := root.WaitForDescendant(ctx, params, true, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to find loading text")
	}
	// Wait for the loading text to be removed to indicate print preview is loaded.
	if err := root.WaitForDescendant(ctx, params, false, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for loading text to be removed")
	}
	return nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Print,
		Desc: "Check that ARC printing is working properly",
		Contacts: []string{
			"bmgordon@google.com",
			"jschettler@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome", "cups", "virtual_usb_printer"},
		Data:         []string{"arc_print_ippusb_golden.pdf"},
		Pre:          arc.Booted(),
	})
}

func Print(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcPrintTest.apk"
		pkgName      = "org.chromium.arc.testapp.print"
		activityName = "MainActivity"
		descriptors  = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		attributes   = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
	)

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
		// The record file is created by the virtual printer on startup. If the
		// record file already exists then startup will fail. For this reason,
		// the record file must be removed after the test has finished.
		if err := os.Remove(recordPath); err != nil {
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
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start MainActivity: ", err)
	}

	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get UI root: ", err)
	}
	defer root.Release(ctx)

	// Wait for print preview to load before selecting a printer.
	s.Log("Waiting for print preview to load")
	if err := waitForPrintPreview(ctx, root); err != nil {
		s.Fatal("Failed to wait for print preview to load: ", err)
	}

	// Select printer.
	s.Log("Selecting printer")
	const printerName = "DavieV Virtual USB Printer (USB) DavieV Virtual USB Printer (USB)"
	if err := printpreview.SelectPrinter(ctx, root, printerName); err != nil {
		s.Fatal("Failed to select printer: ", err)
	}

	s.Log("Changing print settings")

	// Set layout to landscape.
	if err = printpreview.SetLayout(ctx, root, printpreview.Landscape); err != nil {
		s.Fatal("Failed to set layout: ", err)
	}

	// Set custom page selection.
	if err = printpreview.SetPages(ctx, root, "2-5,10-15,36,49-50"); err != nil {
		s.Fatal("Failed to select pages: ", err)
	}

	// Wait for the printer notification to be hidden since it overlaps the
	// print button.
	if err := waitForNotificationHidden(ctx, root); err != nil {
		s.Fatal("Failed to close printer notification: ", err)
	}

	// Click the print button to start the print job.
	s.Log("Clicking print button")
	if err = printpreview.Print(ctx, root); err != nil {
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

	golden := s.DataPath("arc_print_ippusb_golden.pdf")
	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := document.CompareFiles(ctx, recordPath, golden, diffPath); err != nil {
		s.Error("Printed file differs from golden file: ", err)
	}
}
