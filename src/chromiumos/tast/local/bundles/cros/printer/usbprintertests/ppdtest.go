// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprintertests

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func getPPDFilename(ctx context.Context, printerName string) string {
	return fmt.Sprintf("/var/cache/cups/printers/ppd/%s.ppd", printerName)
}

// getPPDMap loads the PPD file which corresponds to the given printer name and
// returns a map containing of each of its key-value pairs.
func getPPDMap(ctx context.Context, printerName string) (map[string]string, error) {
	ppdFilename := getPPDFilename(ctx, printerName)
	f, err := os.Open(ppdFilename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	ppdMap := make(map[string]string)
	for sc.Scan() {
		line := sc.Text()
		data := strings.Split(line, ": ")
		if len(data) == 2 {
			ppdMap[data[0]] = data[1]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return ppdMap, nil
}

// RunIPPUSBPPDTest configures an IPP-over-USB printer using the virtual USB
// printer configured using the given attributes and the well-known default
// descriptors. Once the printer has been automatically configured by CUPS,
// the attributes of the generated PPD file are checked against the provided
// ppdAttributes map. If there are any differences in values between the
// generated PPD and ppdAttributes for the same key, then the test will fail.
func RunIPPUSBPPDTest(ctx context.Context, s *testing.State, attributes string, ppdAttributes map[string]string) {
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithAttributes(attributes),
		usbprinter.WaitUntilConfigured())
	if err != nil {
		s.Fatal("Failed to start IPP-over-USB printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(ctx)

	ppdMap, err := getPPDMap(ctx, printer.ConfiguredName)
	if err != nil {
		s.Fatal("Failed to load PPD file: ", err)
	}

	// Compare the values of ppdAttributes and ppdMap for each key-value pair
	// in ppdAttributes.
	for key, expected := range ppdAttributes {
		v, ok := ppdMap[key]
		if !ok {
			s.Errorf("Found no entry for %v in the generated PPD file", key)
		}
		if v != expected {
			s.Errorf("Unexpected value for key %v in PPD file: got %v, want %v", key, v, expected)
		}
	}
}
