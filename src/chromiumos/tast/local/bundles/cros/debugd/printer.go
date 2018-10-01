// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
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

func Printer(ctx context.Context, s *testing.State) {
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
