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
// produces the expected output at record when a print job is issued.
// toPrint and golden specify the paths to the file to be printed and the
// expected printer output.
func RunPrintTest(ctx context.Context, s *testing.State,
	opts []usbprinter.Option, record, ppd, toPrint, golden string) {

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	pr, err := usbprinter.Start(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := pr.Stop(ctx); err != nil {
			s.Error("Failed to stop virtual printer: ", err)
		}
		// Remove the recorded file. Ignore errors if the path doesn't exist.
		if err := os.Remove(record); err != nil && !os.IsNotExist(err) {
			s.Error("Failed to remove file: ", err)
		}
	}(ctx)

	// If no PPD was provided, then the printer is an IPP-over-USB device
	// for which we waited on autoconf; the name will have been stored in
	// the PrinterInstance.
	var foundPrinterName = pr.ConfiguredName
	if ppd != "" {
		// If a PPD is provided then we configure the printer ourselves.
		foundPrinterName = "virtual-test"
		if err := lp.CupsAddPrinter(ctx, foundPrinterName, usbPrinterURI(ctx, pr.DevInfo), ppd); err != nil {
			s.Fatal("Failed to configure printer: ", err)
		}
	}
	s.Log("Printer configured with name: ", foundPrinterName)

	job, err := lp.CupsStartPrintJob(ctx, foundPrinterName, toPrint)
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}

	s.Logf("Waiting for %s to complete", job)
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
