// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/diff"
	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddGenericPrinter,
		Desc:         "Verifies the lp command enqueues print jobs",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups"},
		Data:         []string{ppdFile, toPrintFile, goldenFile},
	})
}

const (
	// ppdFile is ppd.gz file to be registered via debugd.
	ppdFile = "printer_add_generic_printer_GenericPostScript.ppd.gz"

	// toPrintFile is a PDF file to be printed.
	toPrintFile = "to_print.pdf"

	// goldenFile contains a golden LPR request data.
	goldenFile = "printer_add_generic_printer_golden.ps"

	// diffFile is the name of the file containing the diff between the
	// golden data and actual request in case of failure.
	diffFile = "printer_add_generic_printer_diff.txt"
)

func AddGenericPrinter(ctx context.Context, s *testing.State) {
	const printerID = "FakePrinterID"

	ppd, err := ioutil.ReadFile(s.DataPath(ppdFile))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}
	expect, err := ioutil.ReadFile(s.DataPath(goldenFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}

	fake, err := fake.NewPrinter()
	if err != nil {
		s.Fatal("Failed to start fake printer: ", err)
	}
	defer fake.Close(ctx)

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
	cmd := testexec.CommandContext(ctx, "lp", "-d", printerID, s.DataPath(toPrintFile))
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

	diff, err := diff.Diff(string(request), string(expect))
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
