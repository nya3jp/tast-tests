// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/printing/lp"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     USBPrinterTimeout,
		Desc:     "Tests that USB print jobs timeout if the device does not exist",
		Contacts: []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{"print_usb_ps.ppd.gz", "print_usb_to_print.pdf"},
	})
}

func usbPrinterURI(vid, pid string) string {
	return fmt.Sprintf("usb://%s/%s", vid, pid)
}

func USBPrinterTimeout(ctx context.Context, s *testing.State) {
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	foundPrinterName := "broken_usb_printer"
	// Add a non-existent Kodak (0x040a) printer.
	if err := lp.CupsAddPrinter(ctx, foundPrinterName, usbPrinterURI("0x040a", "0xEEEE"), s.DataPath("print_usb_ps.ppd.gz")); err != nil {
		s.Fatal("Failed to configure printer: ", err)
	}
	s.Log("Printer configured with name: ", foundPrinterName)

	logReader, readerErr := syslog.NewReader(ctx, syslog.Program(syslog.Cupsd))
	if readerErr != nil {
		s.Fatal("Failed to start log reader: ", readerErr)
	}
	defer logReader.Close()

	job, err := lp.CupsStartPrintJob(ctx, foundPrinterName, s.DataPath("print_usb_to_print.pdf"))
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}

	s.Logf("Waiting for %s to complete", job)
	var logEntry *syslog.Entry
	re := regexp.MustCompile(`\[Job \d+\] PID \d+ \(/usr/libexec/cups/backend/usb\) .*`)
	// The USB backend can take up to 20 seconds to timeout so wait for up to 30.
	logEntry, err = logReader.Wait(ctx, time.Duration(30)*time.Second, func(entry *syslog.Entry) bool {
		return re.MatchString(entry.Content)
	})
	if err != nil {
		s.Fatal("Print job never completed: ", err)
	}

	// It's expected that the usb backend exited with "stopped on status 1" indicating a timeout because the printer was unreachable.  Statuses containing "crashed" or any other status are considered failures.
	if strings.Contains(logEntry.Content, "crashed") {
		s.Fatal("USB Backend crashed: " + logEntry.Content)
	} else if !strings.Contains(logEntry.Content, "stopped with status 1") {
		s.Fatal("USB Backend exited with an unrecognized error: " + logEntry.Content)
	}
}
