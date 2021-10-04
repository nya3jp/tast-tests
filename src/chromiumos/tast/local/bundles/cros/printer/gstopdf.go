// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ghostscript"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Gstopdf,
		Desc:     "Tests that the gstopdf CUPS filter produces expected output",
		Contacts: []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{"gstopdf_input.ps", "gstopdf_golden.pdf"},
	})
}

func Gstopdf(ctx context.Context, s *testing.State) {
	const (
		gsFilter = "gstopdf"
		input    = "gstopdf_input.ps"
		golden   = "gstopdf_golden.pdf"
		envVar   = "CUPS_SERVERBIN=/usr/libexec/cups"
	)
	ghostscript.RunTest(ctx, s, gsFilter, s.DataPath(input), s.DataPath(golden), envVar)
}
