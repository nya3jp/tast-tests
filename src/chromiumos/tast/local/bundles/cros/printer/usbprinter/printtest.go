// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprinter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/printer/lp"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/testing"
)

func ippUsbPrinterURI(ctx context.Context, devInfo DevInfo) string {
	const ippUsbPrinterURIFormat = "ippusb://%s_%s/ipp/print"
	return fmt.Sprintf(ippUsbPrinterURIFormat, devInfo.VID, devInfo.PID)
}

func usbPrinterURI(ctx context.Context, devInfo DevInfo) string {
	const usbPrinterURIFormat = "usb://%s/%s"
	return fmt.Sprintf(usbPrinterURIFormat, devInfo.VID, devInfo.PID)
}

// waitPrinterConfigured waits for a printer which has the same VID/PID as
// devInfo to be configured on the system. If a match is found then the name of
// the configured device will be returned.
func waitPrinterConfigured(ctx context.Context, devInfo DevInfo) (string, error) {
	var foundName string
	uri := ippUsbPrinterURI(ctx, devInfo)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		name, err := lp.PrinterNameByURI(ctx, uri)
		if err != nil {
			return err
		}
		foundName = name
		return nil
	}, nil); err != nil {
		return "", err
	}
	return foundName, nil
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

	devInfo, err := LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	// Use oldContext for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	oldContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func() {
		if err := RemoveModules(oldContext); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}()

	printer, err := Start(ctx, devInfo, descriptors, attributes, record)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		printer.Kill()
		printer.Wait()
		// The recording file is created by the virtual printer on startup. If the
		// record file already exists then startup will fail. For this reason the
		// record file must be removed after the test has finished.
		if err := os.Remove(record); err != nil {
			s.Error("Failed to remove file: ", err)
		}
	}()

	var foundPrinterName string
	if ppd != "" {
		// If a PPD is provided then we configure the printer ourselves.
		foundPrinterName = "virtual-test"
		if err := lp.CupsAddPrinter(ctx, foundPrinterName, usbPrinterURI(ctx, devInfo), ppd); err != nil {
			s.Fatal("Failed to configure printer: ", err)
		}
	} else {
		// If no PPD is provided then the printer is an ipp-over-usb device and will
		// be configured automatically by Chrome. We wait until it is configured in
		// order to extract the name of the device.
		s.Log("Waiting for printer to be configured")
		foundPrinterName, err = waitPrinterConfigured(ctx, devInfo)
		if err != nil {
			s.Fatal("Failed to find printer name: ", err)
		}
	}
	s.Log("Printer configured with name: ", foundPrinterName)

	defer func() {
		// Regardless of whether the printer was added automatically by Chrome, or
		// explicitly by the test, it is safe to remove the printer using CUPS.
		if err := lp.CupsRemovePrinter(oldContext, foundPrinterName); err != nil {
			s.Error("Failed to remove printer: ", err)
		}
	}()

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

	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := compareFiles(ctx, record, golden, diffPath); err != nil {
		s.Error("Printed file differs from golden file: ", err)
	}
}
