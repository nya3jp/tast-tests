// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/testing"
)

// genericPPDFile is ppd.gz file to be registered via debugd.
const httpTestPpdFile string = "printer_add_generic_printer_GenericPostScript.ppd.gz"

func init() {
	testing.AddTest(&testing.Test{
		Func: AddHTTPPrinter,
		Desc: "Verifies that http printers can be installed",
		Contacts: []string{
			"skau@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{httpTestPpdFile},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func AddHTTPPrinter(ctx context.Context, s *testing.State) {
	// Downloads the PPD and tries to install the printer using the dbus method.
	const printerID = "HttpPrinterId"

	ppd, err := ioutil.ReadFile(s.DataPath(httpTestPpdFile))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}

	testing.ContextLog(ctx, "Registering a printer")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, printerID, "http://chromium.org:999/this/is/a/test", ppd); err != nil {
		s.Fatal("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSSuccess {
		s.Fatal("Could not set up a printer: ", result)
	}
}
