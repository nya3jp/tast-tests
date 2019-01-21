// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package addtest implements Add*Printer tests.
package addtest

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/diff"
	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Run executes the main test logic with given parameters.
func Run(ctx context.Context, s *testing.State, ppdFile, toPrintFile, goldenFile, diffFile string) {
	// Log in to Chrome.
	// There is a timing issue about /var/spool/cupsd directory. On cupsd
	// initialization, the directory is created, and is removed in
	// cups-clear-state.conf, which is triggered on login-prompt-visible.
	// If cupsd starts between ui restart and login-prompt-visible,
	// the directory is removed unexpectedly, and requesting to the cupsd
	// fails.
	// Practically, it is expected that the cupsd starts on demand during
	// a login session. So, as workaround of the timing issue, log in to
	// Chrome here, too.
	c, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in Chrome: ", err)
	}
	defer c.Close(ctx)

	const printerID = "FakePrinterID"

	ppd, err := ioutil.ReadFile(s.DataPath(ppdFile))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}
	expect, err := ioutil.ReadFile(s.DataPath(goldenFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
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
