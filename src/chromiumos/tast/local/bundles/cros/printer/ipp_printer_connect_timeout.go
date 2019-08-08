// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/printer/usbprinter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IppPrinterConnectTimeout,
		Desc:         "Checks that print jobs timeout if printer is unreachable",
		Contacts:     []string{"luum@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Pre:          chrome.LoggedIn(),
	})
}

// addBogusPrinter adds a fake printer to CUPS via lpadmin.
func addBogusPrinter(ctx context.Context, printerName string) error {
	uri := fmt.Sprintf("ipp://localhost/printers/%s", printerName)
	testing.ContextLog(ctx, "Adding bogus printer to CUPS using ", uri)
	return testexec.CommandContext(ctx, "lpadmin", "-p", printerName, "-v", uri, "-E").Run(testexec.DumpLogOnError)
}

// hasPrintJobTimedOut determines if |job| has timed out by confirming
// |printerName| is unresponsive and |job| has completed. On successful timeout,
// return nil.
func hasPrintJobTimedOut(ctx context.Context, printerName string, job string) error {
	// Check that |printerName| is reported as not responding.
	printerState := testexec.CommandContext(ctx, "lpstat", "-p", printerName)
	printerStateOut, err := printerState.Output()
	if err != nil {
		printerState.DumpLog(ctx)
		return err
	}
	if !strings.Contains(string(printerStateOut), "The printer is not responding.") {
		return errors.New("Printer is not unreachable")
	}

	// Check that timed-out |job| is completed.
	completedJobs := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o", printerName)
	completedJobsOut, err := completedJobs.Output()
	if err != nil {
		completedJobs.DumpLog(ctx)
		return err
	}
	if !strings.Contains(string(completedJobsOut), job) {
		return errors.New("Job never completed")
	}

	return nil
}

const (
	// Printer connection timeout, in seconds.
	timeout = 20

	// Fake printer name.
	printerName = "bogus_printer"

	// Arbitrary PDF file to print.
	toPrintFile = "to_print.pdf"
)

func IppPrinterConnectTimeout(ctx context.Context, s *testing.State) {
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	if err := addBogusPrinter(ctx, printerName); err != nil {
		s.Fatal("Failed to configure bogus printer: ", err)
	}

	job, err := usbprinter.CupsStartPrintJob(ctx, printerName, toPrintFile)
	if err != nil {
		s.Fatal("Failed to start print job: ", err)
	}

	// Wait out initial timeout, then poll for an additional timeout.
	testing.Sleep(ctx, timeout*time.Second)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return hasPrintJobTimedOut(ctx, printerName, job)
	}, &testing.PollOptions{Timeout: timeout * time.Second, Interval: 2 * time.Second}); err != nil {
		s.Fatal("Print Job failed to timeout: ", err)
	}
}
