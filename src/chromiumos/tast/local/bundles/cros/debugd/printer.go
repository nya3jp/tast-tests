// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Printer,
		Desc: "Performs validity testing of printer-related D-Bus methods",
		Contacts: []string{
			"skau@chromium.org",     // Original autotest author
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"GenericPostScript.ppd.gz"},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func pageNotAPrinter(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("It is not a printer!"))
}

func Printer(ctx context.Context, s *testing.State) {
	// Start local HTTP server on port 7001.
	http.HandleFunc("/not_a_printer", pageNotAPrinter)
	server := &http.Server{Addr: ":7001"}
	go server.ListenAndServe()
	defer server.Close()

	ppd, err := ioutil.ReadFile(s.DataPath("GenericPostScript.ppd.gz"))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	s.Log("Validating that a printer can be installed")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, "CUPS rejects names with spaces",
		"socket://127.0.0.1/ipp/fake_printer", ppd); err != nil {
		s.Error("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSFatal {
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
	} else if result != debugd.CUPSPrinterUnreachable {
		s.Error("Incorrect error code received: ", result)
	}

	// Make sure that the HTTP server on port 7001 is ready.
	for tries := 1; true; tries++ {
		res, err := http.Get("http://127.0.0.1:7001/not_a_printer")
		if err == nil && res.StatusCode == http.StatusOK {
			break
		}
		if tries == 5 {
			s.Fatal("Cannot start local HTTP server ")
		}
		testing.Sleep(ctx, 100*time.Millisecond)
	}

	s.Log("Attempting to add a url that returns HTTP_BAD_REQUEST")
	if result, err := d.CupsAddAutoConfiguredPrinter(
		ctx, "NotAPrinter", "ipp://127.0.0.1:7001/bad_request"); err != nil {
		s.Error("Calling printer setup crashed: ", err)
	} else if result != debugd.CUPSPrinterWrongResponse {
		s.Error("Incorrect error code received: ", result)
	}

	s.Log("Attempting to add a url that returns text page")
	if result, err := d.CupsAddAutoConfiguredPrinter(
		ctx, "NotAPrinter", "ipp://127.0.0.1:7001/not_a_printer"); err != nil {
		s.Error("Calling printer setup crashed: ", err)
	} else if result != debugd.CUPSPrinterWrongResponse {
		s.Error("Incorrect error code received: ", result)
	}
}
