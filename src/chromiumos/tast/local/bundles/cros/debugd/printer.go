// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Printer,
		Desc: "Performs sanity testing of printer-related D-Bus methods",
		Contacts: []string{
			"skau@chromium.org",     // Original autotest author
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"GenericPostScript.ppd.gz"},
	})
}

func Printer(ctx context.Context, s *testing.State) {
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

	ppd, err := ioutil.ReadFile(s.DataPath("GenericPostScript.ppd.gz"))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}

	s.Log("Validating that a printer can be installed")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, "CUPS rejects names with spaces",
		"socket://127.0.0.1/ipp/fake_printer", ppd); err != nil {
		s.Error("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSLPAdminFailure {
		s.Error("Names with spaces should be rejected by CUPS: ", result)
	}

	s.Log("Verifying error is returned for lpadmin failure")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, "ManualPrinterGood",
		"socket://127.0.0.1/ipp/fake_printer", ppd); err != nil {
		s.Error("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSSuccess {
		s.Error("Could not set up valid printer: ", result)
	}

	s.Log("Validating that malformed PPDs are rejected")
	badPPD := []byte("This is not a valid ppd")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, "ManualPrinterBreaks",
		"socket://127.0.0.1/ipp/fake_printer", badPPD); err != nil {
		s.Error("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSInvalidPPD {
		s.Error("Incorrect error code received: ", result)
	}

	s.Log("Attempting to add an unreachable autoconfigured printer")
	if result, err := d.CupsAddAutoConfiguredPrinter(
		ctx, "AutoconfPrinter", "ipp://127.0.0.1/ipp/print"); err != nil {
		s.Error("Failed to call CupsAddAutoConfiguredPrinter: ", err)
	} else if result != debugd.CUPSAutoconfFailure {
		s.Error("Incorrect error code received: ", result)
	}
}
