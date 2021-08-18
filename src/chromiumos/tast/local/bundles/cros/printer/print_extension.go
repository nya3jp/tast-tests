// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PrintExtension,
		Desc:     "Tests that printing via the chrome.printing extension API works properly",
		Contacts: []string{"batrapranav@google.com", "cros-printing-dev@chromium.org"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Data:         []string{ppdFile, goldenFile},
		Pre:          chrome.LoggedIn(),
		SoftwareDeps: []string{"chrome", "cros_internal", "cups"},
		Params: []testing.Param{
			{
				Name: "cancel",
				Val:  true,
			},
			{
				Name: "complete",
				Val:  false,
			},
		},
	})
}

const ppdFile = "print_usb_ps.ppd.gz"
const goldenFile = "print_extension_golden.ps"

func PrintExtension(ctx context.Context, s *testing.State) {
	const (
		printerID   = "FakePrinterID"
		printerName = "FakePrinterName"
		printerDesc = "FakePrinterDescription"
	)
	ppdFilePath := s.DataPath(ppdFile)
	if _, err := os.Stat(ppdFilePath); err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}

	expect, err := ioutil.ReadFile(s.DataPath(goldenFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	printer, err := fake.NewPrinter(ctx)
	if err != nil {
		s.Fatal("Failed to start fake printer: ", err)
	}
	defer printer.Close()

	tconn, err := s.PreValue().(*chrome.Chrome).TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Registering a printer")
	if err := tconn.Call(ctx, nil, "chrome.autotestPrivate.updatePrinter", map[string]string{"printerName": printerName, "printerId": printerID, "printerDesc": printerDesc, "printerUri": "socket://localhost", "printerPpd": ppdFilePath}); err != nil {
		s.Fatal("autotestPrivate.updatePrinter() failed: ", err)
	}

	defer func() {
		if err := tconn.Call(ctx, nil, "chrome.autotestPrivate.removePrinter", printerID); err != nil {
			s.Fatal("autotestPrivate.removePrinter() failed: ", err)
		}
	}()

	s.Log("Calling chrome.printing.getPrinters")
	var printers []struct {
		Description      string
		ID               string
		IsDefault        bool
		Name             string
		RecentlyUsedRank int
		Source           string
		URI              string
	}
	if err := tconn.Call(ctx, &printers, "tast.promisify(chrome.printing.getPrinters)"); err != nil {
		s.Fatal("Failed to call getPrinters: ", err)
	}
	if len(printers) != 1 {
		s.Fatalf("Found %d printers", len(printers))
	}
	if printers[0].Description != printerDesc {
		s.Error("Unexpected description: ", printers[0].Description)
	}
	if printers[0].ID != printerID {
		s.Error("Unexpected id: ", printers[0].ID)
	}
	if printers[0].IsDefault != false {
		s.Error("Unexpected isDefault value: ", printers[0].IsDefault)
	}
	if printers[0].Name != printerName {
		s.Error("Unexpected name: ", printers[0].Name)
	}
	if printers[0].Source != "USER" {
		s.Error("Unexpected source: ", printers[0].Source)
	}
	if printers[0].URI != "socket://localhost:9100" {
		s.Error("Unexpected uri: ", printers[0].URI)
	}

	s.Log("Calling chrome.printing.getPrinterInfo")
	var info struct {
		Capabilities struct {
			Version string
			Printer map[string]interface{}
			Scanner map[string]interface{}
		}
		Status string
	}
	if err := tconn.Call(ctx, &info, "tast.promisify(chrome.printing.getPrinterInfo)", printerID); err != nil {
		s.Fatal("Failed to call getPrinterInfo: ", err)
	}
	if info.Capabilities.Version != "1.0" {
		s.Error("Unexpected version: ", info.Capabilities.Version)
	}
	for _, attr := range []string{"color", "collate", "copies", "dpi", "duplex", "media_size", "page_orientation", "pin", "supported_content_type", "vendor_capability"} {
		if _, ok := info.Capabilities.Printer[attr]; !ok {
			s.Error("Missing printer capability: ", attr)
		}
	}
	if len(info.Capabilities.Scanner) != 0 {
		s.Errorf("Unexpected scanner capabilities: found %d elements", len(info.Capabilities.Scanner))
	}
	if info.Status != "AVAILABLE" {
		s.Error("Unexpected status: ", info.Status)
	}

	s.Log("Registering chrome.printing.onJobStatusChanged listener")
	if err := tconn.Eval(ctx, "var events = []; chrome.printing.onJobStatusChanged.addListener((id,status)=>events.push({id: id, status: status}))", nil); err != nil {
		s.Fatal("Failed to register onJobStatusChanged observer: ", err)
	}

	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setWhitelistedPref)", "printing.printing_api_extensions_whitelist", []string{chrome.TestExtensionID}); err != nil {
		s.Fatal("Failed to set printing.printing_api_extensions_whitelist: ", err)
	}

	s.Log("Calling chrome.printing.submitJob")
	var job struct {
		JobID  string
		Status string
	}
	if err := tconn.Eval(ctx, `tast.promisify(chrome.printing.submitJob)({
	  job: {
	    contentType: "application/pdf",
	    document: new Blob([atob("JVBERi0xLjAKMSAwIG9iajw8L1BhZ2VzIDIgMCBSPj5lbmRvYmogMiAwIG9iajw8L0tpZHNbMyAw\nIFJdL0NvdW50IDE+PmVuZG9iaiAzIDAgb2JqPDwvTWVkaWFCb3hbMCAwIDMgM10+PmVuZG9iagp0\ncmFpbGVyPDwvUm9vdCAxIDAgUj4+Cg==")]),
	    printerId: "`+printerID+`",
	    ticket: {
	      version: "1.0",
	      print: {
		color: { type: "STANDARD_COLOR" },
		duplex: { type: "NO_DUPLEX" },
		page_orientation: { type: "PORTRAIT" },
		copies: { copies: 2 },
		margins: { top_microns: 1, right_microns: 1, bottom_microns: 1, left_microns: 1 },
		dpi: { horizontal_dpi: 600, vertical_dpi: 600 },
		media_size: { width_microns: 210000, height_microns: 297000, vendor_id: "iso_a4_210x297mm" },
		collate: { collate: false }
	      }
	    },
	    title: "title"
	  }
	})`, &job); err != nil {
		s.Fatal("Failed to call submitJob: ", err)
	}
	if job.Status != "OK" {
		s.Fatal("Unexpected status: ", job.Status)
	}
	if len(job.JobID) == 0 {
		s.Fatal("Empty JobId")
	}

	s.Log("Receiving print request")
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := printer.ReadRequest(recvCtx)
	if err != nil {
		s.Fatal("Fake printer didn't receive a request: ", err)
	}

	if document.CleanContents(string(expect)) != document.CleanContents(string(request)) {
		outPath := filepath.Join(s.OutDir(), goldenFile)
		if err := ioutil.WriteFile(outPath, request, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Printer output differs from expected: output saved to %q", goldenFile)
	}

	var events []struct {
		ID     string
		Status string
	}
	if err := tconn.Eval(ctx, "events", &events); err != nil {
		s.Fatal("Failed to get events: ", err)
	}
	if len(events) != 2 {
		s.Fatal("Unexpected number of events: ", len(events))
	}
	if events[0].ID != job.JobID || events[0].Status != "PENDING" {
		s.Errorf("Unxpected event: %s %s", events[0].ID, events[0].Status)
	}
	if events[1].ID != job.JobID || events[1].Status != "IN_PROGRESS" {
		s.Errorf("Unexpected event: %s %s", events[1].ID, events[1].Status)
	}

	if s.Param().(bool) {
		s.Log("Calling chrome.printing.cancelJob")
		if err := tconn.Call(ctx, nil, "tast.promisify(chrome.printing.cancelJob)", job.JobID); err != nil {
			s.Fatal("Failed to call cancelJob: ", err)
		}
		if err := tconn.Eval(ctx, "events", &events); err != nil {
			s.Fatal("Failed to get events: ", err)
		}
		if len(events) != 3 {
			s.Fatal("Unexpected number of events: ", len(events))
		}
		if events[2].ID != job.JobID || events[2].Status != "CANCELED" {
			s.Errorf("Unexpected event: %s %s", events[2].ID, events[2].Status)
		}
	} else {
		s.Log("Disconnecting printer")
		printer.Close()
		if err := tconn.WaitForExprFailOnErrWithTimeout(ctx, "events.length >= 3", 10*time.Second); err != nil {
			s.Error("Failure waiting for events: ", err)
		}
		if err := tconn.Eval(ctx, "events", &events); err != nil {
			s.Fatal("Failed to get events: ", err)
		}
		if len(events) != 3 {
			s.Fatal("Unexpected number of events: ", len(events))
		}
		if events[2].ID != job.JobID || events[2].Status != "PRINTED" {
			s.Errorf("Unexpected event: %s %s", events[2].ID, events[2].Status)
		}
	}
}
