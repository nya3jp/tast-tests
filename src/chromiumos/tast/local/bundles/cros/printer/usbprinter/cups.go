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

const lpstatMatcherFormat = `device for (?P<name>usb-[a-f0-9]+): ippusb://(?P<vid>%s)_(?P<pid>%s)/ipp/print`

func getPrinterName(ctx context.Context, devInfo DevInfo) (name string, err error) {
	lpstat := testexec.CommandContext(ctx, "lpstat", "-v")
	testing.ContextLog(ctx, "Searching for printer name")
	output, err := lpstat.Output()
	if err != nil {
		lpstat.DumpLog(ctx)
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	lpstatMatcher := fmt.Sprintf(lpstatMatcherFormat, devInfo.VID, devInfo.PID)
	r := regexp.MustCompile(lpstatMatcher)

	for _, line := range lines {
		testing.ContextLog(ctx, line)
		matches := r.FindStringSubmatch(line)
		subnames := r.SubexpNames()

		if len(matches) != len(subnames) {
			continue
		}

		for i := range matches {
			if subnames[i] == "name" {
				return matches[i], nil
			}
		}
	}

	return "", nil
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
