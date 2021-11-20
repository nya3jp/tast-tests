// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbprintertests provides utility functions for running tests with a
// USB printer.
package usbprintertests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func usbPrinterURI(ctx context.Context, devInfo usbprinter.DevInfo) string {
	return fmt.Sprintf("usb://%s/%s", devInfo.VID, devInfo.PID)
}

// RunPrintTest executes a test for the virtual USB printer defined by the
// given arguments. This tests that the printer is able to be configured, and
// produces the expected output when a print job is issued. The given
// descriptors and attributes provide the virtual printer with paths to the USB
// descriptors and IPP attributes files respectively. record defines the path
// where the printer output should be written. toPrint and golden specify the
// paths to the file to be printed and the expected printer output.
func RunPrintTest(ctx context.Context, s *testing.State, descriptors,
	attributes, record, ppd, toPrint, golden string) {

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	printer, err := usbprinter.StartXXXkdlee(ctx,
		usbprinter.PrinterInfo{Descriptors: descriptors,
			Attributes: attributes,
			RecordPath: record,
			// Without a PPD, we assume the device is an
			// IPP-USB printer that undergoes autoconf.
			WaitUntilConfigured: len(ppd) == 0})
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		printer.Stop(ctx /*expectUdevEvent=*/, false)
		// Remove the recorded file. Ignore errors if the path doesn't exist.
		if err := os.Remove(record); err != nil && !os.IsNotExist(err) {
			s.Error("Failed to remove file: ", err)
		}
	}()

	// If no PPD is provided then the printer is an ipp-over-usb device and will
	// be configured automatically by Chrome. `Start()` waited until it was
	// configured before returning and gives us the device name.
	var foundPrinterName = printer.ConfiguredName
	if ppd != "" {
		// If a PPD is provided then we configure the printer ourselves.
		foundPrinterName = "virtual-test"
		if err := lp.CupsAddPrinter(
			ctx,
			foundPrinterName,
			usbPrinterURI(ctx, printer.DevInfo),
			ppd); err != nil {
			s.Fatal("Failed to configure printer: ", err)
		}
	}
	s.Log("Printer configured with name: ", foundPrinterName)

	job, err := lp.CupsStartPrintJob(ctx, foundPrinterName, toPrint)
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}

	s.Log(ctx, " Waiting for ", job, " to complete")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		if done, err := lp.JobCompleted(ctx, foundPrinterName, job); err != nil {
			return err
		} else if !done {
			return errors.Errorf("job %s is not done yet", job)
		}
		testing.ContextLogf(ctx, "Job %s is complete", job)
		return nil
	}, nil); err != nil {
		s.Fatal("Print job didn't complete: ", err)
	}
	goldenData, err := ioutil.ReadFile(golden)
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}
	output, err := ioutil.ReadFile(record)
	if err != nil {
		s.Fatal("Failed to read output file: ", err)
	}
	if document.CleanContents(string(goldenData)) != document.CleanContents(string(output)) {
		outFile := filepath.Base(golden)
		outPath := filepath.Join(s.OutDir(), outFile)
		if err := ioutil.WriteFile(outPath, output, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Printer output differs from expected: output saved to %q", outFile)
	}
}
