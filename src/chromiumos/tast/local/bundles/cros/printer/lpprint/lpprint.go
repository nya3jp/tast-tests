// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lpprint implements adding a printer, printing to it via the lp command,
// and comparing the data sent to the printer to a golden file.
package lpprint

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"github.com/kylelemons/godebug/diff"

	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// cleanPSContents filters any unwanted lines from content to ensure a stable
// diff.
func cleanPSContents(content string) string {
	r := regexp.MustCompile(
		// Matches the embedded poppler version in the PS file. This gets
		// outdated on every poppler uprev, so we strip it out.
		"(?m)(^(.*poppler.*version:.*" +
			// For Brother jobs, jobtime and printlog item 2 contain
			// time-specific values.
			"|@PJL SET JOBTIME = .*" +
			"|@PJL PRINTLOG ITEM = 2,.*" +
			// For HP jobs, JobAcct4,JobAcc5 & DMINFO contain
			// time-specific values.
			"|@PJL SET JOBATTR=\"JobAcct[45]=.*" +
			"|@PJL DMINFO ASCIIHEX=\".*" +
			// For Ricoh jobs, the SET DATE/TIME values are time-specific.
			"|@PJL SET DATE=\".*" +
			"|@PJL SET TIME=\".*)[\r\n])" +
			// For Ricoh jobs, the /ID tag is time-specific.
			"|(\\/ID \\[<.*>\\])" +
			// For Ricoh jobs, "usercode (\d+)" contains the date
			// and time of the print job.
			"|(usrcode \\(\\d+\\))" +
			// For Ricoh PS jobs, the time is contained here.
			"|(/Time \\(\\d+\\))" +
			// For Ricoh jobs, "(\d+) lppswd" contains the date
			// and time of the print job.
			"|(\\(\\d+\\)) lppswd")
	return r.ReplaceAllLiteralString(content, "")
}

// Run executes the main test logic with given parameters.
func Run(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile, diffFile string) {
	RunWithOptions(ctx, s, ppdFile, toPrintFile, goldenFile, diffFile, "")
}

// RunWithOptions executes the main test logic with options included in the lp command.
func RunWithOptions(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile, diffFile, options string) {
	const printerID = "FakePrinterID"

	ppd, err := ioutil.ReadFile(s.DataPath(ppdFile))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}
	expect, err := ioutil.ReadFile(s.DataPath(goldenFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	fake, err := fake.NewPrinter(ctx)
	if err != nil {
		s.Fatal("Failed to start fake printer: ", err)
	}
	defer fake.Close()

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}

	testing.ContextLog(ctx, "Registering a printer")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, printerID, "socket://127.0.0.1/", ppd); err != nil {
		s.Fatal("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSSuccess {
		s.Fatal("Could not set up a printer: ", result)
	}

	testing.ContextLog(ctx, "Issuing print request")
	var cmd *testexec.Cmd
	if len(options) != 0 {
		cmd = testexec.CommandContext(ctx, "lp", "-d", printerID, "-o", options, s.DataPath(toPrintFile))
	} else {
		cmd = testexec.CommandContext(ctx, "lp", "-d", printerID, s.DataPath(toPrintFile))
	}
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run lp: ", err)
	}

	testing.ContextLog(ctx, "Receiving print request")
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := fake.ReadRequest(recvCtx)
	if err != nil {
		s.Fatal("Fake printer didn't receive a request: ", err)
	}

	if diff := diff.Diff(cleanPSContents(string(expect)), cleanPSContents(string(request))); diff != "" {
		path := filepath.Join(s.OutDir(), diffFile)
		if err := ioutil.WriteFile(path, []byte(diff), 0644); err != nil {
			s.Error("Failed to dump diff: ", err)
		}
		outPath := filepath.Join(s.OutDir(), goldenFile)
		if err := ioutil.WriteFile(outPath, request, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}

		s.Errorf("Printer output differs from expected: diff saved to %q (-want +got), output to %q", diffFile, goldenFile)
	}
}
