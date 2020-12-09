// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/proxylpprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProxyAddHPIPPEverywherePrinter,
		Desc: "Verifies the lp command enqueues print jobs for HP IPP Everywhere printers",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "plugin_vm"},
		Pre:          chrome.LoggedIn(),
		Data:         []string{"to_print.pdf", "hp_ipp_everywhere.ppd", "printer_add_hp_ipp_everywhere_golden.pwg"},
	})
}

func ProxyAddHPIPPEverywherePrinter(ctx context.Context, s *testing.State) {
	proxylpprint.Run(ctx, s, "hp_ipp_everywhere.ppd", "to_print.pdf", "printer_add_hp_ipp_everywhere_golden.pwg")
}
