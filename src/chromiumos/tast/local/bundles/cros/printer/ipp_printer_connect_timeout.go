// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/printer/lp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPPPrinterConnectTimeout,
		Desc:         "Checks that print jobs timeout if printer is unreachable",
		Contacts:     []string{"luum@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"to_print.pdf"},
		Pre:          chrome.LoggedIn(),
	})
}

// printJobTimedOut determines if |job| has timed out by confirming
// |printerName| is unresponsive and |job| has completed. On successful timeout,
// return nil.
func printJobTimedOut(ctx context.Context, printerName, job string) error {
	// Check that |printerName| is reported as not responding.
	if printerReachable, err := lp.PrinterUnreachable(ctx, printerName); err != nil {
		return err
	} else if !printerReachable {
		return errors.New("printer is not unreachable")
	}

	// Check that timed-out |job| is completed.
	if completed, err := lp.JobCompleted(ctx, printerName, job); err != nil {
		return err
	} else if !completed {
		return errors.New("job never completed")
	}

	return nil
}

func IPPPrinterConnectTimeout(ctx context.Context, s *testing.State) {
	const (
		// Printer connection timeout.
		timeout = 20 * time.Second

		// Fake printer name.
		printerName = "bogus_printer"
	)

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	uri := "ipp://localhost/printers/" + printerName
	if err := lp.CupsAddDriverlessPrinter(ctx, printerName, uri); err != nil {
		s.Fatal("Failed to configure bogus printer: ", err)
	}

	job, err := lp.CupsStartPrintJob(ctx, printerName, s.DataPath("to_print.pdf"))
	if err != nil {
		s.Fatal("Failed to start print job: ", err)
	}

	// Since CUPS will will take at least |timeout| seconds before
	// declaring a printer unreachable, we sleep for a |timeout| before
	// polling.
	testing.Sleep(ctx, timeout)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return printJobTimedOut(ctx, printerName, job)
	}, &testing.PollOptions{Timeout: timeout, Interval: 2 * time.Second}); err != nil {
		s.Fatal("Print Job failed to timeout: ", err)
	}
}
