// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package proxylpprint implements adding a printer, printing to it via the lp command,
// and comparing the data sent to the printer to a golden file.
package proxylpprint

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Run executes the main test logic with given parameters.
func Run(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile string) {
	RunWithOptions(ctx, s, ppdFile, toPrintFile, goldenFile, "")
}

// RunWithOptions executes the main test logic with options included in the lp command.
func RunWithOptions(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile, options string) {
	printerID := "FakePrinterID"
	s.Log("printerID: ", printerID)

	err := upstart.EnsureJobRunning(ctx, "cups_proxy")
	if err != nil {
		s.Fatal("Failed to start cups_proxy service: ", err)
	}

	if _, err := os.Stat(s.DataPath(ppdFile)); err != nil {
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

	tconn, err := s.PreValue().(*chrome.Chrome).TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	testing.ContextLog(ctx, "Registering a printer")
	err = tconn.Call(ctx, nil, "chrome.autotestPrivate.updatePrinter", map[string]string{"printerName": printerID, "printerId": printerID, "printerUri": "socket://127.0.0.1/", "printerPpd": s.DataPath(ppdFile)})
	if err != nil {
		s.Fatal("Failed to call autotestPrivate.updatePrinter(): ", err)
	}

	defer func() {
		err := tconn.Call(ctx, nil, "chrome.autotestPrivate.removePrinter", printerID)
		if err != nil {
			s.Fatal("autotestPrivate.removePrinter() failed: ", err)
		}
	}()

	testing.ContextLog(ctx, "Issuing print request")
	args := []string{"-h", "/run/cups_proxy/cups.sock", "-d", printerID}
	if len(options) != 0 {
		args = append(args, "-o", options)
	}
	args = append(args, s.DataPath(toPrintFile))
	cmd := testexec.CommandContext(ctx, "lp", args...)

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run lp: ", err)
	}

	testing.ContextLog(ctx, "Receiving print request")
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := fake.ReadRequest(recvCtx)
	if err != nil {
		s.Fatal("Fake printer didn't receive a request: ", err)
	}

	if lpprint.CleanPSContents(string(expect)) != lpprint.CleanPSContents(string(request)) {
		outPath := filepath.Join(s.OutDir(), goldenFile)
		if err := ioutil.WriteFile(outPath, request, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Printer output differs from expected: output saved to %q", goldenFile)
	}
}
