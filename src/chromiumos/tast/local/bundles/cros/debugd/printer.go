// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"
	"net/http"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Printer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Performs validity testing of printer-related D-Bus methods",
		Contacts: []string{
			"bmgordon@chromium.org",
			"hidehiko@chromium.org", // Tast port author
			"project-bolton@google.com",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"GenericPostScript.ppd.gz"},
		Pre:          chrome.LoggedIn(),
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
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
	defer server.Shutdown(ctx)

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
	getPage := func(ctx context.Context) error {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:7001/not_a_printer", nil)
		if err == nil {
			var res *http.Response
			res, err = http.DefaultClient.Do(httpReq)
			if err == nil && res.StatusCode != http.StatusOK {
				err = errors.Errorf("unexpected status of HTTP response: %d", res.StatusCode)
			}
		}
		return err
	}
	if testing.Poll(ctx, getPage, nil) != nil {
		s.Fatal("Cannot start local HTTP server ")
	}

	s.Log("Attempting to add a url that returns HTTP_BAD_REQUEST")
	if result, err := d.CupsAddAutoConfiguredPrinter(
		ctx, "NotAPrinter", "ipp://127.0.0.1:7001/bad_request"); err != nil {
		s.Error("Calling printer setup crashed: ", err)
	} else if result != debugd.CUPSPrinterWrongResponse {
		s.Error("Incorrect error code received: ", result)
	}
}
