// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lp provides an API for interacting with the CUPS daemon on Chrome OS
// via lp/lpstat/lpamin/etc.
package lp

import (
	"context"
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
