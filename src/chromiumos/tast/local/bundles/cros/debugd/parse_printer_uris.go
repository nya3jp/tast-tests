// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/testing"
)

// A printerSpec defines a struct carrying an omnibus of printer
// configuration commonly needed in several test functions. Using this
// struct helps simplify some function definitions, shortening the
// parameter list.
type printerSpec struct {
	name             string            // a descriptive printer name pertinent to test subject
	uri              string            // the printer URI
	ppdContents      []byte            // PPD contents if applicable - empty otherwise
	expectedStatus   debugd.CUPSResult // result we expect from debugd
	onFailureMessage string            // message logged if debugd return value differs
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ParsePrinterUris,
		Desc: "Tests debugd's behavior when parsing printer URIs",
		Contacts: []string{
			"skau@chromium.org",  // Original autotest author
			"kdlee@chromium.org", // Tast port author
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"GenericPostScript.ppd.gz"},
		Pre:          chrome.LoggedIn(),
	})
}

// addPrinterWithExpectedStatus adds a printer (whether auto- or manually
// configured) with the given |spec|. It expects that the D-Bus call to
// debugd via |d| succeeds and returns spec.expectedStatus, logging
// spec.onFailureMessage otherwise.
//
// Note that if spec.ppdContents is empty, this function treats
// the printer as auto-configured. Otherwise, this function treats
// the printer as manually configured.
func addPrinterWithExpectedStatus(
	ctx context.Context,
	s *testing.State,
	d *debugd.Debugd,
	spec printerSpec) {
	isManuallyConfigured := len(spec.ppdContents) > 0

	var result debugd.CUPSResult
	var err error
	if isManuallyConfigured {
		result, err = d.CupsAddManuallyConfiguredPrinter(
			ctx, spec.name, spec.uri, spec.ppdContents)
	} else {
		result, err = d.CupsAddAutoConfiguredPrinter(
			ctx, spec.name, spec.uri)
	}

	functionCalled := "d.CupsAddAutoConfiguredPrinter"
	if isManuallyConfigured {
		functionCalled = "d.CupsAddManuallyConfiguredPrinter"
	}

	if err != nil {
		s.Errorf("Failed to call %s: %s", functionCalled, err)
	} else if result != spec.expectedStatus {
		s.Error(spec.onFailureMessage, result)
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
		name:             "EmptyUri",
		uri:              "",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSBadURI,
		onFailureMessage: "Empty URI should return CUPSBadURI, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject a URI containing unprintable ASCII")
	spec = printerSpec{
		name:             "BadCharacters",
		uri:              "ipps://hello-there\x7F:9001/general-kenobi",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSBadURI,
		onFailureMessage: "URI with bad ASCII should return CUPSBadURI, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with invalid percent-encodings")
	spec = printerSpec{
		name:             "BadPercents",
		uri:              "http://localhost:9001/your%zz%zzprinter",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSBadURI,
		onFailureMessage: "Bad percent encoding should return CUPSBadURI, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with invalid percent-encodings")
	spec = printerSpec{
		name:             "BadPercents",
		uri:              "ipp://localhost:9001/your-printer%",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSBadURI,
		onFailureMessage: "Trailing percent character should return CUPSBadURI, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must accept URIs with valid percent-encodings")
	spec = printerSpec{
		name:             "GoodPercents",
		uri:              "ippusb://localhost:9001/%ffyour%20printer%19/",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSSuccess,
		onFailureMessage: "Valid percent encoding should return CUPSSuccess, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with invalid port numbers")
	spec = printerSpec{
		name:             "BadPortNumber",
		uri:              "usb://localhost:abcd/hello-there",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSBadURI,
		onFailureMessage: "Bad port number should return CUPSBadURI, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must reject URIs with out-of-range port numbers")
	spec = printerSpec{
		name:             "OverlargePortNumber",
		uri:              "socket://localhost:65536/hello-there",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSBadURI,
		onFailureMessage: "Overlarge port number should return CUPSBadURI, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must accept URIs with valid port numbers")
	spec = printerSpec{
		name:             "ValidPortNumber",
		uri:              "lpd://localhost:65535/hello-there",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSSuccess,
		onFailureMessage: "Valid port number should return CUPSSuccess, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)

	s.Log("debugd must accept IPv6 URIs not violating the above conditions")
	spec = printerSpec{
		name:             "IPv6TestA",
		uri:              "ipp://[2001:4860:4860::8888]:65535/hello%20there",
		ppdContents:      ppd,
		expectedStatus:   debugd.CUPSSuccess,
		onFailureMessage: "Valid URI should return CUPSSuccess, but got: ",
	}
	addPrinterWithExpectedStatus(ctx, s, d, spec)
}
