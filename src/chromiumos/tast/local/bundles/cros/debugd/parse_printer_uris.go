// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/testing"
)

// A printerSpec defines a struct carrying an omnibus of printer
// configuration commonly needed in several test functions. Using this
// struct helps simplify some function definitions, shortening the
// parameter list.
type printerSpec struct {
	name           string            // a descriptive printer name pertinent to test subject
	uri            string            // the printer URI
	ppdContents    []byte            // PPD contents if applicable - empty otherwise
	expectedStatus debugd.CUPSResult // result we expect from debugd
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ParsePrinterUris,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests debugd's behavior when parsing printer URIs",
		Contacts: []string{
			"cros-printing-dev@chromium.org", // Team alias
			"kdlee@chromium.org",             // Test author
		},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"GenericPostScript.ppd.gz"},
		Pre:          chrome.LoggedIn(),
	})
}

// addPrinterWithExpectedStatus adds a manually configured printer with
// the given |spec|. It expects that the D-Bus call to debugd via |d|
// succeeds and returns spec.expectedStatus, logging an error otherwise.
func addPrinterWithExpectedStatus(
	ctx context.Context,
	s *testing.State,
	d *debugd.Debugd,
	spec printerSpec) {
	result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, spec.name, spec.uri, spec.ppdContents)
	if err != nil {
		s.Error("Failed to call d.CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != spec.expectedStatus {
		s.Errorf("%s (%s) failed: got %s; want %s",
			spec.name, spec.uri, result, spec.expectedStatus)
	}
}

// ParsePrinterUris exercises debugd's URI validation routine.
// ATOW this involves a sandboxed helper executable; this test helps
// guard against the possibility of seccomp filters going stale.
func ParsePrinterUris(ctx context.Context, s *testing.State) {
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

	var spec printerSpec

	s.Log("debugd must reject an empty URI")
	spec = printerSpec{
		name:           "EmptyUri",
		uri:            "",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSBadURI,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject a URI containing unprintable ASCII")
	spec = printerSpec{
		name:           "BadCharacters",
		uri:            "ipps://hello-there\x7F:9001/general-kenobi",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSBadURI,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with invalid percent-encodings")
	spec = printerSpec{
		name:           "BadPercents",
		uri:            "http://localhost:9001/your%zz%zzprinter",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSBadURI,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with invalid percent-encodings")
	spec = printerSpec{
		name:           "TrailingPercent",
		uri:            "ipp://localhost:9001/your-printer%",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSBadURI,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must accept URIs with valid percent-encodings")
	spec = printerSpec{
		name:           "GoodPercents",
		uri:            "ippusb://localhost:9001/%ffyour%20printer%19/",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSSuccess,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with invalid port numbers")
	spec = printerSpec{
		name:           "BadPortNumber",
		uri:            "usb://localhost:abcd/hello-there",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSBadURI,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with out-of-range port numbers")
	spec = printerSpec{
		name:           "OverlargePortNumber",
		uri:            "socket://localhost:65536/hello-there",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSBadURI,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must accept URIs with valid port numbers")
	spec = printerSpec{
		name:           "ValidPortNumber",
		uri:            "lpd://localhost:65535/hello-there",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSSuccess,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must accept IPv6 URIs not violating the above conditions")
	spec = printerSpec{
		name:           "IPv6TestA",
		uri:            "ipp://[2001:4860:4860::8888]:65535/hello%20there",
		ppdContents:    ppd,
		expectedStatus: debugd.CUPSSuccess,
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)
}
