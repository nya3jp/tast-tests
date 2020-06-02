// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ghostscript"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gstoraster,
		Desc:         "Tests that the gstoraster CUPS filter produces expected output",
		Contacts:     []string{"skau@chromium.org", "project-bolton@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"gstoraster_input.pdf", "gstoraster_golden.pwg"},
		Pre:          chrome.LoggedIn(),
	})
}

func Gstoraster(ctx context.Context, s *testing.State) {
	const (
		gsFilter = "gstoraster"
		input    = "gstoraster_input.pdf"
		golden   = "gstoraster_golden.pwg"
	)
	ghostscript.RunTest(ctx, s, gsFilter, s.DataPath(input), s.DataPath(golden), "" /*envVar*/)
}
