// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
	"io/ioutil"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Printer,
		Desc:         "Sanity test about printer related D-Bus methods.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"cups"},
		Data:         []string{"GenericPostScript.ppd.gz"},
	})
}

func Printer(s *testing.State) {
	ctx := s.Context()
	const (
		// Values are from platform2/system_api/dbus/debugd/dbus-constants.h
		cupsSuccess         = 0
		cupsInvalidPPDError = 2
		cupsLPAdminError    = 3
		cupsAutoconfFailure = 4
	)

	// Read PPD file.
	ppd, err := ioutil.ReadFile(s.DataPath("GenericPostScript.ppd.gz"))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}

	// Connect to debugd.
	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}

	// testValidConfig validates that a printer can be installed.
	testValidConfig := func() {
		result, err := d.CupsAddManuallyConfiguredPrinter(
			ctx, "CUPS rejects names with spaces",
			"socket://127.0.0.1/ipp/fake_printer", ppd)
		if err != nil {
			s.Error("Failed to call CupsAddManuallyCOnfiguredPrinter: ", err)
			return
		}
		if result != cupsLPAdminError {
			s.Error("Names with spaces should be rejected by CUPS: ", result)
		}
	}

	// testLPAdmin verifies the error for a failure in lpadmin.
	testLPAdmin := func() {
		result, err := d.CupsAddManuallyConfiguredPrinter(
			ctx, "ManualPrinterGood",
			"socket://127.0.0.1/ipp/fake_printer", ppd)
		if err != nil {
			s.Error("Failed to call CupsAddManuallyCOnfiguredPrinter: ", err)
			return
		}
		if result != cupsSuccess {
			s.Error("Could not set up valid printer: ", result)
		}

	}

	// testPPDError validates that malformed PPDs are rejected.
	testPPDError := func() {
		badPPD := []byte("This is not a valid ppd")
		result, err := d.CupsAddManuallyConfiguredPrinter(
			ctx, "ManualPrinterBreaks",
			"socket://127.0.0.1/ipp/fake_printer", badPPD)
		if err != nil {
			s.Error("Failed to call CupsAddManuallyCOnfiguredPrinter: ", err)
			return
		}
		if result != cupsInvalidPPDError {
			s.Error("Incorrect error code received: ", result)
		}
	}

	// testAutoconf attempts to add an unreachable autoconfigured printer.
	testAutoconf := func() {
		result, err := d.CupsAddAutoConfiguredPrinter(
			ctx, "AutoconfPrinter", "ipp://127.0.0.1/ipp/print")
		if err != nil {
			s.Error("Failed to call CupsAddAutoConfiguredPrinter: ", err)
			return
		}
		if result != cupsAutoconfFailure {
			s.Error("Incorrect error code received: ", result)
		}
	}

	s.Log("Running testValidConfig")
	testValidConfig()
	s.Log("Running testLPAdmin")
	testLPAdmin()
	s.Log("Running testPPDError")
	testPPDError()
	s.Log("Running testAutoconf")
	testAutoconf()
}
