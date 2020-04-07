// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBPrinterTimeout,
		Desc:         "Tests that USB print jobs timeout if the device does not exist",
		Contacts:     []string{"skau@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cups"},
		Data:         []string{"print_usb_ps.ppd.gz", "print_usb_to_print.pdf"},
	})
}

func usbPrinterURI(vid string, pid string) string {
	return fmt.Sprintf("usb://%s/%s", vid, pid)
}

func USBPrinterTimeout(ctx context.Context, s *testing.State) {
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	foundPrinterName := "broken_usb_printer"
	// Add a non-existant Kodak (0x040a) printer.
	if err := lp.CupsAddPrinter(ctx, foundPrinterName, usbPrinterURI("0x040a", "0xEEEE"), s.DataPath("print_usb_ps.ppd.gz")); err != nil {
		s.Fatal("Failed to configure printer: ", err)
	}
	s.Log("Printer configured with name: ", foundPrinterName)

	defer func() {
		// Regardless of whether the printer was added automatically by Chrome, or
		// explicitly by the test, it is safe to remove the printer using CUPS.
		if err := lp.CupsRemovePrinter(ctx, foundPrinterName); err != nil {
			s.Error("Failed to remove printer: ", err)
		}
	}()

	job, err := lp.CupsStartPrintJob(ctx, foundPrinterName, s.DataPath("print_usb_to_print.pdf"))
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}

	s.Log(ctx, " Waiting for ", job, " to complete")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		if done, err := lp.JobCompleted(ctx, foundPrinterName, job); err != nil {
			return err
		} else if !done {
			return errors.Errorf("Job %s is not done yet", job)
		}
		testing.ContextLogf(ctx, "Job %s is complete", job)
		return nil
	}, nil); err != nil {
		s.Fatal("Print job didn't complete: ", err)
	}
}
