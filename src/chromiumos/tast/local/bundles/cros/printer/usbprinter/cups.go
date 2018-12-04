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

// cupsAddPrinter adds a new virtual USB printer using CUPS. If attributes is
// non-empty then the printer will be configured using IPPUSB. Otherwise, it is
// configured as a generic USB printing using the given ppd.
func cupsAddPrinter(ctx context.Context, vid, pid, attributes, ppd string) error {
	var uri string
	var lpadmin *testexec.Cmd
	if attributes != "" {
		uri = fmt.Sprintf("ippusb://%s_%s/ipp/print", vid, pid)
		lpadmin = testexec.CommandContext(ctx, "lpadmin", "-p", printerName,
			"-v", uri, "-m", "everywhere", "-E")
	} else {
		uri = fmt.Sprintf("usb://%s/%s", vid, pid)
		lpadmin = testexec.CommandContext(ctx, "lpadmin", "-p", printerName,
			"-v", uri, "-P", ppd, "-E")
	}

	testing.ContextLog(ctx, "Adding printer to CUPS")
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
		return "", err
	}

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
		return false, err
	}

	if strings.Contains(string(output), job) {
		return true, nil
	}

	return false, nil
}
