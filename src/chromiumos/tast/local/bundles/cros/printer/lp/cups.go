// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lp

import (
	"context"
	//"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Regular expression used to match a line from the output of the lpstat
// command.
const lpstatPatternPrefix = `device for (-[-a-zA-Z0-9]+): `

// PrinterNameByURI runs the lpstat command to search for a configured printer
// which corresponds to |uri|. Return the name of the matching printer if found.
func PrinterNameByURI(ctx context.Context, uri string) (name string, err error) {
	out, err := testexec.CommandContext(ctx, "lpstat", "-v").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run scan for configured printers")
	}

	r := regexp.MustCompile(lpstatPatternPrefix + regexp.QuoteMeta(uri))
	for _, line := range strings.Split(string(out), "\n") {
		submatches := r.FindStringSubmatch(line)
		if submatches != nil {
			return submatches[1], nil
		}
	}

	return "", errors.Errorf("failed to find printer with uri %s", uri)
}

// CupsAddPrinter adds a new printer using CUPS. Returns an error if the |ppd|
// is empty or lpadmin fails.
func CupsAddPrinter(ctx context.Context, printerName, uri, ppd string) error {
	if ppd == "" {
		return errors.New("must provide PPD to CupsAddPrinter")
	}
	testing.ContextLog(ctx, "Adding driverless printer to CUPS using ", uri)
	return testexec.CommandContext(ctx, "lpadmin", "-p", printerName, "-v", uri, "-P", ppd, "-E").Run(testexec.DumpLogOnError)
}

// CupsAddDriverlessPrinter adds a new driverless printer using CUPS. Returns
// an error on lpadmin failure.
func CupsAddDriverlessPrinter(ctx context.Context, printerName, uri string) error {
	testing.ContextLog(ctx, "Adding driverless printer to CUPS using ", uri)
	return testexec.CommandContext(ctx, "lpadmin", "-p", printerName, "-v", uri, "-E").Run(testexec.DumpLogOnError)
}

// CupsRemovePrinter removes the printer that was configured for testing.
func CupsRemovePrinter(ctx context.Context, printerName string) error {
	return testexec.CommandContext(ctx, "lpadmin", "-x", printerName).Run()
}

// CupsStartPrintJob starts a new print job for the file |toPrint|. Returns the
// ID of the newly created job if successful.
func CupsStartPrintJob(ctx context.Context, printerName string, toPrint string) (job string, err error) {
	testing.ContextLog(ctx, "Starting print job")
	output, err := testexec.CommandContext(ctx, "lp", "-d", printerName, "--", toPrint).Output(testexec.DumpLogOnError)
	if err != nil {
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

// JobCompleted checks whether or not the given print job has been marked as
// completed.
func JobCompleted(ctx context.Context, printerName, job string) (bool, error) {
	out, err := testexec.CommandContext(ctx, "lpstat", "-W", "completed", "-o",
		printerName).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "failed to capture lpstat output")
	}

	return strings.Contains(string(out), job), nil
}

// PrinterStateReasons returns the current printer-state-reasons for
// |printerName| via lpstat.
func PrinterStateReasons(ctx context.Context, printerName string) (string, error) {
	out, err := testexec.CommandContext(ctx, "lpstat", "-p", printerName).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to capture lpstat output")
	}

	// Each printer status is made of two lines, like:
	// printer {printerName} is idle. enable since Fri Aug 16 11:19:24 2019
	//	{printer-state-reasons}
	lines := strings.Split(string(out), "\n")
	for i := 0; i < len(lines)-1; i += 2 {
		if strings.Contains(lines[i], printerName) {
			return lines[i+1], nil
		}
	}

	return "", errors.New("failed to get printer status reasons")
}

// PrinterUnreachable returns whether |printerName| is unreachable by
// evaluating its printer-state-reasons.
func PrinterUnreachable(ctx context.Context, printerName string) (bool, error) {
	const printerUnreachableStateReason = "The printer is not responding."
	if stateReason, err := PrinterStateReasons(ctx, printerName); err != nil {
		return false, err
	} else if strings.Contains(stateReason, printerUnreachableStateReason) {
		return true, nil
	}

	return false, nil
}
