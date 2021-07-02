// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ippprint"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HpPcl3Gui2,
		Desc: "Verifies the lp command enqueues print jobs",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{"test_doc_fourpages.pdf", "printer_hp_pcl3gui2.ppd.gz", "printer_add_hp_pcl3gui2_golden.pcl"},
		Fixture:      "ensureUI",
	})
}

func HpPcl3Gui2(ctx context.Context, s *testing.State) {
	param := ippprint.Params{
		PPDFile:      "printer_hp_pcl3gui2.ppd.gz",
		PrintFile:    "test_doc_fourpages.pdf",
		ExpectedFile: "printer_add_hp_pcl3gui2_golden.pcl",
	}
	ippprint.Run(ctx, s, &param)
}
