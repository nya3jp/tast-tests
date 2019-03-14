// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/addtest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/compupdater"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AddEpsonPrinter,
		Desc: "Verifies the lp command enqueues print jobs with Epson config",
		Contacts: []string{
			"xiaochu@chromium.org",  // Original autotest author
			"hidehiko@chromium.org", // Tast port author
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "cups"},
		Data:         []string{epsonPPDFile, epsonToPrintFile, epsonGoldenFile},
		Pre:          chrome.LoggedIn(),
	})
}

const (
	// epsonPPDFile is ppd.gz file to be registered via debugd.
	epsonPPDFile = "printer_add_epson_printer_EpsonWF3620.ppd"

	// epsonToPrintFile is a PDF file to be printed.
	epsonToPrintFile = "to_print.pdf"

	// epsonGoldenFile contains a golden LPR request data.
	epsonGoldenFile = "printer_add_epson_printer_golden.ps"
)

func AddEpsonPrinter(ctx context.Context, s *testing.State) {
	const (
		// Component name to be loaded before the exercising.
		componentName = "epson-inkjet-printer-escpr"

		// diffFile is the name of the file containing the diff between
		// the golden data and actual request in case of failure.
		diffFile = "printer_add_epson_printer_diff.txt"
	)

	updater, err := compupdater.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to ComponentUpdaterService: ", err)
	}

	// Empty return path (when using compupdater.Mount option) indicates
	// component updater fails to install the given component.
	path, err := updater.LoadComponent(ctx, componentName, compupdater.Mount)
	if err != nil || path == "" {
		s.Fatalf("Failed to load %s: %v", componentName, err)
	}
	defer updater.UnloadComponent(ctx, componentName)

	addtest.Run(ctx, s, epsonPPDFile, epsonToPrintFile, epsonGoldenFile, diffFile)
}
