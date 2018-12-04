// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbprinter provides an interface to configure and attach a virtual
// USB printer onto the system to be used for testing.
package usbprinter

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/testing"
)

// RunPrintTest executes a test for the a virtual USB printer defined by the
// given arguments. This tests that the printer is able to be configured, and
// produces the expected output when a print job is issued.
func RunPrintTest(ctx context.Context, s *testing.State, descriptors,
	attributes, record, ppd, toPrint, golden, diffPath string) {
	vid, pid, err := LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatal("Failed to load printer IDs: ", err)
	}

	if err := InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
		if err := removeFile(ctx, record); err != nil {
			s.Error("Failed to remove file: ", err)
		}
		if err := cupsRemovePrinter(ctx); err != nil {
			s.Error("Failed to remove printer: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)

	printer, err := Start(ctx, vid, pid, descriptors, attributes, record)
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}

	if err := cupsAddPrinter(ctx, vid, pid, attributes, ppd); err != nil {
		s.Fatal("Failed to add printer: ", err)
	}

	job, err := cupsStartPrintJob(ctx, toPrint)
	if err != nil {
		s.Fatal("Failed to start printer: ", err)
	}

	if err := waitCompleted(ctx, job); err != nil {
		s.Fatal("Print job didn't complete: ", err)
	}

	if err := compareFiles(ctx, record, golden, diffPath); err != nil {
		s.Error("Printed file differs from golden file: ", err)
	}

	printer.Kill()
	printer.Wait()
	cancel()
}
