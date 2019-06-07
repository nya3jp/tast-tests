// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprinter

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Regular expression used to match a line from the output of the lpstat
// command. The values for "vid" and "pid" must be filled in before attempting
// to compile it.
const lpstatPatternFormat = `device for (?P<name>usb-[a-f0-9]+): ippusb://%s_%s/ipp/print`

// printerName runs the lpstat command to search for a configured printer
// which has the an address that matches lpstatPattern and has the same vid/pid
// as in devInfo. Return the name of the matching printer if found.
func printerName(ctx context.Context, devInfo DevInfo) (name string, err error) {
	out, err := testexec.CommandContext(ctx, "lpstat", "-v").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run scan for configured printers")
	}

	lpstatPattern := fmt.Sprintf(lpstatPatternFormat, regexp.QuoteMeta(devInfo.VID), regexp.QuoteMeta(devInfo.PID))
	r := regexp.MustCompile(lpstatPattern)

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		submatches := r.FindStringSubmatch(line)
		if submatches == nil {
			continue
		}
		return submatches[1], nil
	}

	return "", errors.Errorf("failed to find printer with vid %s pid %s", devInfo.VID, devInfo.PID)
}

// cupsRemovePrinter removes the printer that was configured for testing.
func cupsRemovePrinter(ctx context.Context, printerName string) error {
	return testexec.CommandContext(ctx, "lpadmin", "-x", printerName).Run()
}

// cupsStartPrintJob starts a new print job for the file toPrint. Returns the ID
// of the newly created job if successful.
func cupsStartPrintJob(ctx context.Context, printerName string, toPrint string) (job string, err error) {
	lp := testexec.CommandContext(ctx, "lp", "-d", printerName, "--", toPrint)
	testing.ContextLog(ctx, "Starting print job")
	output, err := lp.Output()
	if err != nil {
		lp.DumpLog(ctx)
		return "", err
	}

	// Example output from lp command: "request id is MyPrinter-32"
	// In this case the job ID is "MyPrinter-32".
	r := regexp.MustCompile(printerName + "-[0-9]+")

	if job = r.FindString(string(output)); job == "" {
		return "", errors.New("failed to find prompt for print job started")
	}
	return job, nil
}

// jobCompleted checks whether or not the given print job has been marked as
// completed.
func jobCompleted(ctx context.Context, printerName string, job string) (bool, error) {
	lpstat := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o",
		printerName)

	output, err := lpstat.Output()
	if err != nil {
		lpstat.DumpLog(ctx)
		return false, err
	}

	return strings.Contains(string(output), job), nil
}
