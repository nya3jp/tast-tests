// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package addtest implements Add*Printer tests.
package addtest

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/diff"
	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// cleanPSContents filters any unwanted lines from |content| to ensure a stable
// diff.
func cleanPSContents(content string) string {
	// Matches the embedded poppler version in the PS file. This gets
	// outdated on every poppler uprev, so we strip it out.
	r := regexp.MustCompile("(?m)^.*poppler.*version:.*[\r\n]")
	return r.ReplaceAllLiteralString(content, "")
}

// Run executes the main test logic with given parameters.
func Run(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile, diffFile string) {
	RunWithOptions(ctx, s, ppdFile, toPrintFile, goldenFile, diffFile, "")
}

// RunWithOptions executes the main test logic with |options| included in the lp command.
func RunWithOptions(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile, diffFile string, options string) {
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

	diff, err := diff.Diff(cleanPSContents(string(request)), cleanPSContents(string(expect)))
	if err != nil {
		s.Fatal("Unexpected diff output: ", err)
	}
	if diff != "" {
		path := filepath.Join(s.OutDir(), diffFile)
		if err := ioutil.WriteFile(path, []byte(diff), 0644); err != nil {
			s.Error("Failed to dump diff: ", err)
		}
		s.Errorf("Read request has diff from the golden file, dumped at %s", diffFile)
	}
}
