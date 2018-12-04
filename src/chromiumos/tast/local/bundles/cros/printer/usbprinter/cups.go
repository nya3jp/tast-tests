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

// Name of the printer to use for configuration with CUPS.
const printerName = "virtual-test"

// cupsAddPrinter adds a new virtual USB printer using CUPS. If ppd is non-empty
// it is used to configure a generic USB printer. Otherwise, the printer is
// configured using IPPUSB.
func cupsAddPrinter(ctx context.Context, devInfo DevInfo, ppd string) error {
	var uri string
	var lpadmin *testexec.Cmd
	if ppd == "" {
		uri = fmt.Sprintf("ippusb://%s_%s/ipp/print", devInfo.VID, devInfo.PID)
		lpadmin = testexec.CommandContext(ctx, "lpadmin", "-p", printerName,
			"-v", uri, "-m", "everywhere", "-E")
	} else {
		uri = fmt.Sprintf("usb://%s/%s", devInfo.VID, devInfo.PID)
		lpadmin = testexec.CommandContext(ctx, "lpadmin", "-p", printerName,
			"-v", uri, "-P", ppd, "-E")
	}

	testing.ContextLog(ctx, "Adding printer to CUPS using ", uri)
	return lpadmin.Run()
}

// cupsRemovePrinter removes the printer that was configured for testing.
func cupsRemovePrinter(ctx context.Context) error {
	return testexec.CommandContext(ctx, "lpadmin", "-x", printerName).Run()
}

// cupsStartPrintJob starts a new print job for the file toPrint. Returns the ID
// of the newly created job if successful.
func cupsStartPrintJob(ctx context.Context, toPrint string) (job string, err error) {
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
func jobCompleted(ctx context.Context, job string) (bool, error) {
	lpstat := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o",
		printerName)

	output, err := lpstat.Output()
	if err != nil {
		lpstat.DumpLog(ctx)
		return false, err
	}

	return strings.Contains(string(output), job), nil
}
