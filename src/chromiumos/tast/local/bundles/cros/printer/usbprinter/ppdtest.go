// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprinter

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/printer"
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
		return map[string]string{}, err
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
		return map[string]string{}, err
	}

	return ppdMap, nil
}

// RunIPPUSBPPDTest configures an IPP-over-USB printer using the virtual USB
// printer configured using the given descriptors and attributes. Once the
// printer has been automatically configured by CUPS the attributes of the
// generated PPD file are checked against the provided ppdAttributes map. If
// there are any differences in values between the generated PPD and
// ppdAttributes for the same key then the test will
func RunIPPUSBPPDTest(ctx context.Context, s *testing.State, descriptors, attributes string, ppdAttributes map[string]string) {
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	devInfo, err := LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	// Use oldContext for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	oldContext := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func() {
		if err := RemoveModules(oldContext); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}()

	printer, name, err := StartIPPUSB(ctx, devInfo, descriptors, attributes, "" /*record*/)
	if err != nil {
		if printer == nil {
			s.Fatal("Failed to attach virtual printer: ", err)
		}
		if name == "" {
			s.Fatal("Failed to find configured printer name: ", err)
		}
	}

	ppdMap, err := getPPDMap(ctx, name)
	if err != nil {
		s.Fatal("Failed to load PPD file: ", err)
	}

	// Compare the values of ppdAttributes and ppdMap for each key-value pair
	// in ppdAttributes.
	for key, expected := range ppdAttributes {
		v, ok := ppdMap[key]
		if !ok {
			s.Errorf("Found no entry for %s in the generated PPD file", key)
		}
		if v != expected {
			s.Errorf("Value for key %s in PPD file does not match the expected value (%s vs. %s)", key, v, expected)
		}
	}
}
